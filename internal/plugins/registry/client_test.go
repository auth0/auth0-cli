package registry_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins/registry"
	"github.com/auth0/auth0-cli/internal/plugins/registry/registrytest"
)

func TestClient_Refresh_Valid(t *testing.T) {
	f := newFixture(t)
	client := f.client()

	index, err := client.Refresh(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 1, index.Version)
	plugin, found := index.FindPlugin(testPluginID)
	require.True(t, found)
	assert.Equal(t, "2.0.0", plugin.Latest)

	t.Run("stores the index, the signature and the state", func(t *testing.T) {
		files := f.cache()
		assert.Len(t, files, 3)
		assert.Equal(t, string(f.builder().SetExpires(index.Expires).Build()), files["index.json"])
		assert.NotEmpty(t, files["index.json.sig"])

		var state map[string]any
		require.NoError(t, json.Unmarshal([]byte(files["state.json"]), &state))
		assert.EqualValues(t, 1, state["highestVersion"])
		assert.NotEmpty(t, state["lastRefresh"])
	})

	t.Run("loads the same index from the cache", func(t *testing.T) {
		loaded, status, err := client.Load(t.Context())
		require.NoError(t, err)

		assert.Equal(t, index, loaded)
		assert.Equal(t, 1, status.Version)
		assert.Equal(t, index.Expires, status.Expires)
		assert.False(t, status.Stale)
		assert.True(t, status.Runnable)
		assert.Empty(t, status.Warning)
	})

	t.Run("EnsureFresh does not use the network for a fresh cache", func(t *testing.T) {
		before := f.server.Requests("/index.json")

		ensured, status, err := client.EnsureFresh(context.Background())
		require.NoError(t, err)

		assert.Equal(t, index, ensured)
		assert.False(t, status.Stale)
		assert.Equal(t, before, f.server.Requests("/index.json"))
	})
}

// signatureDocument returns the content of an index.json.sig file.
func signatureDocument(t *testing.T, keyID, algorithm, signature string) []byte {
	t.Helper()

	document, err := json.Marshal(map[string]string{"keyId": keyID, "algorithm": algorithm, "signature": signature})
	require.NoError(t, err)

	return document
}

func TestClient_Refresh_SignatureFailures(t *testing.T) {
	otherID, _, otherPrivate := registrytest.NewKeyPair(t)

	tests := []struct {
		name  string
		serve func(f *fixture)
		keys  registry.KeySet
	}{
		{
			name: "tampered index",
			serve: func(f *fixture) {
				f.server.Set("/index.json", mutateIndex(t, f.builder().Build(), setValue(99, "version")))
			},
		},
		{
			name: "tampered signature",
			serve: func(f *fixture) {
				index := f.builder().Build()
				signature := registrytest.Sign(index, f.keyID, f.private)
				f.server.Set("/index.json", index)
				f.server.Set("/index.json.sig", bytes.Replace(signature, []byte(`"signature":"`), []byte(`"signature":"AAAA`), 1))
			},
		},
		{
			name: "signed by another key with a known key ID",
			serve: func(f *fixture) {
				index := f.builder().Build()
				f.server.Set("/index.json", index)
				f.server.Set("/index.json.sig", registrytest.Sign(index, f.keyID, otherPrivate))
			},
		},
		{
			name: "unknown key ID",
			serve: func(f *fixture) {
				index := f.builder().Build()
				f.server.Set("/index.json", index)
				f.server.Set("/index.json.sig", registrytest.Sign(index, otherID, otherPrivate))
			},
		},
		{
			name: "unknown algorithm",
			serve: func(f *fixture) {
				f.server.Set("/index.json.sig", signatureDocument(t, f.keyID, "rsa", base64.StdEncoding.EncodeToString(make([]byte, 64))))
			},
		},
		{
			name: "signature that is not base64",
			serve: func(f *fixture) {
				f.server.Set("/index.json.sig", signatureDocument(t, f.keyID, "ed25519", "not base64!"))
			},
		},
		{
			name: "signature of the wrong length",
			serve: func(f *fixture) {
				f.server.Set("/index.json.sig", signatureDocument(t, f.keyID, "ed25519", base64.StdEncoding.EncodeToString(make([]byte, 63))))
			},
		},
		{
			name: "signature file that is not JSON",
			serve: func(f *fixture) {
				f.server.Set("/index.json.sig", []byte("garbage"))
			},
		},
		{
			name: "key that is not the right size",
			serve: func(f *fixture) {
				f.keys = registry.KeySet{f.keyID: []byte("short")}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			test.serve(f)

			_, err := f.client().Refresh(context.Background())

			require.ErrorIs(t, err, registry.ErrSignatureInvalid)
			assert.Contains(t, err.Error(), "The auth0 registry index failed signature verification.")
			assert.Empty(t, f.cache(), "nothing is written before the index is verified")
			assert.Equal(t, 2, f.server.Requests("/index.json"), "the pair is retried once")
		})
	}
}

func TestClient_Refresh_NoTrustedKeys(t *testing.T) {
	for name, keys := range map[string]registry.KeySet{"nil": nil, "empty": {}} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			f.keys = keys

			_, err := f.client().Refresh(context.Background())
			require.ErrorIs(t, err, registry.ErrNoTrustedKeys)
			assert.EqualError(t, err, "no trusted registry keys are embedded in this build")
			assert.Zero(t, f.server.Requests("/index.json"), "nothing is fetched")

			_, _, err = f.client().Load(t.Context())
			assert.ErrorIs(t, err, registry.ErrNoTrustedKeys)

			_, _, err = f.client().EnsureFresh(context.Background())
			assert.ErrorIs(t, err, registry.ErrNoTrustedKeys)
			assert.NoDirExists(t, f.dir, "not even the lock is created")
		})
	}
}

