package registry_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins/registry"
	"github.com/auth0/auth0-cli/internal/plugins/registry/registrytest"
)

// rotatedFixture returns a fixture whose registry has rotated the targets key
// twice and a root key once, so that the root is version 4, and a client that
// has refreshed and so has the roots 2 to 4 in its cache.
func rotatedFixture(t *testing.T) (*fixture, *registry.Client) {
	t.Helper()

	f := newFixture(t)
	f.repo.RotateKey(registrytest.RoleTargets)
	f.repo.RotateKey(registrytest.RoleRoot)
	f.repo.RotateKey(registrytest.RoleTargets)
	f.publish(f.builder().SetVersion(2))

	client := refreshedAt(t, f, tufBase)
	require.Contains(t, f.cache(), "roots/4.root.json")

	return f, client
}

func TestLoad_ReVerifiesTheWholeCache(t *testing.T) {
	t.Run("an old client walks several cached roots", func(t *testing.T) {
		_, client := rotatedFixture(t)

		index, status, err := client.Load(t.Context())

		require.NoError(t, err)
		assert.Equal(t, 2, index.Version)
		assert.True(t, status.Runnable)
	})

	tests := []struct {
		name   string
		damage func(t *testing.T, f *fixture)
	}{
		{
			name: "a targets file with another version",
			damage: func(t *testing.T, f *fixture) {
				f.writeCache("metadata/targets.json", mutateIndex(t, f.readCache("metadata/targets.json"), setValue(9, "signed", "version")))
			},
		},
		{
			name: "a targets file of the key that was retired",
			damage: func(t *testing.T, f *fixture) {
				f.writeCache("metadata/targets.json", f.repo.File("1.targets.json"))
			},
		},
		{
			name: "a targets file that is not JSON",
			damage: func(_ *testing.T, f *fixture) {
				f.writeCache("metadata/targets.json", []byte("{"))
			},
		},
		{
			name: "an index with a changed byte",
			damage: func(_ *testing.T, f *fixture) {
				index := f.readCache("index.json")
				index[len(index)/2] ^= 0x01
				f.writeCache("index.json", index)
			},
		},
		{
			name: "an index that is longer than the metadata says",
			damage: func(_ *testing.T, f *fixture) {
				f.writeCache("index.json", append(f.readCache("index.json"), ' '))
			},
		},
		{
			name: "an index that is shorter than the metadata says",
			damage: func(_ *testing.T, f *fixture) {
				index := f.readCache("index.json")
				f.writeCache("index.json", index[:len(index)-1])
			},
		},
		{
			name: "a root with a changed byte",
			damage: func(t *testing.T, f *fixture) {
				f.writeCache("roots/3.root.json", mutateIndex(t, f.readCache("roots/3.root.json"), setValue("2098-01-01T00:00:00Z", "signed", "expires")))
			},
		},
		{
			name: "a root with the version of another one",
			damage: func(_ *testing.T, f *fixture) {
				f.writeCache("roots/2.root.json", f.repo.File("3.root.json"))
			},
		},
		{
			name: "a root that is not JSON",
			damage: func(_ *testing.T, f *fixture) {
				f.writeCache("roots/2.root.json", []byte("{"))
			},
		},
		{
			name: "a root in the middle of the chain that is gone",
			damage: func(_ *testing.T, f *fixture) {
				require.NoError(f.t, os.Remove(filepath.Join(f.dir, "roots", "2.root.json")))
			},
		},
		{
			name: "the last root of the chain that is gone",
			damage: func(_ *testing.T, f *fixture) {
				require.NoError(f.t, os.Remove(filepath.Join(f.dir, "roots", "4.root.json")))
			},
		},
		{
			name: "targets metadata of another repository",
			damage: func(t *testing.T, f *fixture) {
				other := registrytest.NewRepo(t)
				registrytest.NewServer(t).Publish(f.builder().SetVersion(2), other)
				f.writeCache("metadata/targets.json", other.File("1.targets.json"))
			},
		},
	}

	for _, test := range tests {
		t.Run("rejects "+test.name, func(t *testing.T) {
			f, client := rotatedFixture(t)
			test.damage(t, f)

			index, _, err := client.Load(t.Context())

			assert.Nil(t, index)
			assert.ErrorIs(t, err, registry.ErrSignatureInvalid)
		})
	}

	t.Run("rejects a root that fewer keys signed than the root before it needs", func(t *testing.T) {
		f := newFixture(t)
		client := refreshedAt(t, f, tufBase)

		f.repo.SignRootWith(1)
		f.repo.RotateKey(registrytest.RoleSnapshot)
		f.writeCache("roots/2.root.json", f.repo.File("2.root.json"))

		_, _, err := client.Load(t.Context())
		assert.ErrorIs(t, err, registry.ErrSignatureInvalid)
	})

	t.Run("rejects a root that the keys of the root before it do not vouch for", func(t *testing.T) {
		f := newFixture(t)
		client := refreshedAt(t, f, tufBase)

		f.repo.ReplaceRootKeys(1)
		f.writeCache("roots/2.root.json", f.repo.File("2.root.json"))

		_, _, err := client.Load(t.Context())

		require.ErrorIs(t, err, registry.ErrSignatureInvalid, "the new keys meet the threshold of the new root, and one old key does not meet that of the first")
		assert.ErrorContains(t, err, "not signed by the root before it")
		assert.ErrorContains(t, err, "remove "+f.dir+" to start over")
	})

	t.Run("rejects a root that its own keys do not vouch for", func(t *testing.T) {
		f := newFixture(t)
		client := refreshedAt(t, f, tufBase)

		f.repo.ReplaceRootKeysSignedByOldKeys(1)
		f.writeCache("roots/2.root.json", f.repo.File("2.root.json"))

		_, _, err := client.Load(t.Context())

		require.ErrorIs(t, err, registry.ErrSignatureInvalid, "the old keys meet the threshold of the first root, and one of them is in the new root")
		assert.ErrorContains(t, err, "not signed by its own keys")
		assert.ErrorContains(t, err, "remove "+f.dir+" to start over")
	})

	t.Run("rejects a cache that went back to an older index with older metadata", func(t *testing.T) {
		f := newFixture(t)
		client := refreshedAt(t, f, tufBase)
		older := f.cache()

		f.publish(f.builder().SetVersion(2))
		_, err := client.Refresh(context.Background())
		require.NoError(t, err)

		f.writeCache("index.json", []byte(older["index.json"]))
		f.writeCache("metadata/targets.json", []byte(older["metadata/targets.json"]))

		_, _, err = client.Load(t.Context())
		assert.ErrorIs(t, err, registry.ErrRollback, "the state remembers version 2, and everything else verifies")
	})

	t.Run("reports what is missing as not cached", func(t *testing.T) {
		for _, name := range []string{"state.json", "index.json", "metadata/targets.json"} {
			f, client := rotatedFixture(t)
			require.NoError(t, os.Remove(filepath.Join(f.dir, filepath.FromSlash(name))))

			_, _, err := client.Load(t.Context())
			assert.ErrorIs(t, err, registry.ErrNotCached, name)
		}
	})

	t.Run("does not use a cache without a root that vouches for it", func(t *testing.T) {
		_, client := rotatedFixture(t)
		client.Root = registrytest.NewRepo(t).Root()

		_, _, err := client.Load(t.Context())
		assert.ErrorIs(t, err, registry.ErrSignatureInvalid, "the keys of the other root do not vouch for these files")
	})
}

