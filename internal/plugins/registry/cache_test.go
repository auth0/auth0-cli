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

	for _, name := range []string{"index.json", "index.json.sig", "state.json", ".lock"} {
		info, err := os.Stat(filepath.Join(f.dir, name))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), name)
	}

	entries, err := os.ReadDir(f.dir)
	require.NoError(t, err)
	assert.Len(t, entries, 4, "no temporary file is left behind")
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
		{name: "writing the signature fails", inject: func() func() { return failStaging("index.json.sig") }},
		{name: "writing the state fails", inject: func() func() { return failStaging("state.json") }},
		{name: "replacing the signature fails", inject: func() func() { return failRenaming("index.json.sig") }},
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
	f := newFixture(t)
	client := f.client()

	f.publish(f.builder().SetVersion(1))
	_, err := client.Refresh(context.Background())
	require.NoError(t, err)

	f.publish(f.builder().SetVersion(2))

	var (
		order []string
		seen  = map[string]error{}
	)

	restore := registry.SetRenameFile(func(oldPath, newPath string) error {
		name := filepath.Base(newPath)
		order = append(order, name)
		seen[name] = client.LoadWithoutRetry()

		return os.Rename(oldPath, newPath)
	})
	defer restore()

	_, err = client.Refresh(context.Background())
	require.NoError(t, err)

	assert.Equal(t, []string{"state.json", "index.json.sig", "index.json"}, order)
	assert.NoError(t, seen["state.json"], "before the first rename the cache is the old one")
	assert.ErrorIs(t, seen["index.json.sig"], registry.ErrRollback, "with the new state the old index is too old")
	assert.ErrorIs(t, seen["index.json"], registry.ErrSignatureInvalid, "with the new signature the old index does not verify")
	assert.NoError(t, client.LoadWithoutRetry())
}
