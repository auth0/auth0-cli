package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/display"
)

// apiWriteClient captures the method and URI runJSONWrite sends, so a --data
// test can assert the command built the right resource path.
type apiWriteClient struct {
	rawHTTPClientStub
	gotMethod string
	gotURI    string
}

// Request records the method and URI, then returns nil so runJSONWrite can
// unmarshal the sent payload back for rendering.
func (c *apiWriteClient) Request(_ context.Context, method string, uri string, _ interface{}, _ ...management.RequestOption) error {
	c.gotMethod = method
	c.gotURI = uri
	return nil
}

func newAPIWriteCLI(client *apiWriteClient, stdout *bytes.Buffer) *cli {
	return &cli{
		api: &auth0.API{HTTPClient: client},
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  stdout,
		},
	}
}

func TestApisListCmdInvalidQuery(t *testing.T) {
	cli := &cli{
		renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: io.Discard},
		api:      &auth0.API{},
	}

	cmd := listApisCmd(cli)
	cmd.SetArgs([]string{"--query", "not-valid-json"})

	assert.ErrorContains(t, cmd.Execute(), "invalid --query value")
}

func TestApisCreateCmdData(t *testing.T) {
	t.Run("sends the validated payload to POST /resource-servers", func(t *testing.T) {
		client := &apiWriteClient{}
		cmd := createAPICmd(newAPIWriteCLI(client, &bytes.Buffer{}))
		// The leaf command runs without the root PreRun, so clear the required-flag
		// annotations here (stdin is non-TTY under test) to reach RunE with only --data.
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"name":"json-api","identifier":"https://json-api.example.com"}`})

		require.NoError(t, cmd.Execute())
		assert.Equal(t, http.MethodPost, client.gotMethod)
		assert.Contains(t, client.gotURI, "/resource-servers")
	})

	t.Run("surfaces a schema validation failure before calling the API", func(t *testing.T) {
		client := &apiWriteClient{}
		cmd := createAPICmd(newAPIWriteCLI(client, &bytes.Buffer{}))
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"name":123,"identifier":"https://x.example.com"}`})

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schema validation")
		assert.Empty(t, client.gotMethod, "must not reach the API when validation fails")
	})
}

func TestApisUpdateCmdData(t *testing.T) {
	client := &apiWriteClient{}
	cmd := updateAPICmd(newAPIWriteCLI(client, &bytes.Buffer{}))
	prepareInteractivity(cmd)
	cmd.SetArgs([]string{"rs_123", "--data", `{"name":"renamed-api"}`})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, http.MethodPatch, client.gotMethod)
	assert.Contains(t, client.gotURI, "/resource-servers/rs_123")
}
