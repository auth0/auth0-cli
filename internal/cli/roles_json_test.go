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

// roleWriteClient captures the method and URI runJSONWrite sends, so a --data
// test can assert the command built the right resource path. Request returns nil
// so runJSONWrite can unmarshal the payload back for rendering.
type roleWriteClient struct {
	rawHTTPClientStub
	gotMethod string
	gotURI    string
}

func (c *roleWriteClient) Request(
	_ context.Context,
	method string,
	uri string,
	_ interface{},
	_ ...management.RequestOption,
) error {
	c.gotMethod = method
	c.gotURI = uri
	return nil
}

func newRoleWriteCLI(client *roleWriteClient, stdout *bytes.Buffer) *cli {
	return &cli{
		api: &auth0.API{HTTPClient: client},
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  stdout,
		},
	}
}

func TestRolesListCmdInvalidQuery(t *testing.T) {
	cli := &cli{
		renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: io.Discard},
		api:      &auth0.API{},
	}

	cmd := listRolesCmd(cli)
	cmd.SetArgs([]string{"--query", "not-valid-json"})

	assert.ErrorContains(t, cmd.Execute(), "invalid --query value")
}

func TestRolesCreateCmdData(t *testing.T) {
	t.Run("sends the validated payload to POST /roles", func(t *testing.T) {
		client := &roleWriteClient{}
		cmd := createRoleCmd(newRoleWriteCLI(client, &bytes.Buffer{}))
		// Clear the required-flag annotations (as the root PreRun does for
		// non-TTY input) so --data alone satisfies the command.
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"name":"json-role","description":"created via data"}`})

		require.NoError(t, cmd.Execute())
		assert.Equal(t, http.MethodPost, client.gotMethod)
		assert.Contains(t, client.gotURI, "/roles")
	})

	t.Run("surfaces a schema validation failure before calling the API", func(t *testing.T) {
		client := &roleWriteClient{}
		cmd := createRoleCmd(newRoleWriteCLI(client, &bytes.Buffer{}))
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"name":123}`})

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schema validation")
		assert.Empty(t, client.gotMethod, "must not reach the API when validation fails")
	})
}

func TestRolesUpdateCmdData(t *testing.T) {
	client := &roleWriteClient{}
	cmd := updateRoleCmd(newRoleWriteCLI(client, &bytes.Buffer{}))
	prepareInteractivity(cmd)
	cmd.SetArgs([]string{"rol_123", "--data", `{"description":"updated via data"}`})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, http.MethodPatch, client.gotMethod)
	assert.Contains(t, client.gotURI, "/roles/rol_123")
}