func TestClient_Refresh_KeyRotation(t *testing.T) {
	f := newFixture(t)
	secondID, secondKeys, secondPrivate := registrytest.NewKeyPair(t)

	f.keys = registry.KeySet{f.keyID: f.keys[f.keyID], secondID: secondKeys[secondID]}

	t.Run("accepts an index signed with the second key", func(t *testing.T) {
		f.server.Publish(f.builder(), secondID, secondPrivate)

		_, err := f.client().Refresh(context.Background())
		assert.NoError(t, err)
	})

	t.Run("still accepts an index signed with the first key", func(t *testing.T) {
		f.publish(f.builder())

		_, err := f.client().Refresh(context.Background())
		assert.NoError(t, err)
	})

	t.Run("rejects the first key once it is no longer embedded", func(t *testing.T) {
		f.keys = secondKeys

		_, err := f.client().Refresh(context.Background())
		assert.ErrorIs(t, err, registry.ErrSignatureInvalid)

		_, _, err = f.client().Load(t.Context())
		assert.ErrorIs(t, err, registry.ErrSignatureInvalid, "the cache is verified again with the current keys")
	})
}

func TestClient_Refresh_Expiry(t *testing.T) {
	expires := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		now     time.Time
		wantErr bool
	}{
		{name: "a second before it expires", now: expires.Add(-time.Second)},
		{name: "exactly when it expires", now: expires, wantErr: true},
		{name: "after it expired", now: expires.Add(time.Hour), wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			f.publish(f.builder().SetExpires(expires))

			client := f.client()
			client.Now = fixedClock(test.now)

			_, err := client.Refresh(context.Background())
			if !test.wantErr {
				require.NoError(t, err)
				return
			}

			require.ErrorIs(t, err, registry.ErrExpired)
			assert.Contains(t, err.Error(), "The registry index expired on 2026-12-01T00:00:00Z")
			assert.Empty(t, f.cache(), "nothing is written before the index is verified")
		})
	}

	t.Run("Load reports an expired cache without returning the index", func(t *testing.T) {
		f := newFixture(t)
		f.publish(f.builder().SetExpires(expires))

		client := f.client()
		client.Now = fixedClock(expires.Add(-time.Hour))
		_, err := client.Refresh(context.Background())
		require.NoError(t, err)

		client.Now = fixedClock(expires)
		index, status, err := client.Load(t.Context())

		require.ErrorIs(t, err, registry.ErrExpired)
		assert.Nil(t, index)
		assert.Equal(t, expires, status.Expires)
		assert.False(t, status.Runnable)
	})
}

func TestClient_Refresh_Rollback(t *testing.T) {
	f := newFixture(t)
	client := f.client()

	f.publish(f.builder().SetVersion(10))
	_, err := client.Refresh(context.Background())
	require.NoError(t, err)

	cached := f.cache()

	t.Run("rejects an older index and leaves the cache as it was", func(t *testing.T) {
		f.publish(f.builder().SetVersion(9))

		_, err := client.Refresh(context.Background())

		require.ErrorIs(t, err, registry.ErrRollback)
		assert.Equal(t, cached, f.cache())
	})

	t.Run("accepts the same version", func(t *testing.T) {
		f.publish(f.builder().SetVersion(10))

		index, err := client.Refresh(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 10, index.Version)
	})

	t.Run("accepts a newer version and remembers it", func(t *testing.T) {
		f.publish(f.builder().SetVersion(11))

		_, err := client.Refresh(context.Background())
		require.NoError(t, err)
		assert.Contains(t, string(f.readCache("state.json")), `"highestVersion": 11`)
	})

	t.Run("Load rejects a cached index older than the highest version seen", func(t *testing.T) {
		older := f.builder().SetVersion(5).Build()
		f.writeCache("index.json", older)
		f.writeCache("index.json.sig", registrytest.Sign(older, f.keyID, f.private))

		_, _, err := client.Load(t.Context())
		assert.ErrorIs(t, err, registry.ErrRollback)
	})
}

func TestClient_Refresh_Format(t *testing.T) {
	t.Run("does not look at the format version of an unsigned index", func(t *testing.T) {
		f := newFixture(t)
		f.server.Set("/index.json", mutateIndex(t, f.builder().Build(), setValue(2, "formatVersion")))

		_, err := f.client().Refresh(context.Background())
		assert.ErrorIs(t, err, registry.ErrSignatureInvalid)
	})

	t.Run("rejects a signed index of a newer format and stores nothing", func(t *testing.T) {
		f := newFixture(t)
		f.publishRaw(mutateIndex(t, f.builder().Build(), setValue(2, "formatVersion")))

		_, err := f.client().Refresh(context.Background())

		require.ErrorIs(t, err, registry.ErrUnsupportedFormat)
		assert.Empty(t, f.cache(), "nothing is written before the index is verified")
	})
}

// The rules that make an index invalid are tested with ParseIndex. This test
// only checks what Refresh does with a signed index that breaks one of them.
func TestClient_Refresh_InvalidIndex(t *testing.T) {
	f := newFixture(t)
	f.publishRaw(mutateIndex(t, f.builder().Build(), removeValue("plugins", 0, "id")))

	_, err := f.client().Refresh(context.Background())

	requireInvalid(t, err, "plugins[0].id: required field is missing")
	assert.Equal(t, 1, f.server.Requests("/index.json"), "a validly signed index is not fetched again")
	assert.Empty(t, f.cache(), "nothing is written before the index is verified")
}

