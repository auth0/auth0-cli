package registry_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins/registry"
	"github.com/auth0/auth0-cli/internal/plugins/registry/registrytest"
)

// tufBase is the time that the scenarios of the TUF tests start at.
var tufBase = time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

// refreshedAt returns a client of the fixture that has refreshed at the given
// time and whose clock stands at that time.
func refreshedAt(t *testing.T, f *fixture, now time.Time) *registry.Client {
	t.Helper()

	client := f.client()
	client.Now = fixedClock(now)

	_, err := client.Refresh(context.Background())
	require.NoError(t, err)

	return client
}

// countingTransport counts the bytes of the bodies that are read from the
// responses to requests for target files.
type countingTransport struct {
	base http.RoundTripper
	read atomic.Int64
}

// RoundTrip sends the request and wraps the body of the response of a target.
func (c *countingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := c.base.RoundTrip(request)
	if err != nil || !strings.HasPrefix(request.URL.Path, "/targets/") {
		return response, err
	}

	response.Body = &countingBody{ReadCloser: response.Body, read: &c.read}

	return response, nil
}

// countingBody adds what is read from a body to a counter.
type countingBody struct {
	io.ReadCloser
	read *atomic.Int64
}

// Read reads from the body and counts the bytes.
func (b *countingBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.read.Add(int64(n))

	return n, err
}

func TestTUF_Fetch(t *testing.T) {
	f := newFixture(t)
	client := f.client()

	_, _, err := client.Load(t.Context())
	require.ErrorIs(t, err, registry.ErrNotCached)

	index, err := client.Refresh(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, index.Version)

	loaded, _, err := client.Load(t.Context())
	require.NoError(t, err)
	assert.Equal(t, index, loaded)

	files := f.cache()
	assert.NotContains(t, files, "roots/2.root.json", "no root after the embedded one was needed")
	assert.Equal(t, 1, f.server.Requests("/2.root.json"), "the end of the chain of roots was looked for once")
	assert.Equal(t, 1, f.server.Requests("/timestamp.json"))
}

func TestTUF_SigningKeyRotation(t *testing.T) {
	f := newFixture(t)
	client := refreshedAt(t, f, tufBase)

	f.repo.RotateKey(registrytest.RoleTargets)
	f.repo.RotateKey(registrytest.RoleSnapshot)
	f.repo.RotateKey(registrytest.RoleTimestamp)
	f.publish(f.builder().SetVersion(2))

	index, err := client.Refresh(context.Background())
	require.NoError(t, err, "the client that only knew the first root follows the rotation")
	assert.Equal(t, 2, index.Version)

	files := f.cache()
	assert.Equal(t, string(f.repo.File("4.root.json")), files["metadata/root.json"])

	for _, name := range []string{"2.root.json", "3.root.json", "4.root.json"} {
		assert.Equal(t, string(f.repo.File(name)), files["roots/"+name])
	}

	_, _, err = client.Load(t.Context())
	require.NoError(t, err, "the cache verifies through the chain of roots")

	t.Run("the next refresh starts from the last root", func(t *testing.T) {
		f.publish(f.builder().SetVersion(3))
		walked := f.server.Requests("/2.root.json")

		index, err := client.Refresh(context.Background())

		require.NoError(t, err)
		assert.Equal(t, 3, index.Version)
		assert.Equal(t, walked, f.server.Requests("/2.root.json"), "the roots that were walked are not downloaded again")
		assert.Equal(t, 2, f.server.Requests("/5.root.json"), "only the end of the chain is looked for")
	})
}

func TestTUF_RootChain(t *testing.T) {
	f := newFixture(t)
	f.repo.RotateKey(registrytest.RoleRoot)
	f.repo.RotateKey(registrytest.RoleRoot)
	f.publish(f.builder().SetVersion(2))

	client := f.client()
	index, err := client.Refresh(context.Background())

	require.NoError(t, err, "version 1 leads to version 3 through the root keys that stayed")
	assert.Equal(t, 2, index.Version)

	files := f.cache()
	assert.Equal(t, string(f.repo.File("3.root.json")), files["metadata/root.json"])
	assert.Equal(t, string(f.repo.File("2.root.json")), files["roots/2.root.json"])
	assert.Equal(t, string(f.repo.File("3.root.json")), files["roots/3.root.json"])

	_, _, err = client.Load(t.Context())
	assert.NoError(t, err)
}

