package registry_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins/registry"
	"github.com/auth0/auth0-cli/internal/plugins/registry/registrytest"
)

func TestCache_FileModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not have Unix file modes")
	}

	f := newFixture(t)
	_, err := f.client().Refresh(context.Background())
	require.NoError(t, err)

	info, err := os.Stat(f.dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())

	for _, dir := range []string{"metadata", "roots"} {
		info, err := os.Stat(filepath.Join(f.dir, dir))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o700), info.Mode().Perm(), dir)
	}

	files := []string{
		"index.json", "state.json", ".lock",
		"metadata/root.json", "metadata/timestamp.json", "metadata/snapshot.json", "metadata/targets.json",
	}

	for _, name := range files {
		info, err := os.Stat(filepath.Join(f.dir, filepath.FromSlash(name)))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), name)
	}

	for dir, want := range map[string]int{".": 5, "metadata": 4, "roots": 0} {
		entries, err := os.ReadDir(filepath.Join(f.dir, dir))
		require.NoError(t, err)
		assert.Len(t, entries, want, "no temporary file is left behind in %s", dir)
	}
}

func TestCache_RestrictsAnExistingDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not have Unix file modes")
	}

	f := newFixture(t)
	require.NoError(t, os.MkdirAll(f.dir, 0o755))
	require.NoError(t, os.Chmod(f.dir, 0o755), "the umask must not have narrowed it")

	_, err := f.client().Refresh(context.Background())
	require.NoError(t, err)

	info, err := os.Stat(f.dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}

func TestCache_DamagedState(t *testing.T) {
	tests := []struct {
		name    string
		damage  func(state string) string
		wantErr string
	}{
		{name: "cut short", damage: func(string) string { return "{" }, wantErr: "not valid JSON"},
		{name: "a key that appears twice", damage: func(state string) string { return strings.Replace(state, "{", `{"highestVersion":0,`, 1) }, wantErr: `duplicate key "highestVersion"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			client := f.client()
			_, err := client.Refresh(context.Background())
			require.NoError(t, err)

			f.writeCache("state.json", []byte(test.damage(string(f.readCache("state.json")))))

			_, _, err = client.Load(t.Context())
			require.Error(t, err)
			assert.Contains(t, err.Error(), "damaged")
			assert.Contains(t, err.Error(), test.wantErr)

			_, err = client.Refresh(context.Background())
			assert.Error(t, err, "a damaged state does not reset rollback protection")
		})
	}
}

func TestCache_Atomicity(t *testing.T) {
	// The helper "failStaging" makes writing the named cache file fail, as a full disk does.
	failStaging := func(file string) func() {
		return registry.SetStageFile(func(dir, name string, data []byte) (string, error) {
			if name == file {
				return "", errors.New("no space left on device")
			}

			return registry.StageTempFile(dir, name, data)
		})
	}
	// The helper "failRenaming" makes replacing the named cache file fail.
	failRenaming := func(file string) func() {
		return registry.SetRenameFile(func(oldPath, newPath string) error {
			if filepath.Base(newPath) == file {
				return errors.New("permission denied")
			}

			return os.Rename(oldPath, newPath)
		})
	}

	tests := []struct {
		name   string
		inject func() func()
	}{
		{name: "writing the index fails", inject: func() func() { return failStaging("index.json") }},
		{name: "writing the targets metadata fails", inject: func() func() { return failStaging("targets.json") }},
		{name: "writing the state fails", inject: func() func() { return failStaging("state.json") }},
		{name: "replacing the timestamp fails", inject: func() func() { return failRenaming("timestamp.json") }},
		{name: "replacing the snapshot fails", inject: func() func() { return failRenaming("snapshot.json") }},
		{name: "replacing the targets metadata fails", inject: func() func() { return failRenaming("targets.json") }},
		{name: "replacing the state fails", inject: func() func() { return failRenaming("state.json") }},
		{name: "replacing the index fails", inject: func() func() { return failRenaming("index.json") }},
	}

	for _, test := range tests {
		t.Run("keeps the previous cache when "+test.name, func(t *testing.T) {
			f := newFixture(t)
			client := f.client()

			f.publish(f.builder().SetVersion(1))
			_, err := client.Refresh(context.Background())
			require.NoError(t, err)

			before := f.cache()

			f.publish(f.builder().SetVersion(2))
			restore := test.inject()
			_, err = client.Refresh(context.Background())
			restore()

			require.Error(t, err)
			assert.Equal(t, before, f.cache(), "no file changed and no temporary file is left")

			index, _, err := client.Load(t.Context())
			require.NoError(t, err, "the previous cache still verifies")
			assert.Equal(t, 1, index.Version)

			index, err = client.Refresh(context.Background())
			require.NoError(t, err, "the next refresh works")
			assert.Equal(t, 2, index.Version)
		})
	}

	t.Run("leaves nothing behind when the first refresh fails halfway", func(t *testing.T) {
		f := newFixture(t)

		restore := failRenaming("index.json")
		_, err := f.client().Refresh(context.Background())
		restore()

		require.Error(t, err)
		assert.Empty(t, f.cache())

		_, _, err = f.client().Load(t.Context())
		assert.ErrorIs(t, err, registry.ErrNotCached)
	})
}

func TestCache_ReplacesTheFilesInAFailClosedOrder(t *testing.T) {
	tests := []struct {
		name string

		// Prepare changes what the second refresh gets, after the first one.
		prepare func(f *fixture)

		// Order is the files that are replaced, and want what the cache does at
		// the moment before each is replaced.
		order []string
		want  []error
	}{
		{
			name:    "an update of the index",
			prepare: func(*fixture) {},
			order:   []string{"state.json", "timestamp.json", "snapshot.json", "targets.json", "index.json"},
			want: []error{
				nil,
				registry.ErrRollback,
				registry.ErrRollback,
				registry.ErrRollback,
				registry.ErrSignatureInvalid,
			},
		},
		{
			name:    "an update of the index and a new targets key",
			prepare: func(f *fixture) { f.repo.RotateKey(registrytest.RoleTargets) },
			order:   []string{"state.json", "2.root.json", "root.json", "timestamp.json", "snapshot.json", "targets.json", "index.json"},
			want: []error{
				nil,
				registry.ErrRollback,
				registry.ErrSignatureInvalid,
				registry.ErrSignatureInvalid,
				registry.ErrSignatureInvalid,
				registry.ErrSignatureInvalid,
				registry.ErrSignatureInvalid,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			client := f.client()

			f.publish(f.builder().SetVersion(1))
			_, err := client.Refresh(context.Background())
			require.NoError(t, err)

			test.prepare(f)
			f.publish(f.builder().SetVersion(2))

			var (
				order []string
				seen  []error
			)

			restore := registry.SetRenameFile(func(oldPath, newPath string) error {
				order = append(order, filepath.Base(newPath))
				seen = append(seen, client.LoadWithoutRetry())

				return os.Rename(oldPath, newPath)
			})
			defer restore()

			_, err = client.Refresh(context.Background())
			require.NoError(t, err)

			require.Equal(t, test.order, order)
			require.Len(t, seen, len(test.want))

			for n, want := range test.want {
				if want == nil {
					assert.NoError(t, seen[n], "before %s is replaced", order[n])
				} else {
					assert.ErrorIs(t, seen[n], want, "before %s is replaced", order[n])
				}
			}

			assert.NoError(t, client.LoadWithoutRetry())
		})
	}
}
