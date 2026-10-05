package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/config"
	"github.com/auth0/auth0-cli/internal/display"
)

// appWriteClient captures the method and URI runJSONWrite sends, so a --data
// test can assert the command built the right resource path.
type appWriteClient struct {
	rawHTTPClientStub
	gotMethod string
	gotURI    string
}

// Request records the method and URI, then returns nil so runJSONWrite can
// unmarshal the sent payload back for rendering.
func (c *appWriteClient) Request(_ context.Context, method string, uri string, _ interface{}, _ ...management.RequestOption) error {
	c.gotMethod = method
	c.gotURI = uri
	return nil
}

func newAppWriteCLI(client *appWriteClient, stdout *bytes.Buffer) *cli {
	return &cli{
		api: &auth0.API{HTTPClient: client},
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  stdout,
		},
	}
}

// newAppWriteCLIWithTenant seeds a single tenant in a throwaway config under a
// temp HOME so createAppFromJSON's SetDefaultAppIDForTenant side effect resolves
// and saves without touching the real user config.
func newAppWriteCLIWithTenant(t *testing.T, client *appWriteClient) *cli {
	t.Helper()

	const tenantDomain = "test.auth0.com"
	home := t.TempDir()
	t.Setenv("HOME", home)

	configDir := filepath.Join(home, ".config", "auth0")
	require.NoError(t, os.MkdirAll(configDir, 0700))

	data, err := json.Marshal(config.Config{
		DefaultTenant: tenantDomain,
		Tenants: map[string]config.Tenant{
			tenantDomain: {Domain: tenantDomain, Name: "test"},
		},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.json"), data, 0600))

	c := newAppWriteCLI(client, &bytes.Buffer{})
	c.tenant = tenantDomain

	return c
}

func TestAppsListCmdInvalidQuery(t *testing.T) {
	cli := &cli{
		renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: io.Discard},
		api:      &auth0.API{},
	}

	cmd := listAppsCmd(cli)
	cmd.SetArgs([]string{"--query", "not-valid-json"})

	assert.ErrorContains(t, cmd.Execute(), "invalid --query value")
}

func TestAppsCreateCmdData(t *testing.T) {
	t.Run("sends the validated payload to POST /clients", func(t *testing.T) {
		client := &appWriteClient{}
		cmd := createAppCmd(newAppWriteCLIWithTenant(t, client))
		// The leaf command runs without the root PreRun, so clear the required-flag
		// annotations here (stdin is non-TTY under test) to reach RunE with only --data.
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"name":"json-app"}`})

		require.NoError(t, cmd.Execute())
		assert.Equal(t, http.MethodPost, client.gotMethod)
		assert.Contains(t, client.gotURI, "/clients")
	})

	t.Run("surfaces a schema validation failure before calling the API", func(t *testing.T) {
		client := &appWriteClient{}
		cmd := createAppCmd(newAppWriteCLI(client, &bytes.Buffer{}))
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"name":123}`})

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schema validation")
		assert.Empty(t, client.gotMethod, "must not reach the API when validation fails")
	})
}

func TestAppsUpdateCmdData(t *testing.T) {
	client := &appWriteClient{}
	cmd := updateAppCmd(newAppWriteCLI(client, &bytes.Buffer{}))
	prepareInteractivity(cmd)
	cmd.SetArgs([]string{"cli_123", "--data", `{"name":"renamed-app"}`})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, http.MethodPatch, client.gotMethod)
	assert.Contains(t, client.gotURI, "/clients/cli_123")
}