func TestTUF_RetiredKeyIsRejected(t *testing.T) {
	for _, role := range []string{registrytest.RoleTargets, registrytest.RoleSnapshot, registrytest.RoleTimestamp} {
		t.Run(role, func(t *testing.T) {
			f := newFixture(t)
			client := refreshedAt(t, f, tufBase)
			cached := f.cache()

			f.repo.RotateKey(role)
			f.repo.SignWithRetiredKey(role)
			f.publish(f.builder().SetVersion(2))

			_, err := client.Refresh(context.Background())
			require.ErrorIs(t, err, registry.ErrSignatureInvalid)

			client.Now = fixedClock(tufBase.Add(2 * 24 * time.Hour))

			index, _, err := client.EnsureFresh(context.Background())
			assert.Nil(t, index, "a signature failure is not hidden by the cache")
			assert.ErrorIs(t, err, registry.ErrSignatureInvalid)
			assert.Equal(t, cached, f.cache(), "not even the root that was walked is stored")

			f.repo.SignWithCurrentKeys()
			f.publish(f.builder().SetVersion(2))

			index, err = client.Refresh(context.Background())
			require.NoError(t, err, "the current key is accepted")
			assert.Equal(t, 2, index.Version)
		})
	}
}

func TestTUF_Rollback(t *testing.T) {
	tests := []struct {
		name string

		// Serve has the registry, which published version 3 after the client
		// cached version 2, serve a file of an older version in its place.
		serve func(f *fixture, first []byte)
	}{
		{
			name:  "an older timestamp",
			serve: func(f *fixture, first []byte) { f.server.Set("/timestamp.json", first) },
		},
		{
			name:  "an older snapshot",
			serve: func(f *fixture, _ []byte) { f.server.Set("/3.snapshot.json", f.repo.File("2.snapshot.json")) },
		},
		{
			name:  "older targets metadata",
			serve: func(f *fixture, _ []byte) { f.server.Set("/3.targets.json", f.repo.File("2.targets.json")) },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			first := f.repo.File("timestamp.json")

			f.publish(f.builder().SetVersion(2))
			client := refreshedAt(t, f, tufBase)
			cached := f.cache()

			f.publish(f.builder().SetVersion(3))
			test.serve(f, first)

			_, err := client.Refresh(context.Background())
			require.ErrorIs(t, err, registry.ErrRollback)
			assert.Equal(t, cached, f.cache())

			client.Now = fixedClock(tufBase.Add(3 * 24 * time.Hour))
			index, status, err := client.EnsureFresh(context.Background())

			require.NoError(t, err, "a registry that goes back is treated like one that cannot be reached")
			assert.Equal(t, 2, index.Version)
			assert.True(t, status.Stale)
			assert.True(t, status.Runnable)
			assert.Equal(t, tufBase, status.LastRefresh)
			assert.Contains(t, status.Warning, "The registry index is older than one that was already accepted.")
		})
	}

	// The index of the older timestamp is the one that the client has, so only the
	// version of the metadata stands in the way, which go-tuf knows from the copy
	// of the cached metadata that it is given.
	t.Run("an older timestamp for the index that the client has", func(t *testing.T) {
		f := newFixture(t)
		first := f.repo.File("timestamp.json")

		f.publish(f.builder())
		client := refreshedAt(t, f, tufBase)
		cached := f.cache()

		f.server.Set("/timestamp.json", first)

		_, err := client.Refresh(context.Background())
		require.ErrorIs(t, err, registry.ErrRollback)
		assert.Equal(t, cached, f.cache())

		client.Now = fixedClock(tufBase.Add(3 * 24 * time.Hour))
		index, status, err := client.EnsureFresh(context.Background())

		require.NoError(t, err, "a registry that goes back is treated like one that cannot be reached")
		assert.Equal(t, 1, index.Version)
		assert.True(t, status.Stale)
		assert.True(t, status.Runnable)
		assert.Equal(t, tufBase, status.LastRefresh)
		assert.Contains(t, status.Warning, "The registry index is older than one that was already accepted.")
		assert.Equal(t, cached, f.cache(), "the time of the last refresh did not move")
	})
}