func TestClient_Versions(t *testing.T) {
	refPlugin := func() registry.Plugin {
		return registrytest.NewPlugin(testPluginID, registrytest.NewVersion("1.0.0"), registrytest.NewVersion("2.0.0"))
	}

	refreshPlugin := func(t *testing.T, f *fixture) (*registry.Client, registry.Plugin) {
		t.Helper()

		client := f.client()
		index, err := client.Refresh(context.Background())
		require.NoError(t, err)

		plugin, found := index.FindPlugin(testPluginID)
		require.True(t, found)

		return client, plugin
	}

	t.Run("returns the versions listed in the index without the network", func(t *testing.T) {
		f := newFixture(t)
		client, plugin := refreshPlugin(t, f)

		versions, err := client.Versions(context.Background(), plugin)

		require.NoError(t, err)
		assert.Equal(t, plugin.Versions, versions)
		assert.Zero(t, f.server.Requests("/plugins/checkmate.json"))
	})

	t.Run("downloads the file that the ref points to", func(t *testing.T) {
		f := newFixture(t)
		f.publish(registrytest.NewIndexBuilder().AddRefPlugin(refPlugin()))
		client, plugin := refreshPlugin(t, f)

		require.NotNil(t, plugin.Ref)
		assert.Equal(t, "plugins/checkmate.json", plugin.Ref.Path)
		assert.Empty(t, plugin.Versions)

		versions, err := client.Versions(context.Background(), plugin)

		require.NoError(t, err)
		assert.Equal(t, refPlugin().Versions, versions)
	})

	t.Run("resolves the path relative to the index", func(t *testing.T) {
		f := newFixture(t)
		builder := registrytest.NewIndexBuilder().AddRefPlugin(refPlugin())
		index := builder.Build()
		f.server.Set("/a/b/index.json", index)
		f.server.Set("/a/b/index.json.sig", registrytest.Sign(index, f.keyID, f.private))
		f.server.Set("/a/b/plugins/checkmate.json", builder.Files()["plugins/checkmate.json"])

		client := f.client()
		client.Registry.URL = f.server.URL() + "/a/b/index.json"
		loaded, err := client.Refresh(context.Background())
		require.NoError(t, err)
		plugin, _ := loaded.FindPlugin(testPluginID)

		versions, err := client.Versions(context.Background(), plugin)

		require.NoError(t, err)
		assert.Len(t, versions, 2)
		assert.Equal(t, 1, f.server.Requests("/a/b/plugins/checkmate.json"))
	})

	t.Run("rejects a file that does not have the pinned hash", func(t *testing.T) {
		f := newFixture(t)
		builder := registrytest.NewIndexBuilder().AddRefPlugin(refPlugin())
		f.publish(builder)
		client, plugin := refreshPlugin(t, f)

		f.server.Set("/plugins/checkmate.json", []byte(`{"versions":[]}`))

		versions, err := client.Versions(context.Background(), plugin)

		require.ErrorIs(t, err, registry.ErrHashMismatch)
		assert.Nil(t, versions)
	})

	t.Run("rejects a file that is too big before looking at its hash", func(t *testing.T) {
		f := newFixture(t)
		f.publish(registrytest.NewIndexBuilder().AddRefPlugin(refPlugin()))
		client, plugin := refreshPlugin(t, f)

		f.server.Set("/plugins/checkmate.json", bytes.Repeat([]byte(" "), 8<<20+1))

		_, err := client.Versions(context.Background(), plugin)

		require.ErrorIs(t, err, registry.ErrInvalidIndex)
		assert.Contains(t, err.Error(), "larger than the allowed")
	})

	t.Run("applies the semantic checks to the file", func(t *testing.T) {
		wrongHash := registrytest.NewVersion("1.0.0")
		wrongHash.PermissionsHash = "sha256:" + string(bytes.Repeat([]byte("0"), 64))
		duplicate := registrytest.NewVersion("2.0.0")

		tests := []struct {
			name     string
			versions []registry.VersionRecord
			wantErr  string
		}{
			{name: "a wrong permissions hash", versions: []registry.VersionRecord{wrongHash, duplicate}, wantErr: "versions[0].permissionsHash:"},
			{name: "a duplicate version", versions: []registry.VersionRecord{duplicate, duplicate}, wantErr: "versions[1].version: duplicate version"},
			{name: "a missing latest version", versions: []registry.VersionRecord{registrytest.NewVersion("1.0.0")}, wantErr: `versions: the latest version "2.0.0" of the plugin is not listed`},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				f := newFixture(t)

				file, err := json.Marshal(map[string]any{"versions": test.versions})
				require.NoError(t, err)

				plugin := refPlugin()
				plugin.Versions = nil
				plugin.Ref = &registry.Ref{Path: "plugins/checkmate.json", SHA256: registrytest.SHA256Hex(file)}
				f.publish(registrytest.NewIndexBuilder().AddPlugin(plugin))
				f.server.Set("/plugins/checkmate.json", file)

				client, loaded := refreshPlugin(t, f)
				_, err = client.Versions(context.Background(), loaded)

				require.ErrorIs(t, err, registry.ErrInvalidIndex)
				assert.Contains(t, err.Error(), test.wantErr)
			})
		}
	})
}

func TestClient_Refresh_Limits(t *testing.T) {
	t.Run("rejects an index that is too big", func(t *testing.T) {
		f := newFixture(t)
		f.server.Set("/index.json", bytes.Repeat([]byte(" "), 8<<20+1))

		_, err := f.client().Refresh(context.Background())

		require.ErrorIs(t, err, registry.ErrInvalidIndex)
		assert.Contains(t, err.Error(), "larger than the allowed 8388608 bytes")
		assert.Empty(t, f.cache(), "nothing is written before the index is verified")
	})

	t.Run("accepts an index of exactly the limit", func(t *testing.T) {
		f := newFixture(t)
		index := f.builder().Build()
		padded := append(append([]byte{}, index...), bytes.Repeat([]byte(" "), 8<<20-len(index))...)
		f.publishRaw(padded)

		_, err := f.client().Refresh(context.Background())
		assert.NoError(t, err)
	})

	t.Run("rejects a signature that is too big", func(t *testing.T) {
		f := newFixture(t)
		f.server.Set("/index.json.sig", bytes.Repeat([]byte(" "), 16<<10+1))

		_, err := f.client().Refresh(context.Background())

		require.ErrorIs(t, err, registry.ErrInvalidIndex)
		assert.Contains(t, err.Error(), "larger than the allowed 16384 bytes")
		assert.Empty(t, f.cache(), "nothing is written before the index is verified")
	})
}

