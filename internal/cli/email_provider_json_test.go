package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/display"
)

// emailProviderWriteClient captures the method and URI runJSONWrite sends, so a
// --data test can assert the command built the right resource path.
type emailProviderWriteClient struct {
	rawHTTPClientStub
	gotMethod string
	gotURI    string
}

// Request records the method and URI, writing a minimal email-provider response
// into the payload pointer so runJSONWrite can decode it for rendering.
func (c *emailProviderWriteClient) Request(_ context.Context, method, uri string, payload interface{}, _ ...management.RequestOption) error {
	c.gotMethod = method
	c.gotURI = uri
	if raw, ok := payload.(*json.RawMessage); ok {
		*raw = json.RawMessage(`{"name":"sendgrid","enabled":true}`)
	}
	return nil
}

func newEmailProviderWriteCLI(client *emailProviderWriteClient, stdout *bytes.Buffer) *cli {
	return &cli{
		api: &auth0.API{HTTPClient: client},
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  stdout,
		},
	}
}

func TestEmailProviderCreateCmdData(t *testing.T) {
	t.Run("sends the validated payload to POST /emails/provider", func(t *testing.T) {
		client := &emailProviderWriteClient{}
		cmd := createEmailProviderCmd(newEmailProviderWriteCLI(client, &bytes.Buffer{}))
		// The leaf command runs without the root PreRun, so clear the required-flag
		// annotations here (stdin is non-TTY under test) to reach RunE with only --data.
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"name":"sendgrid","credentials":{"api_key":"key"},"enabled":true}`})

		require.NoError(t, cmd.Execute())
		assert.Equal(t, http.MethodPost, client.gotMethod)
		assert.Contains(t, client.gotURI, "/emails/provider")
	})

	t.Run("surfaces a schema validation failure before calling the API", func(t *testing.T) {
		client := &emailProviderWriteClient{}
		cmd := createEmailProviderCmd(newEmailProviderWriteCLI(client, &bytes.Buffer{}))
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"name":123}`})

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schema validation")
		assert.Empty(t, client.gotMethod, "must not reach the API when validation fails")
	})
}

func TestEmailProviderUpdateCmdData(t *testing.T) {
	client := &emailProviderWriteClient{}
	cmd := updateEmailProviderCmd(newEmailProviderWriteCLI(client, &bytes.Buffer{}))
	prepareInteractivity(cmd)
	cmd.SetArgs([]string{"--data", `{"credentials":{"api_key":"NewKey"}}`})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, http.MethodPatch, client.gotMethod)
	assert.Contains(t, client.gotURI, "/emails/provider")
}