func TestTUF_OutOfStep(t *testing.T) {
	tests := []struct {
		name    string
		serve   func(f *fixture)
		wantErr error
	}{
		{
			name: "an index that is not the one the metadata names",
			serve: func(f *fixture) {
				f.server.Set(registrytest.IndexPath(f.builder().SetVersion(2).Build()), f.builder().SetVersion(1).Build())
			},
			wantErr: registry.ErrSignatureInvalid,
		},
		{
			name: "a snapshot of another repository",
			serve: func(f *fixture) {
				other := registrytest.NewRepo(t)
				registrytest.NewServer(t).Publish(f.builder().SetVersion(2), other)
				registrytest.NewServer(t).Publish(f.builder().SetVersion(2), other)
				f.server.Set("/2.snapshot.json", other.File("2.snapshot.json"))
			},
			wantErr: registry.ErrSignatureInvalid,
		},
		{
			name: "targets metadata of the first version under the second",
			serve: func(f *fixture) {
				f.server.Set("/2.targets.json", f.repo.File("1.targets.json"))
			},
			wantErr: registry.ErrRollback,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			client := refreshedAt(t, f, tufBase)
			cached := f.cache()

			f.publish(f.builder().SetVersion(2))
			test.serve(f)

			_, err := client.Refresh(context.Background())

			require.ErrorIs(t, err, test.wantErr)
			assert.Equal(t, cached, f.cache(), "a set of files that does not belong together is never trusted")

			// Once the registry serves files that belong together, the client moves on.
			f.publish(f.builder().SetVersion(2))

			index, err := client.Refresh(context.Background())
			require.NoError(t, err)
			assert.Equal(t, 2, index.Version)
		})
	}
}

func TestTUF_LeakedRootKey(t *testing.T) {
	tests := []struct {
		name   string
		attack func(f *fixture)
	}{
		{
			name: "a root signed by one key",
			attack: func(f *fixture) {
				f.repo.SignRootWith(1)
				f.repo.RotateKey(registrytest.RoleTargets)
			},
		},
		{
			name: "a root that installs keys of the attacker, signed by all of them and the key that leaked",
			attack: func(f *fixture) {
				f.repo.ReplaceRootKeys(1)
			},
		},
		{
			name: "a root that installs keys of the attacker, signed by the key that leaked only",
			attack: func(f *fixture) {
				f.repo.SignRootWith(1)
				f.repo.ReplaceRootKeys(1)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			client := refreshedAt(t, f, tufBase)
			cached := f.cache()

			test.attack(f)
			f.repo.RotateKey(registrytest.RoleTimestamp)
			f.publish(f.builder().SetVersion(2))

			_, err := client.Refresh(context.Background())
			require.ErrorIs(t, err, registry.ErrSignatureInvalid)

			client.Now = fixedClock(tufBase.Add(2 * 24 * time.Hour))

			index, _, err := client.EnsureFresh(context.Background())
			assert.Nil(t, index, "the cache does not stand in for a root that was forged")
			assert.ErrorIs(t, err, registry.ErrSignatureInvalid)
			assert.Equal(t, cached, f.cache())

			_, _, err = client.Load(t.Context())
			assert.NoError(t, err, "the cache still verifies against the first root")
		})
	}

	t.Run("a new client with no cache rejects it too", func(t *testing.T) {
		f := newFixture(t)
		f.repo.ReplaceRootKeys(1)
		f.publish(f.builder())

		_, err := f.client().Refresh(context.Background())

		require.ErrorIs(t, err, registry.ErrSignatureInvalid)
		assert.Empty(t, f.cache())
	})
}