func TestClient_Refresh_HTTPFailures(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		status   int
		truncate bool
		wantErr  string
	}{
		{name: "an index that is not found", path: "/index.json", status: http.StatusNotFound, wantErr: "404 Not Found"},
		{name: "an index that fails", path: "/index.json", status: http.StatusInternalServerError, wantErr: "500 Internal Server Error"},
		{name: "a signature that is not found", path: "/index.json.sig", status: http.StatusNotFound, wantErr: "404 Not Found"},
		{name: "a signature that fails", path: "/index.json.sig", status: http.StatusBadGateway, wantErr: "502 Bad Gateway"},
		{name: "an index that is cut short", path: "/index.json", truncate: true, wantErr: "EOF"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			if test.truncate {
				f.server.Truncate(test.path)
			} else {
				f.server.Fail(test.path, test.status)
			}

			_, err := f.client().Refresh(context.Background())

			require.ErrorIs(t, err, registry.ErrOffline)
			assert.Contains(t, err.Error(), "The registry could not be reached at "+f.server.Registry().URL[:len(f.server.URL())])
			assert.Contains(t, err.Error(), test.wantErr)
			assert.Empty(t, f.cache(), "nothing is written before the index is verified")
		})
	}

	t.Run("a server that cannot be reached", func(t *testing.T) {
		f := newFixture(t)
		f.server = registrytest.NewServer(t)
		client := f.client()
		client.Registry.URL = "https://127.0.0.1:1/index.json"

		_, err := client.Refresh(context.Background())

		require.ErrorIs(t, err, registry.ErrOffline)
		assert.Contains(t, err.Error(), "https://127.0.0.1:1/index.json")
	})

	t.Run("a cancelled context is not reported as offline", func(t *testing.T) {
		f := newFixture(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := f.client().Refresh(ctx)

		require.ErrorIs(t, err, context.Canceled)
		assert.NotErrorIs(t, err, registry.ErrOffline)
	})

	t.Run("the deadline of the context is reported as offline", func(t *testing.T) {
		f := newFixture(t)
		f.server.OnRequest(func(string, int) { time.Sleep(200 * time.Millisecond) })

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		_, err := f.client().Refresh(ctx)

		require.ErrorIs(t, err, registry.ErrOffline)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})
}

func TestClient_Refresh_Redirects(t *testing.T) {
	t.Run("rejects a redirect to http", func(t *testing.T) {
		f := newFixture(t)
		f.server.Redirect("/index.json", "http://example.invalid/index.json")

		_, err := f.client().Refresh(context.Background())

		require.ErrorIs(t, err, registry.ErrInsecureURL)
		assert.NotErrorIs(t, err, registry.ErrOffline)
		assert.Empty(t, f.cache(), "nothing is written before the index is verified")
	})

	t.Run("rejects a redirect to http even when the HTTP client allows it", func(t *testing.T) {
		f := newFixture(t)
		f.server.Redirect("/index.json.sig", "http://example.invalid/index.json.sig")

		client := f.client()
		client.HTTP.CheckRedirect = func(*http.Request, []*http.Request) error { return nil }

		_, err := client.Refresh(context.Background())
		assert.ErrorIs(t, err, registry.ErrInsecureURL)
	})

	t.Run("follows a redirect to https", func(t *testing.T) {
		f := newFixture(t)
		f.server.Redirect("/index.json", "/moved/index.json")
		f.server.Redirect("/index.json.sig", "/moved/index.json.sig")

		index := f.builder().Build()
		f.server.Set("/moved/index.json", index)
		f.server.Set("/moved/index.json.sig", registrytest.Sign(index, f.keyID, f.private))

		_, err := f.client().Refresh(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 1, f.server.Requests("/moved/index.json"))
		assert.Equal(t, 1, f.server.Requests("/moved/index.json.sig"))
	})

	t.Run("stops redirect loops", func(t *testing.T) {
		f := newFixture(t)
		f.server.Redirect("/index.json", "/index.json")

		_, err := f.client().Refresh(context.Background())
		assert.Error(t, err)
	})
}

func TestClient_Refresh_PairRetry(t *testing.T) {
	t.Run("succeeds when the registry republishes between the two requests", func(t *testing.T) {
		f := newFixture(t)
		f.server.OnRequest(func(path string, count int) {
			if path == "/index.json.sig" && count == 1 {
				f.publish(f.builder().SetVersion(2))
			}
		})

		index, err := f.client().Refresh(context.Background())

		require.NoError(t, err)
		assert.Equal(t, 2, index.Version)
		assert.Equal(t, 2, f.server.Requests("/index.json"))
		assert.Equal(t, 2, f.server.Requests("/index.json.sig"))
	})

	t.Run("gives up after the second attempt", func(t *testing.T) {
		f := newFixture(t)
		f.server.Set("/index.json", []byte(`{"formatVersion":1}`))

		_, err := f.client().Refresh(context.Background())

		require.ErrorIs(t, err, registry.ErrSignatureInvalid)
		assert.Equal(t, 2, f.server.Requests("/index.json"))
		assert.Equal(t, 2, f.server.Requests("/index.json.sig"))
	})

	t.Run("does not retry when the failure is not a signature failure", func(t *testing.T) {
		f := newFixture(t)
		f.publishRaw([]byte("not json"))

		_, err := f.client().Refresh(context.Background())

		require.ErrorIs(t, err, registry.ErrInvalidIndex)
		assert.Equal(t, 1, f.server.Requests("/index.json"))
	})
}

