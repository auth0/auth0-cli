//go:build !plugintest

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

func TestEmbeddedRoot_FailsClosedUntilTheProductionRootExists(t *testing.T) {
	f := newFixture(t)

	// A root that a test hands over through the environment is not read by a
	// regular build.
	path := filepath.Join(t.TempDir(), "root.json")
	require.NoError(t, os.WriteFile(path, f.repo.Root(), 0o600))
	t.Setenv("AUTH0_CLI_PLUGIN_TEST_ROOT", path)

	client := registry.NewClient(f.server.Registry(), f.dir)
	client.HTTP = f.server.Client()

	_, err := client.Refresh(context.Background())
	require.ErrorIs(t, err, registry.ErrNoTrustedKeys)

	_, _, err = client.Load(t.Context())
	assert.ErrorIs(t, err, registry.ErrNoTrustedKeys)

	_, _, err = client.EnsureFresh(context.Background())
	assert.ErrorIs(t, err, registry.ErrNoTrustedKeys)

	assert.Zero(t, f.server.Requests("/timestamp.json"), "nothing is fetched")
	assert.NoDirExists(t, f.dir, "not even the lock is created")
}

func TestLoopbackHTTP_IsRejectedWithoutThePlugintestTag(t *testing.T) {
	var requested bool

	plain := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requested = true }))
	t.Cleanup(plain.Close)

	for _, host := range []string{plain.URL, "http://localhost:1", "http://[::1]:1"} {
		client := registry.NewClient(registry.Registry{Name: "auth0", URL: host + "/registry/"}, t.TempDir())
		client.Root = registrytest.NewRepo(t).Root()

		_, err := client.Refresh(context.Background())
		require.ErrorIs(t, err, registry.ErrInsecureURL, host)
	}

	assert.False(t, requested)

	_, err := registry.ResolveRegistries([]config.RegistryConfig{{Name: "auth0", URL: plain.URL + "/registry/"}})
	assert.ErrorIs(t, err, registry.ErrInsecureURL)
}