func TestTUF_Freeze(t *testing.T) {
	timestampExpires := tufBase.Add(7 * 24 * time.Hour)

	// The helper "frozen" publishes a timestamp that expires after a week, and
	// caches the index at the start.
	frozen := func(t *testing.T) (*fixture, *registry.Client) {
		t.Helper()

		f := newFixture(t)
		f.repo.SetExpiry(registrytest.RoleTimestamp, timestampExpires)
		f.publish(f.builder())

		return f, refreshedAt(t, f, tufBase)
	}

	t.Run("a cache that is within the offline grace stays in use", func(t *testing.T) {
		tests := []struct {
			name      string
			republish func(f *fixture)
		}{
			{name: "the registry serves the timestamp that the client has", republish: func(*fixture) {}},
			{name: "the registry serves a newer timestamp that has also expired", republish: func(f *fixture) { f.publish(f.builder()) }},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				f, client := frozen(t)
				test.republish(f)

				client.Now = fixedClock(tufBase.Add(8 * 24 * time.Hour))

				_, err := client.Refresh(context.Background())
				require.ErrorIs(t, err, registry.ErrExpired)
				assert.Contains(t, err.Error(), "timestamp.json")

				index, status, err := client.EnsureFresh(context.Background())

				require.NoError(t, err)
				require.NotNil(t, index)
				assert.True(t, status.Stale)
				assert.True(t, status.Runnable)
				assert.Equal(t, tufBase, status.LastRefresh)
				assert.Contains(t, status.Warning, "The registry has not been re-signed since 2026-06-08T12:00:00Z")
				assert.NotContains(t, status.Warning, "index expired")
			})
		}
	})

	t.Run("a cache beyond the offline grace is not runnable", func(t *testing.T) {
		_, client := frozen(t)
		client.Now = fixedClock(tufBase.Add(31 * 24 * time.Hour))

		index, status, err := client.EnsureFresh(context.Background())

		require.NoError(t, err)
		require.NotNil(t, index)
		assert.False(t, status.Runnable)
		assert.Contains(t, status.Warning, "installed plugins cannot run")
	})

	t.Run("nothing cached is an error", func(t *testing.T) {
		f := newFixture(t)
		f.repo.SetExpiry(registrytest.RoleTimestamp, timestampExpires)
		f.publish(f.builder())

		client := f.client()
		client.Now = fixedClock(tufBase.Add(8 * 24 * time.Hour))

		index, _, err := client.EnsureFresh(context.Background())

		assert.Nil(t, index)
		require.ErrorIs(t, err, registry.ErrExpired)
		assert.Contains(t, err.Error(), "timestamp.json")
		assert.Empty(t, f.cache())
	})

	t.Run("the registry signs a timestamp again", func(t *testing.T) {
		f, client := frozen(t)
		f.repo.SetExpiry(registrytest.RoleTimestamp, time.Date(2099, time.January, 1, 0, 0, 0, 0, time.UTC))
		f.publish(f.builder())

		now := tufBase.Add(8 * 24 * time.Hour)
		client.Now = fixedClock(now)

		index, status, err := client.EnsureFresh(context.Background())

		require.NoError(t, err)
		require.NotNil(t, index)
		assert.False(t, status.Stale)
		assert.Equal(t, now, status.LastRefresh, "the cached timestamp had expired, and the refresh still worked")
	})
}

func TestTUF_ExpiredMetadataNeverFallsBackToTheCache(t *testing.T) {
	soon := tufBase.Add(24 * time.Hour)
	later := tufBase.Add(2 * 24 * time.Hour)

	tests := []struct {
		name string

		// Prepare makes the next publication carry the expiry, and file is the
		// metadata that the error names.
		prepare func(f *fixture)
		file    string
	}{
		{
			name:    "snapshot",
			prepare: func(f *fixture) { f.repo.SetExpiry(registrytest.RoleSnapshot, soon) },
			file:    "snapshot.json",
		},
		{
			name:    "targets",
			prepare: func(f *fixture) { f.repo.SetExpiry(registrytest.RoleTargets, soon) },
			file:    "targets.json",
		},
		{
			name: "root",
			prepare: func(f *fixture) {
				f.repo.SetExpiry(registrytest.RoleRoot, soon)
				f.repo.RotateKey(registrytest.RoleTimestamp)
			},
			file: "root.json",
		},
	}

	for _, test := range tests {
		for _, newer := range []bool{false, true} {
			name := test.name + " that the client has"
			if newer {
				name = test.name + " that is newer"
			}

			t.Run(name, func(t *testing.T) {
				f := newFixture(t)
				if !newer {
					test.prepare(f)
					f.publish(f.builder())
				}

				// The root that expires is the first one, so the client has to be
				// handed it before the metadata of the registry is published.
				client := refreshedAt(t, f, tufBase)

				if newer {
					test.prepare(f)
					f.publish(f.builder().SetVersion(2))
				}

				cached := f.cache()
				client.Now = fixedClock(later)

				_, err := client.Refresh(context.Background())
				require.ErrorIs(t, err, registry.ErrExpired)
				assert.Contains(t, err.Error(), test.file)
				assert.NotErrorIs(t, err, registry.ErrOffline)

				index, _, err := client.EnsureFresh(context.Background())
				assert.Nil(t, index, "the cache does not stand in for expired metadata")
				assert.ErrorIs(t, err, registry.ErrExpired)
				assert.Equal(t, cached, f.cache())
			})
		}
	}
}

