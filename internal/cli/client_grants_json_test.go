package cli

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/display"
)

func TestClientGrantsListCmdInvalidQuery(t *testing.T) {
	cli := &cli{
		renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: io.Discard},
		api:      &auth0.API{},
	}

	cmd := listClientGrantsCmd(cli)
	cmd.SetArgs([]string{"--query", "not-valid-json"})

	assert.ErrorContains(t, cmd.Execute(), "invalid --query value")
}

func TestClientGrantsCreateCmdData(t *testing.T) {
	t.Run("sends the validated payload to POST /client-grants", func(t *testing.T) {
		client := &brandingWriteClient{}
		cmd := createClientGrantCmd(newBrandingWriteCLI(client, &bytes.Buffer{}))
		// The leaf command runs without the root PreRun, so clear the required-flag
		// annotations here (stdin is non-TTY under test) to reach RunE with only --data.
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"client_id":"abc","audience":"https://example.com/api","scope":["read:users"]}`})

		require.NoError(t, cmd.Execute())
		assert.Equal(t, http.MethodPost, client.gotMethod)
		assert.Contains(t, client.gotURI, "/client-grants")
	})

	t.Run("surfaces a schema validation failure before calling the API", func(t *testing.T) {
		client := &brandingWriteClient{}
		cmd := createClientGrantCmd(newBrandingWriteCLI(client, &bytes.Buffer{}))
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"audience":123}`})

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schema validation")
		assert.Empty(t, client.gotMethod, "must not reach the API when validation fails")
	})
}

func TestClientGrantsUpdateCmdData(t *testing.T) {
	client := &brandingWriteClient{}
	cmd := updateClientGrantCmd(newBrandingWriteCLI(client, &bytes.Buffer{}))
	prepareInteractivity(cmd)
	cmd.SetArgs([]string{"cgr_123", "--data", `{"scope":["read:users"]}`})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, http.MethodPatch, client.gotMethod)
	assert.Contains(t, client.gotURI, "/client-grants/cgr_123")
}
