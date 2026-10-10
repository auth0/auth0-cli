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

// phoneProviderWriteClient captures the method and URI runJSONWrite sends, so a
// --data test can assert the command built the right resource path.
type phoneProviderWriteClient struct {
	rawHTTPClientStub
	gotMethod string
	gotURI    string
}

// Request records the method and URI, writing a minimal phone-provider response
// into the payload pointer so runJSONWrite can decode it for rendering.
func (c *phoneProviderWriteClient) Request(_ context.Context, method, uri string, payload interface{}, _ ...management.RequestOption) error {
	c.gotMethod = method
	c.gotURI = uri
	if raw, ok := payload.(*json.RawMessage); ok {
		*raw = json.RawMessage(`{"id":"pp_123","name":"twilio","disabled":false}`)
	}
	return nil
}

func newPhoneProviderWriteCLI(client *phoneProviderWriteClient, stdout *bytes.Buffer) *cli {
	return &cli{
		api: &auth0.API{HTTPClient: client},
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  stdout,
		},
	}
}

func TestPhoneProviderListCmdInvalidQuery(t *testing.T) {
	c := &cli{
		renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: io.Discard},
		api:      &auth0.API{},
	}

	cmd := listBrandingPhoneProviderCmd(c)
	cmd.SetArgs([]string{"--query", "not-valid-json"})

	assert.ErrorContains(t, cmd.Execute(), "invalid --query value")
}

func TestPhoneProviderCreateCmdData(t *testing.T) {
	t.Run("sends the validated payload to POST /branding/phone/providers", func(t *testing.T) {
		client := &phoneProviderWriteClient{}
		cmd := createBrandingPhoneProviderCmd(newPhoneProviderWriteCLI(client, &bytes.Buffer{}))
		// The leaf command runs without the root PreRun, so clear the required-flag
		// annotations here (stdin is non-TTY under test) to reach RunE with only --data.
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"name":"twilio","credentials":{"auth_token":"tok"},"configuration":{"delivery_methods":["text"]}}`})

		require.NoError(t, cmd.Execute())
		assert.Equal(t, http.MethodPost, client.gotMethod)
		assert.Contains(t, client.gotURI, "/branding/phone/providers")
	})

	t.Run("surfaces a schema validation failure before calling the API", func(t *testing.T) {
		client := &phoneProviderWriteClient{}
		cmd := createBrandingPhoneProviderCmd(newPhoneProviderWriteCLI(client, &bytes.Buffer{}))
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"name":123}`})

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schema validation")
		assert.Empty(t, client.gotMethod, "must not reach the API when validation fails")
	})
}

func TestPhoneProviderUpdateCmdData(t *testing.T) {
	client := &phoneProviderWriteClient{}
	cmd := updateBrandingPhoneProviderCmd(newPhoneProviderWriteCLI(client, &bytes.Buffer{}))
	prepareInteractivity(cmd)
	cmd.SetArgs([]string{"pp_123", "--data", `{"credentials":{"auth_token":"newTok"}}`})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, http.MethodPatch, client.gotMethod)
	assert.Contains(t, client.gotURI, "/branding/phone/providers/pp_123")
}