func TestTUF_OfflineGrace(t *testing.T) {
	f := newFixture(t)
	client := refreshedAt(t, f, tufBase)
	state := f.readCache("state.json")

	f.server.Fail("/timestamp.json", http.StatusServiceUnavailable)

	t.Run("the cache is used and the time of the last refresh stands still", func(t *testing.T) {
		client.Now = fixedClock(tufBase.Add(3 * 24 * time.Hour))

		index, status, err := client.EnsureFresh(context.Background())

		require.NoError(t, err)
		require.NotNil(t, index)
		assert.True(t, status.Stale)
		assert.True(t, status.Runnable)
		assert.Equal(t, tufBase, status.LastRefresh)
		assert.Contains(t, status.Warning, "503 Service Unavailable")
		assert.Equal(t, state, f.readCache("state.json"))
	})

	t.Run("the index cannot run any more after 30 days", func(t *testing.T) {
		client.Now = fixedClock(tufBase.Add(30*24*time.Hour + time.Second))

		index, status, err := client.EnsureFresh(context.Background())

		require.NoError(t, err)
		require.NotNil(t, index)
		assert.False(t, status.Runnable)
		assert.Contains(t, status.Warning, "installed plugins cannot run")
	})

	t.Run("a registry that is back makes it current", func(t *testing.T) {
		f.server.Fail("/timestamp.json", 0)

		now := tufBase.Add(31 * 24 * time.Hour)
		client.Now = fixedClock(now)

		_, status, err := client.EnsureFresh(context.Background())

		require.NoError(t, err)
		assert.False(t, status.Stale)
		assert.True(t, status.Runnable)
		assert.Equal(t, now, status.LastRefresh)
	})
}

func TestTUF_NeverWritesIntoTheCacheWhenItFails(t *testing.T) {
	tests := []struct {
		name string

		// Sabotage runs after the client cached version 2 of the index, which is
		// the second publication, and gets the timestamp of the first one.
		sabotage func(f *fixture, first []byte)
	}{
		{name: "the registry cannot be reached", sabotage: func(f *fixture, _ []byte) {
			f.server.Fail("/timestamp.json", http.StatusBadGateway)
		}},
		{name: "an older timestamp", sabotage: func(f *fixture, first []byte) {
			f.publish(f.builder().SetVersion(3))
			f.server.Set("/timestamp.json", first)
		}},
		{name: "a root that was walked and metadata that does not verify", sabotage: func(f *fixture, _ []byte) {
			f.repo.RotateKey(registrytest.RoleTargets)
			f.repo.SignWithRetiredKey(registrytest.RoleTargets)
			f.publish(f.builder().SetVersion(3))
		}},
		{name: "a root below the threshold", sabotage: func(f *fixture, _ []byte) {
			f.repo.SignRootWith(1)
			f.repo.RotateKey(registrytest.RoleSnapshot)
			f.publish(f.builder().SetVersion(3))
		}},
		{name: "targets metadata that does not belong to the snapshot", sabotage: func(f *fixture, _ []byte) {
			f.publish(f.builder().SetVersion(3))
			f.server.Set("/3.targets.json", f.repo.File("2.targets.json"))
		}},
		{name: "an index that is not the one that was signed", sabotage: func(f *fixture, _ []byte) {
			f.publish(f.builder().SetVersion(3))
			f.server.Set(registrytest.IndexPath(f.builder().SetVersion(3).Build()), []byte("{}"))
		}},
		{name: "an index that was signed and is older", sabotage: func(f *fixture, _ []byte) {
			f.publish(f.builder().SetVersion(1))
		}},
		{name: "an index that is not valid", sabotage: func(f *fixture, _ []byte) {
			f.publishRaw([]byte("not json"))
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			first := f.repo.File("timestamp.json")

			// The cache holds a walked root too, so a root that goes missing or
			// changes would show.
			f.repo.RotateKey(registrytest.RoleSnapshot)
			f.publish(f.builder().SetVersion(2))

			client := refreshedAt(t, f, tufBase)
			require.Contains(t, f.cache(), "roots/2.root.json")

			cached := f.cache()
			test.sabotage(f, first)

			_, err := client.Refresh(context.Background())

			require.Error(t, err)
			assert.Equal(t, cached, f.cache(), "go-tuf works on a copy, and a failure stores nothing")
		})
	}
}

func TestTUF_SeededMetadataThatIsDamagedIsReported(t *testing.T) {
	f := newFixture(t)
	client := refreshedAt(t, f, tufBase)

	f.writeCache("metadata/timestamp.json", []byte("{"))

	_, err := client.Refresh(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "damaged")
	assert.NotErrorIs(t, err, registry.ErrOffline)
	assert.Equal(t, 1, f.server.Requests("/timestamp.json"), "the registry is not asked")

	_, _, err = client.Load(t.Context())
	assert.NoError(t, err, "the timestamp does not take part in what Load checks")
}