func TestClient_Load_DetectsCacheTampering(t *testing.T) {
	setup := func(t *testing.T) (*fixture, *registry.Client) {
		t.Helper()

		f := newFixture(t)
		client := f.client()
		_, err := client.Refresh(context.Background())
		require.NoError(t, err)

		return f, client
	}

	t.Run("a flipped byte in the cached index", func(t *testing.T) {
		f, client := setup(t)
		index := f.readCache("index.json")
		index[len(index)/2] ^= 0x01
		f.writeCache("index.json", index)

		_, _, err := client.Load(t.Context())
		assert.ErrorIs(t, err, registry.ErrSignatureInvalid)
	})

	t.Run("a cached index with a higher version signed by nobody", func(t *testing.T) {
		f, client := setup(t)
		f.writeCache("index.json", mutateIndex(t, f.readCache("index.json"), setValue(50, "version")))

		_, _, err := client.Load(t.Context())
		assert.ErrorIs(t, err, registry.ErrSignatureInvalid)
	})

	t.Run("a replaced signature", func(t *testing.T) {
		f, client := setup(t)
		otherID, _, otherPrivate := registrytest.NewKeyPair(t)
		f.writeCache("index.json.sig", registrytest.Sign(f.readCache("index.json"), otherID, otherPrivate))

		_, _, err := client.Load(t.Context())
		assert.ErrorIs(t, err, registry.ErrSignatureInvalid)
	})

	t.Run("a signature from another index", func(t *testing.T) {
		f, client := setup(t)
		f.writeCache("index.json.sig", registrytest.Sign(f.builder().SetVersion(7).Build(), f.keyID, f.private))

		_, _, err := client.Load(t.Context())
		assert.ErrorIs(t, err, registry.ErrSignatureInvalid)
	})

	t.Run("a missing state file", func(t *testing.T) {
		f, client := setup(t)
		require.NoError(t, os.Remove(filepath.Join(f.dir, "state.json")))

		_, _, err := client.Load(t.Context())
		assert.ErrorIs(t, err, registry.ErrNotCached)
	})

	t.Run("a missing index", func(t *testing.T) {
		f, client := setup(t)
		require.NoError(t, os.Remove(filepath.Join(f.dir, "index.json")))

		_, _, err := client.Load(t.Context())
		assert.ErrorIs(t, err, registry.ErrNotCached)
	})

	t.Run("nothing cached yet", func(t *testing.T) {
		f := newFixture(t)

		_, _, err := f.client().Load(t.Context())
		assert.ErrorIs(t, err, registry.ErrNotCached)
	})

	t.Run("EnsureFresh replaces a tampered cache when the registry is reachable", func(t *testing.T) {
		f, client := setup(t)
		f.writeCache("index.json", []byte("tampered"))

		index, status, err := client.EnsureFresh(context.Background())

		require.NoError(t, err)
		assert.Equal(t, 1, index.Version)
		assert.False(t, status.Stale)

		_, _, err = client.Load(t.Context())
		assert.NoError(t, err)
	})

	t.Run("EnsureFresh does not use a tampered cache when the registry is not reachable", func(t *testing.T) {
		f, client := setup(t)
		f.writeCache("index.json", []byte("tampered"))
		f.server.Fail("/index.json", http.StatusInternalServerError)

		index, _, err := client.EnsureFresh(context.Background())

		assert.Nil(t, index)
		assert.ErrorIs(t, err, registry.ErrOffline)
		assert.ErrorIs(t, err, registry.ErrSignatureInvalid)
	})
}

func TestClient_Refresh_FailureKeepsCache(t *testing.T) {
	f := newFixture(t)
	client := f.client()

	f.publish(f.builder().SetVersion(4))
	_, err := client.Refresh(context.Background())
	require.NoError(t, err)

	cached := f.cache()

	tests := []struct {
		name     string
		sabotage func()
	}{
		{name: "an unreachable registry", sabotage: func() { f.server.Fail("/index.json", http.StatusServiceUnavailable) }},
		{name: "a bad signature", sabotage: func() { f.server.Set("/index.json", []byte("{}")) }},
		{name: "an older index", sabotage: func() { f.publish(f.builder().SetVersion(3)) }},
		{name: "an expired index", sabotage: func() { f.publish(f.builder().SetVersion(5).SetExpires(time.Now().Add(-time.Hour))) }},
		{name: "an invalid index", sabotage: func() {
			f.publishRaw(mutateIndex(t, f.builder().SetVersion(6).Build(), removeValue("plugins", 0, "id")))
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f.publish(f.builder().SetVersion(4))
			f.server.Fail("/index.json", 0)

			test.sabotage()

			_, err := client.Refresh(context.Background())

			require.Error(t, err)
			assert.Equal(t, cached, f.cache(), "the cache, including the highest version, is untouched")
		})
	}
}

func TestClient_Status(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	expires := base.Add(365 * 24 * time.Hour)

	tests := []struct {
		name         string
		policy       registry.Policy
		age          time.Duration
		wantStale    bool
		wantRunnable bool
		wantWarning  string
	}{
		{name: "just refreshed", age: 0, wantRunnable: true},
		{name: "within the default check interval", age: 24 * time.Hour, wantRunnable: true},
		{name: "past the default check interval", age: 24*time.Hour + time.Second, wantStale: true, wantRunnable: true, wantWarning: "last refreshed 24 hours ago"},
		{name: "days old", age: 5 * 24 * time.Hour, wantStale: true, wantRunnable: true, wantWarning: "last refreshed 5 days ago"},
		{name: "exactly at the default offline grace", age: 30 * 24 * time.Hour, wantStale: true, wantRunnable: true, wantWarning: "30 days ago"},
		{name: "past the default offline grace", age: 30*24*time.Hour + time.Second, wantStale: true, wantWarning: "installed plugins cannot run"},
		{name: "past a short check interval", policy: registry.Policy{CheckIntervalHours: 1}, age: 90 * time.Minute, wantStale: true, wantRunnable: true, wantWarning: "1 hour ago"},
		{name: "within a short offline grace", policy: registry.Policy{OfflineGraceDays: 2}, age: 47 * time.Hour, wantStale: true, wantRunnable: true, wantWarning: "47 hours ago"},
		{name: "past a short offline grace", policy: registry.Policy{OfflineGraceDays: 2}, age: 49 * time.Hour, wantStale: true, wantWarning: "installed plugins cannot run"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			f.publish(f.builder().SetExpires(expires).SetPolicy(test.policy))

			client := f.client()
			client.Now = fixedClock(base)
			_, err := client.Refresh(context.Background())
			require.NoError(t, err)

			client.Now = fixedClock(base.Add(test.age))
			_, status, err := client.Load(t.Context())

			require.NoError(t, err)
			assert.Equal(t, base, status.LastRefresh)
			assert.Equal(t, expires, status.Expires)
			assert.Equal(t, test.wantStale, status.Stale)
			assert.Equal(t, test.wantRunnable, status.Runnable)

			if test.wantWarning == "" {
				assert.Empty(t, status.Warning)
			} else {
				assert.Contains(t, status.Warning, test.wantWarning)
			}
		})
	}
}

