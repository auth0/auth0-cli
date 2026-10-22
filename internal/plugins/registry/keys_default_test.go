//go:build !plugintest

package registry_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/config"
	"github.com/auth0/auth0-cli/internal/plugins/registry"
)

func TestTrustedKeys_AreEmptyUntilTheProductionKeyExists(t *testing.T) {
	assert.Empty(t, registry.TrustedKeys(), "the client fails closed with ErrNoTrustedKeys until a key is embedded")
}

func TestLoopbackHTTP_IsRejectedWithoutThePlugintestTag(t *testing.T) {
	var requested bool

	plain := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requested = true }))
	t.Cleanup(plain.Close)

	for _, host := range []string{plain.URL, "http://localhost:1", "http://[::1]:1"} {
		client := registry.NewClient(registry.Registry{Name: "auth0", URL: host + "/index.json"}, t.TempDir())
		client.Keys = registry.KeySet{"k": make([]byte, 32)}

		_, err := client.Refresh(context.Background())
		require.ErrorIs(t, err, registry.ErrInsecureURL, host)
	}

	assert.False(t, requested)

	_, err := registry.ResolveRegistries([]config.RegistryConfig{{Name: "auth0", URL: plain.URL + "/index.json"}})
	assert.ErrorIs(t, err, registry.ErrInsecureURL)
}