func TestTUF_IndexLongerThanTheSignedLength(t *testing.T) {
	f := newFixture(t)
	signed := f.builder().Build()

	// The body is far longer than what the targets metadata says, and far shorter
	// than the largest index the client would ever accept.
	oversized := append(append([]byte{}, signed...), strings.Repeat(" ", 2<<20)...)
	f.server.Set(registrytest.IndexPath(signed), oversized)

	counter := &countingTransport{base: f.server.Client().Transport}

	client := f.client()
	client.HTTP = &http.Client{Transport: counter}

	index, err := client.Refresh(context.Background())

	assert.Nil(t, index)
	require.ErrorIs(t, err, registry.ErrSignatureInvalid)
	assert.ErrorContains(t, err, fmt.Sprintf("larger than the allowed %d bytes", len(signed)))
	assert.LessOrEqual(t, counter.read.Load(), int64(len(signed))+1, "the download stops one byte after the signed length")
	assert.Empty(t, f.cache())
}

func TestTUF_DamagedCachedRoot(t *testing.T) {
	damage := func(name string) func(t *testing.T, f *fixture) {
		return func(t *testing.T, f *fixture) {
			t.Helper()

			f.writeCache(name, mutateIndex(t, f.readCache(name), setValue("2098-01-01T00:00:00Z", "signed", "expires")))
		}
	}

	tests := []struct {
		name   string
		damage func(t *testing.T, f *fixture)
	}{
		{name: "the first root after the embedded one", damage: damage("roots/2.root.json")},
		{name: "a root in the middle of the chain", damage: damage("roots/3.root.json")},
		{name: "the last root of the chain", damage: damage("roots/4.root.json")},
		{
			name: "a root after the last one",
			damage: func(_ *testing.T, f *fixture) {
				f.writeCache("roots/5.root.json", []byte("{"))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name+" is replaced by a refresh", func(t *testing.T) {
			f, client := rotatedFixture(t)
			pristine := f.cache()

			test.damage(t, f)
			require.NotEqual(t, pristine, f.cache())

			_, _, err := client.Load(t.Context())
			require.ErrorIs(t, err, registry.ErrSignatureInvalid, "the cache is not used as it is")

			index, err := client.Refresh(context.Background())

			require.NoError(t, err, "the roots that verify lead to the ones that the registry serves")
			assert.Equal(t, 2, index.Version)
			assert.Equal(t, pristine, f.cache(), "the damaged file is back as it was, and nothing is left over")

			_, _, err = client.Load(t.Context())
			assert.NoError(t, err)
		})

		t.Run(test.name+" is replaced by EnsureFresh", func(t *testing.T) {
			f, client := rotatedFixture(t)
			pristine := f.cache()

			test.damage(t, f)

			now := tufBase.Add(3 * 24 * time.Hour)
			client.Now = fixedClock(now)

			index, status, err := client.EnsureFresh(context.Background())

			require.NoError(t, err)
			require.NotNil(t, index)
			assert.False(t, status.Stale)
			assert.Equal(t, now, status.LastRefresh)

			healed := f.cache()
			assert.NotEqual(t, pristine["state.json"], healed["state.json"], "the refresh moved the time of the last refresh")

			delete(pristine, "state.json")
			delete(healed, "state.json")
			assert.Equal(t, pristine, healed)
		})

		t.Run(test.name+" is not used while the registry cannot be reached", func(t *testing.T) {
			f, client := rotatedFixture(t)
			test.damage(t, f)

			damaged := f.cache()
			f.server.Fail("/timestamp.json", http.StatusServiceUnavailable)

			for _, root := range []string{"/2.root.json", "/3.root.json", "/4.root.json", "/5.root.json"} {
				f.server.Fail(root, http.StatusServiceUnavailable)
			}

			_, err := client.Refresh(context.Background())
			require.ErrorIs(t, err, registry.ErrOffline)

			_, _, err = client.Load(t.Context())
			require.ErrorIs(t, err, registry.ErrSignatureInvalid)
			assert.ErrorContains(t, err, "remove "+f.dir+" to start over")

			client.Now = fixedClock(tufBase.Add(3 * 24 * time.Hour))
			index, _, err := client.EnsureFresh(context.Background())

			assert.Nil(t, index, "the cache does not stand in when its roots do not verify")
			require.ErrorIs(t, err, registry.ErrSignatureInvalid)
			assert.ErrorContains(t, err, "remove "+f.dir+" to start over")
			assert.Equal(t, damaged, f.cache())
		})
	}
}
