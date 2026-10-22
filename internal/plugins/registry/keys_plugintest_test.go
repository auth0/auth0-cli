//go:build plugintest

package registry_test

import (
	"context"
	"crypto/ed25519"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/config"
	"github.com/auth0/auth0-cli/internal/plugins/registry"
	"github.com/auth0/auth0-cli/internal/plugins/registry/registrytest"
)

func TestTrustedKeys_IncludeTheTestKey(t *testing.T) {
	keys := registry.TrustedKeys()

	require.Contains(t, keys, registrytest.TestKeyID)
	assert.Len(t, keys, 1)

	public, ok := registrytest.TestPrivateKey().Public().(ed25519.PublicKey)
	require.True(t, ok)
	assert.Equal(t, public, keys[registrytest.TestKeyID], "the embedded key is the public half of the test fixture")
}

func TestTestKey_VerifiesAnIndexOverTLS(t *testing.T) {
	server := registrytest.NewServer(t)
	server.Publish(
		registrytest.NewIndexBuilder().AddPlugin(registrytest.NewPlugin(testPluginID, registrytest.NewVersion("1.0.0"))),
		registrytest.TestKeyID, registrytest.TestPrivateKey(),
	)

	client := registry.NewClient(server.Registry(), t.TempDir())
	client.HTTP = server.Client()

	index, err := client.Refresh(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, index.Version)
}

func TestLoopbackHTTP_IsAcceptedWithThePlugintestTag(t *testing.T) {
	builder := registrytest.NewIndexBuilder().AddRefPlugin(registrytest.NewPlugin(testPluginID, registrytest.NewVersion("1.0.0")))
	index := builder.Build()
	files := builder.Files()

	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json":
			_, _ = w.Write(index)
		case "/index.json.sig":
			_, _ = w.Write(registrytest.Sign(index, registrytest.TestKeyID, registrytest.TestPrivateKey()))
		case "/plugins/checkmate.json":
			_, _ = w.Write(files["plugins/checkmate.json"])
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(plain.Close)

	client := registry.NewClient(registry.Registry{Name: "auth0", URL: plain.URL + "/index.json"}, t.TempDir())

	loaded, err := client.Refresh(context.Background())
	require.NoError(t, err, "the registry is fetched over loopback http")

	plugin, found := loaded.FindPlugin(testPluginID)
	require.True(t, found)

	versions, err := client.Versions(context.Background(), plugin)
	require.NoError(t, err, "the plugin file is fetched over loopback http")
	assert.Len(t, versions, 1)

	registries, err := registry.ResolveRegistries([]config.RegistryConfig{{Name: "auth0", URL: plain.URL + "/index.json"}})
	require.NoError(t, err)
	assert.Equal(t, plain.URL+"/index.json", registries[0].URL)
}

func TestLoopbackHTTP_IsStillLimitedToLoopback(t *testing.T) {
	_, err := registry.ResolveRegistries([]config.RegistryConfig{{Name: "auth0", URL: "http://example.com/index.json"}})
	assert.ErrorIs(t, err, registry.ErrInsecureURL)

	for _, url := range []string{"http://localhost:8080/index.json", "http://[::1]:8080/index.json", "http://127.0.0.1/index.json"} {
		_, err := registry.ResolveRegistries([]config.RegistryConfig{{Name: "auth0", URL: url}})
		assert.NoError(t, err, url)
	}
}