func TestClient_EnsureFresh(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	expires := base.Add(365 * 24 * time.Hour)

	setup := func(t *testing.T) (*fixture, *registry.Client) {
		t.Helper()

		f := newFixture(t)
		f.publish(f.builder().SetExpires(expires))

		client := f.client()
		client.Now = fixedClock(base)
		_, err := client.Refresh(context.Background())
		require.NoError(t, err)

		return f, client
	}

	t.Run("refreshes a cache that is older than the check interval", func(t *testing.T) {
		f, client := setup(t)
		f.publish(f.builder().SetExpires(expires).SetVersion(2))
		client.Now = fixedClock(base.Add(25 * time.Hour))

		index, status, err := client.EnsureFresh(context.Background())

		require.NoError(t, err)
		assert.Equal(t, 2, index.Version)
		assert.False(t, status.Stale)
		assert.Equal(t, base.Add(25*time.Hour), status.LastRefresh)
		assert.Equal(t, 2, f.server.Requests("/index.json"))
	})

	t.Run("does not refresh a cache that is within the check interval", func(t *testing.T) {
		f, client := setup(t)
		client.Now = fixedClock(base.Add(23 * time.Hour))

		_, _, err := client.EnsureFresh(context.Background())

		require.NoError(t, err)
		assert.Equal(t, 1, f.server.Requests("/index.json"))
	})

	t.Run("fetches when nothing is cached", func(t *testing.T) {
		f := newFixture(t)

		index, status, err := f.client().EnsureFresh(context.Background())

		require.NoError(t, err)
		assert.Equal(t, 1, index.Version)
		assert.True(t, status.Runnable)
	})

	t.Run("falls back to a usable cache when the registry cannot be reached", func(t *testing.T) {
		f, client := setup(t)
		f.server.Fail("/index.json", http.StatusServiceUnavailable)
		client.Now = fixedClock(base.Add(3 * 24 * time.Hour))

		index, status, err := client.EnsureFresh(context.Background())

		require.NoError(t, err)
		require.NotNil(t, index)
		assert.True(t, status.Stale)
		assert.True(t, status.Runnable)
		assert.Contains(t, status.Warning, "503 Service Unavailable")
		assert.Contains(t, status.Warning, "3 days")
	})

	t.Run("returns the cache beyond the offline grace but not runnable", func(t *testing.T) {
		f, client := setup(t)
		f.server.Fail("/index.json", http.StatusServiceUnavailable)
		client.Now = fixedClock(base.Add(40 * 24 * time.Hour))

		index, status, err := client.EnsureFresh(context.Background())

		require.NoError(t, err)
		require.NotNil(t, index)
		assert.True(t, status.Stale)
		assert.False(t, status.Runnable)
		assert.Contains(t, status.Warning, "installed plugins cannot run")
		assert.Contains(t, status.Warning, "503 Service Unavailable", "the warning keeps both messages")
	})

	t.Run("does not fall back to an expired cache", func(t *testing.T) {
		f, client := setup(t)
		f.server.Fail("/index.json", http.StatusServiceUnavailable)
		client.Now = fixedClock(expires.Add(time.Hour))

		index, _, err := client.EnsureFresh(context.Background())

		assert.Nil(t, index)
		require.ErrorIs(t, err, registry.ErrOffline)
		assert.ErrorIs(t, err, registry.ErrExpired)
	})

	t.Run("fails when nothing is cached and the registry cannot be reached", func(t *testing.T) {
		f := newFixture(t)
		f.server.Fail("/index.json", http.StatusInternalServerError)

		index, _, err := f.client().EnsureFresh(context.Background())

		assert.Nil(t, index)
		require.ErrorIs(t, err, registry.ErrOffline)
		assert.NotErrorIs(t, err, registry.ErrNotCached)
	})

	// The helper "cacheNewerThanRegistry" caches version 3, which is refreshed 25
	// hours after the start, and then makes the registry serve version 2.
	cacheNewerThanRegistry := func(t *testing.T) (*fixture, *registry.Client) {
		t.Helper()

		f, client := setup(t)
		f.publish(f.builder().SetExpires(expires).SetVersion(3))
		client.Now = fixedClock(base.Add(25 * time.Hour))
		_, _, err := client.EnsureFresh(context.Background())
		require.NoError(t, err)

		f.publish(f.builder().SetExpires(expires).SetVersion(2))

		return f, client
	}

	t.Run("falls back to a usable cache when the registry serves an older index", func(t *testing.T) {
		_, client := cacheNewerThanRegistry(t)
		client.Now = fixedClock(base.Add(50 * time.Hour))

		index, status, err := client.EnsureFresh(context.Background())

		require.NoError(t, err)
		assert.Equal(t, 3, index.Version)
		assert.True(t, status.Stale)
		assert.True(t, status.Runnable)
		assert.Equal(t, base.Add(25*time.Hour), status.LastRefresh, "the time of the last refresh does not advance")
		assert.Contains(t, status.Warning, "The registry index is older than one that was already accepted.")
		assert.Contains(t, status.Warning, "version 2 is older than version 3")
		assert.Contains(t, status.Warning, "using the cached index")

		_, err = client.Refresh(context.Background())
		assert.ErrorIs(t, err, registry.ErrRollback, "Refresh still reports the rollback")
	})

	t.Run("stops being runnable when the registry keeps serving an older index", func(t *testing.T) {
		_, client := cacheNewerThanRegistry(t)

		client.Now = fixedClock(base.Add(25*time.Hour + 29*24*time.Hour))
		_, status, err := client.EnsureFresh(context.Background())
		require.NoError(t, err)
		assert.True(t, status.Runnable, "still within the offline grace period")

		client.Now = fixedClock(base.Add(25*time.Hour + 31*24*time.Hour))
		index, status, err := client.EnsureFresh(context.Background())

		require.NoError(t, err)
		assert.Equal(t, 3, index.Version)
		assert.False(t, status.Runnable)
		assert.Contains(t, status.Warning, "installed plugins cannot run")
		assert.Contains(t, status.Warning, "version 2 is older than version 3")
	})

	t.Run("does not fall back to an expired cache when the registry serves an older index", func(t *testing.T) {
		f, client := setup(t)
		f.publish(f.builder().SetExpires(base.Add(10 * 24 * time.Hour)).SetVersion(3))
		_, err := client.Refresh(context.Background())
		require.NoError(t, err)

		f.publish(f.builder().SetExpires(expires).SetVersion(2))
		client.Now = fixedClock(base.Add(11 * 24 * time.Hour))

		index, _, err := client.EnsureFresh(context.Background())

		assert.Nil(t, index)
		require.ErrorIs(t, err, registry.ErrRollback)
		assert.ErrorIs(t, err, registry.ErrExpired)
	})

	t.Run("falls back to a usable cache when the deadline of the context passes", func(t *testing.T) {
		f, client := setup(t)
		f.server.OnRequest(func(string, int) { time.Sleep(200 * time.Millisecond) })
		client.Now = fixedClock(base.Add(3 * 24 * time.Hour))

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		index, status, err := client.EnsureFresh(ctx)

		require.NoError(t, err)
		require.NotNil(t, index)
		assert.True(t, status.Stale)
		assert.Contains(t, status.Warning, "The registry could not be reached")
		assert.Contains(t, status.Warning, "context deadline exceeded")
	})

	t.Run("does not fall back to the cache when the context is cancelled", func(t *testing.T) {
		_, client := setup(t)
		client.Now = fixedClock(base.Add(3 * 24 * time.Hour))

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		index, _, err := client.EnsureFresh(ctx)

		assert.Nil(t, index)
		require.ErrorIs(t, err, context.Canceled)
		assert.NotErrorIs(t, err, registry.ErrOffline)
	})
}

