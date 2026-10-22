//go:build plugintest

package registry_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/config"
	"github.com/auth0/auth0-cli/internal/plugins/registry"
	"github.com/auth0/auth0-cli/internal/plugins/registry/registrytest"
)

const testRootEnv = "AUTH0_CLI_PLUGIN_TEST_ROOT"

// writeRoot writes a root to a file in a new temporary directory, and returns
// the path.
func writeRoot(t *testing.T, root []byte) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "root.json")
	require.NoError(t, os.WriteFile(path, root, 0o600))

	return path
}

func TestEmbeddedRoot_IsATestRootNobodyCanPublishFor(t *testing.T) {
	f := newFixture(t)

	client := registry.NewClient(f.server.Registry(), f.dir)
	client.HTTP = f.server.Client()

	// The embedded root is a valid root, signed by its own keys, so the failure is
	// that the registry of the fixture is not signed by the keys that it lists.
	_, err := client.Refresh(context.Background())

	require.ErrorIs(t, err, registry.ErrSignatureInvalid)
	assert.NotErrorIs(t, err, registry.ErrNoTrustedKeys)
	assert.Empty(t, f.cache())
}

func TestEmbeddedRoot_HasNoPrivateKey(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "test-root.json"))
	require.NoError(t, err)

	assert.NotContains(t, string(data), "private")
}

func TestEmbeddedRoot_CanBeReplacedFromTheEnvironment(t *testing.T) {
	f := newFixture(t)
	t.Setenv(testRootEnv, writeRoot(t, f.repo.Root()))

	client := registry.NewClient(f.server.Registry(), f.dir)
	client.HTTP = f.server.Client()

	index, err := client.Refresh(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, index.Version)

	_, _, err = client.Load(t.Context())
	assert.NoError(t, err)
}

func TestEmbeddedRoot_FailsClosedWhenTheEnvironmentRootCannotBeRead(t *testing.T) {
	tests := map[string]string{
		"a file that does not exist": filepath.Join(t.TempDir(), "missing.json"),
		"a file that is not a root":  writeRoot(t, []byte("not json")),
	}

	for name, path := range tests {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			t.Setenv(testRootEnv, path)

			client := registry.NewClient(f.server.Registry(), f.dir)
			client.HTTP = f.server.Client()

			_, err := client.Refresh(context.Background())

			require.ErrorIs(t, err, registry.ErrNoTrustedKeys, "the embedded root is not used instead")
			assert.Zero(t, f.server.Requests("/timestamp.json"), "nothing is fetched")
		})
	}
}

func TestEmbeddedRoot_RootOfTheClientComesFirst(t *testing.T) {
	f := newFixture(t)
	t.Setenv(testRootEnv, writeRoot(t, registrytest.NewRepo(t).Root()))

	_, err := f.client().Refresh(context.Background())
	assert.NoError(t, err)
}

func TestLoopbackHTTP_IsAcceptedWithThePlugintestTag(t *testing.T) {
	f := newFixture(t)
	builder := registrytest.NewIndexBuilder().AddRefPlugin(registrytest.NewPlugin(testPluginID, registrytest.NewVersion("1.0.0")))
	f.publish(builder)

	files := f.repo.Files()
	for name, data := range builder.Files() {
		files[name] = data
	}

	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, found := files[r.URL.Path[1:]]
		if !found {
			http.NotFound(w, r)

			return
		}

		_, _ = w.Write(data)
	}))
	t.Cleanup(plain.Close)

	client := f.client()
	client.Registry = registry.Registry{Name: "auth0", URL: plain.URL + "/"}
	client.HTTP = nil

	loaded, err := client.Refresh(context.Background())
	require.NoError(t, err, "the registry is fetched over loopback http")

	plugin, found := loaded.FindPlugin(testPluginID)
	require.True(t, found)

	versions, err := client.Versions(context.Background(), plugin)
	require.NoError(t, err, "the plugin file is fetched over loopback http")
	assert.Len(t, versions, 1)

	resolved, err := registry.ResolveRegistries([]config.RegistryConfig{{Name: "auth0", URL: plain.URL + "/"}})
	require.NoError(t, err)
	assert.Equal(t, plain.URL+"/", resolved.URL)
}

func TestLoopbackHTTP_IsStillLimitedToLoopback(t *testing.T) {
	_, err := registry.ResolveRegistries([]config.RegistryConfig{{Name: "auth0", URL: "http://example.com/registry/"}})
	assert.ErrorIs(t, err, registry.ErrInsecureURL)

	for _, url := range []string{"http://localhost:8080/registry/", "http://[::1]:8080/registry/", "http://127.0.0.1/registry/"} {
		_, err := registry.ResolveRegistries([]config.RegistryConfig{{Name: "auth0", URL: url}})
		assert.NoError(t, err, url)
	}
}