func TestLoad_Expiry(t *testing.T) {
	soon := tufBase.Add(24 * time.Hour)
	later := tufBase.Add(10 * 24 * time.Hour)
	far := time.Date(2099, time.January, 1, 0, 0, 0, 0, time.UTC)

	t.Run("rejects targets metadata that expired", func(t *testing.T) {
		f := newFixture(t)
		f.repo.SetExpiry(registrytest.RoleTargets, soon)
		f.publish(f.builder())

		client := refreshedAt(t, f, tufBase)
		client.Now = fixedClock(later)

		index, status, err := client.Load(t.Context())

		assert.Nil(t, index)
		require.ErrorIs(t, err, registry.ErrExpired)
		assert.Contains(t, err.Error(), "targets.json")
		assert.False(t, status.Runnable)
	})

	t.Run("rejects the last root when it expired", func(t *testing.T) {
		f := newFixture(t)
		f.repo.SetNextRootExpiry(far)
		f.repo.RotateKey(registrytest.RoleTimestamp)
		f.repo.SetNextRootExpiry(soon)
		f.repo.RotateKey(registrytest.RoleTimestamp)
		f.publish(f.builder())

		client := f.client()
		client.Now = fixedClock(tufBase)
		_, err := client.Refresh(context.Background())
		require.NoError(t, err)
		require.Contains(t, f.cache(), "roots/3.root.json")

		client.Now = fixedClock(later)
		index, _, err := client.Load(t.Context())

		assert.Nil(t, index)
		require.ErrorIs(t, err, registry.ErrExpired)
		assert.Contains(t, err.Error(), "root.json")
	})

	t.Run("accepts a root in the middle of the chain that expired", func(t *testing.T) {
		f := newFixture(t)
		f.repo.SetNextRootExpiry(soon)
		f.repo.RotateKey(registrytest.RoleTimestamp)
		f.repo.SetNextRootExpiry(far)
		f.repo.RotateKey(registrytest.RoleTimestamp)
		f.publish(f.builder())

		client := f.client()
		client.Now = fixedClock(tufBase)
		_, err := client.Refresh(context.Background())
		require.NoError(t, err)
		require.Contains(t, f.cache(), "roots/3.root.json")

		cached := f.cache()
		assert.Equal(t, soon.Format(time.RFC3339), follow(decodeIndex(t, []byte(cached["roots/2.root.json"])), []any{"signed", "expires"}), "the first root after the embedded one expires soon")
		assert.Equal(t, far.Format(time.RFC3339), follow(decodeIndex(t, []byte(cached["roots/3.root.json"])), []any{"signed", "expires"}), "the last root does not")

		client.Now = fixedClock(later)
		index, _, err := client.Load(t.Context())

		require.NoError(t, err, "the root in the middle expired and the last one did not")
		assert.NotNil(t, index)
	})

	t.Run("ignores the expiry of the timestamp and of the snapshot", func(t *testing.T) {
		f := newFixture(t)
		f.repo.SetExpiry(registrytest.RoleTimestamp, soon)
		f.repo.SetExpiry(registrytest.RoleSnapshot, soon)
		f.publish(f.builder())

		client := refreshedAt(t, f, tufBase)
		client.Now = fixedClock(later)

		index, status, err := client.Load(t.Context())

		require.NoError(t, err)
		assert.NotNil(t, index)
		assert.True(t, status.Runnable)
	})

	t.Run("does not look at the timestamp and the snapshot at all", func(t *testing.T) {
		f := newFixture(t)
		client := refreshedAt(t, f, tufBase)

		require.NoError(t, os.Remove(filepath.Join(f.dir, "metadata", "timestamp.json")))
		f.writeCache("metadata/snapshot.json", []byte("not json"))

		_, _, err := client.Load(t.Context())
		assert.NoError(t, err)
	})

	t.Run("rejects an index that expired", func(t *testing.T) {
		f := newFixture(t)
		f.publish(f.builder().SetExpires(soon))

		client := refreshedAt(t, f, tufBase)
		client.Now = fixedClock(later)

		index, _, err := client.Load(t.Context())

		assert.Nil(t, index)
		assert.ErrorIs(t, err, registry.ErrExpired)
	})
}