func TestNewClient(t *testing.T) {
	client := registry.NewClient(registry.Registry{Name: "auth0", URL: "https://example.com/index.json"}, "/some/dir")

	assert.Equal(t, "/some/dir", client.Dir)
	assert.Equal(t, registry.TrustedKeys(), client.Keys)
	assert.Nil(t, client.HTTP)
	assert.Nil(t, client.Now)
}

// proxyRequests counts the requests that reached the proxy of
// TestClient_DefaultHTTPClientHonorsProxy.
var proxyRequests atomic.Int32

// testProxy starts, once for the test process, a proxy that answers every request
// with an error. It is not closed, because the standard library reads the proxy
// settings of the environment only once per process, so every run of the test
// has to use the same proxy.
var testProxy = sync.OnceValue(func() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		proxyRequests.Add(1)
		http.Error(w, "no", http.StatusBadGateway)
	}))
})

func TestClient_DefaultHTTPClientHonorsProxy(t *testing.T) {
	proxy := testProxy()
	before := proxyRequests.Load()

	t.Setenv("HTTPS_PROXY", proxy.URL)
	t.Setenv("https_proxy", proxy.URL)
	t.Setenv("NO_PROXY", "")
	t.Setenv("no_proxy", "")

	keyID, keys, _ := registrytest.NewKeyPair(t)
	_ = keyID

	client := registry.NewClient(registry.Registry{Name: "auth0", URL: "https://registry.example.com/index.json"}, t.TempDir())
	client.Keys = keys

	_, err := client.Refresh(context.Background())

	require.ErrorIs(t, err, registry.ErrOffline)
	assert.Greater(t, proxyRequests.Load(), before, "the request went through the proxy")
}

// The duplicate keys of an index are tested with ParseIndex, and those of the
// state file with TestCache_DamagedState. The signature file is only read by
// Refresh, so it is tested here.
func TestClient_Refresh_DuplicateKeysInTheSignature(t *testing.T) {
	f := newFixture(t)
	f.server.Set("/index.json.sig", []byte(strings.Replace(
		string(registrytest.Sign(f.builder().Build(), f.keyID, f.private)), `{`, `{"keyId":"other",`, 1)))

	_, err := f.client().Refresh(context.Background())

	require.ErrorIs(t, err, registry.ErrSignatureInvalid)
	assert.Contains(t, err.Error(), `duplicate key "keyId"`)
}

// A failed signature check is retried once, because the registry may have
// published a new index between the two requests. Whatever goes wrong with the
// second attempt must not hide the first failure: Refresh returns it, and
// EnsureFresh does not take the registry for unreachable, or serving old data,
// and fall back to the cache.
func TestClient_SignatureFailureIsNotHiddenByTheRetry(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string

		// What the registry does once the first attempt has downloaded the index,
		// which does not match its signature.
		second func(f *fixture)

		// The failure of the second attempt, which the error still mentions, and
		// the error that it must not be mistaken for.
		wantDetail string
		notError   error
	}{
		{
			name:       "the registry cannot be reached the second time",
			second:     func(f *fixture) { f.server.Fail("/index.json", http.StatusInternalServerError) },
			wantDetail: "500 Internal Server Error",
			notError:   registry.ErrOffline,
		},
		{
			name:       "the registry serves an older index the second time",
			second:     func(f *fixture) { f.publish(f.builder().SetVersion(3)) },
			wantDetail: "version 3 is older than version 5",
			notError:   registry.ErrRollback,
		},
		{
			name:       "the registry serves an index that is not valid the second time",
			second:     func(f *fixture) { f.publishRaw([]byte("not json")) },
			wantDetail: "retry also failed",
			notError:   registry.ErrInvalidIndex,
		},
		{
			name:       "the registry serves an expired index the second time",
			second:     func(f *fixture) { f.publish(f.builder().SetVersion(6).SetExpires(base.Add(-time.Hour))) },
			wantDetail: "expired on",
			notError:   registry.ErrExpired,
		},
	}

	// The helper "setup" caches version 5, and makes the next refresh fail the
	// signature check the first time.
	setup := func(t *testing.T, second func(f *fixture)) *registry.Client {
		t.Helper()

		f := newFixture(t)
		f.publish(f.builder().SetVersion(5))

		client := f.client()
		client.Now = fixedClock(base)
		_, err := client.Refresh(context.Background())
		require.NoError(t, err)

		f.server.Set("/index.json", []byte(`{"formatVersion":1}`))

		firstSignatureRequest := f.server.Requests("/index.json.sig") + 1
		f.server.OnRequest(func(path string, count int) {
			if path == "/index.json.sig" && count == firstSignatureRequest {
				second(f)
			}
		})

		return client
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Run("Refresh returns the signature failure", func(t *testing.T) {
				client := setup(t, test.second)

				_, err := client.Refresh(context.Background())

				require.ErrorIs(t, err, registry.ErrSignatureInvalid)
				assert.NotErrorIs(t, err, test.notError)
				assert.Contains(t, err.Error(), test.wantDetail, "the failure of the second attempt is still shown")
			})

			t.Run("EnsureFresh does not fall back to the cache", func(t *testing.T) {
				client := setup(t, test.second)
				client.Now = fixedClock(base.Add(3 * 24 * time.Hour))

				index, _, err := client.EnsureFresh(context.Background())

				assert.Nil(t, index)
				require.ErrorIs(t, err, registry.ErrSignatureInvalid)
				assert.NotErrorIs(t, err, test.notError)
			})
		})
	}
}

func TestClient_Refresh_KeepsTheQueryOfTheRegistryURL(t *testing.T) {
	f := newFixture(t)

	var queries []string

	client := f.client()
	client.Registry.URL += "?ref=main"
	inner := client.HTTP.Transport
	client.HTTP.Transport = roundTripper(func(r *http.Request) (*http.Response, error) {
		queries = append(queries, r.URL.Path+"?"+r.URL.RawQuery)

		return inner.RoundTrip(r)
	})

	_, err := client.Refresh(context.Background())

	require.NoError(t, err)
	assert.Equal(t, []string{"/index.json?ref=main", "/index.json.sig?ref=main"}, queries)
}

// roundTripper is an http.RoundTripper made of a function.
type roundTripper func(r *http.Request) (*http.Response, error)

// RoundTrip calls the function.
func (f roundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestClient_EnsureFresh_JudgesTheCacheAfterTheRefreshFailed(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	expires := base.Add(48 * time.Hour)

	f := newFixture(t)
	f.publish(f.builder().SetExpires(expires))

	client := f.client()
	client.Now = fixedClock(base)
	_, err := client.Refresh(context.Background())
	require.NoError(t, err)

	// The index expires in a minute, and the registry answers after two.
	var clock atomic.Int64
	clock.Store(expires.Add(-time.Minute).UnixNano())

	client.Now = func() time.Time { return time.Unix(0, clock.Load()).UTC() }
	f.server.OnRequest(func(string, int) {
		clock.Add(int64(2 * time.Minute))
	})
	f.server.Fail("/index.json", http.StatusServiceUnavailable)

	index, status, err := client.EnsureFresh(context.Background())

	assert.Nil(t, index, "an index that expired while the refresh was running is not returned")
	require.ErrorIs(t, err, registry.ErrExpired)
	assert.NotErrorIs(t, err, registry.ErrOffline)
	assert.Contains(t, err.Error(), "503 Service Unavailable")
	assert.False(t, status.Runnable)
}

func TestClient_Load_RecoversFromARefreshInProgress(t *testing.T) {
	tests := []struct {
		name string

		// The file whose rename the refresh pauses before.
		pauseBefore string
	}{
		{name: "after the state was replaced", pauseBefore: "index.json.sig"},
		{name: "after the signature was replaced", pauseBefore: "index.json"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)

			f.publish(f.builder().SetVersion(1))
			_, err := f.client().Refresh(context.Background())
			require.NoError(t, err)

			f.publish(f.builder().SetVersion(2))

			type loaded struct {
				index *registry.Index
				err   error
			}

			results := make(chan loaded, 1)
			early := false

			restore := registry.SetRenameFile(func(oldPath, newPath string) error {
				if filepath.Base(newPath) == test.pauseBefore {
					// The cache on disk does not verify now, so this Load has to wait for
					// the refresh instead of reporting that.
					go func() {
						index, _, err := f.client().Load(t.Context())
						results <- loaded{index, err}
					}()

					select {
					case result := <-results:
						results <- result
						early = true
					case <-time.After(300 * time.Millisecond):
					}
				}

				return os.Rename(oldPath, newPath)
			})

			_, err = f.client().Refresh(context.Background())
			restore()
			require.NoError(t, err)

			assert.False(t, early, "Load returned while the cache did not verify")

			select {
			case result := <-results:
				require.NoError(t, result.err)
				assert.Equal(t, 2, result.index.Version)
			case <-time.After(10 * time.Second):
				t.Fatal("Load never returned")
			}
		})
	}

	t.Run("still fails for a cache that is really tampered with", func(t *testing.T) {
		f := newFixture(t)
		client := f.client()
		_, err := client.Refresh(context.Background())
		require.NoError(t, err)

		index := f.readCache("index.json")
		index[len(index)/2] ^= 0x01
		f.writeCache("index.json", index)

		loadedIndex, _, err := client.Load(t.Context())

		assert.Nil(t, loadedIndex)
		assert.ErrorIs(t, err, registry.ErrSignatureInvalid)
	})
}
