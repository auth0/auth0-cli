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

// customDomainWriteClient captures the method and URI runJSONWrite sends, so a
// --data test can assert the command built the right resource path.
type customDomainWriteClient struct {
	rawHTTPClientStub
	gotMethod string
	gotURI    string
}

// Request records the method and URI, then writes a minimal custom-domain
// response back into the payload pointer — mirroring how the real Management
// client returns the created/updated resource — so runJSONWrite can decode it
// for rendering. The verification object must be present because the renderer
// walks its methods, exactly as the live API always returns it.
func (c *customDomainWriteClient) Request(_ context.Context, method string, uri string, payload interface{}, _ ...management.RequestOption) error {
	c.gotMethod = method
	c.gotURI = uri
	if raw, ok := payload.(*json.RawMessage); ok {
		*raw = json.RawMessage(`{"custom_domain_id":"cd_123","domain":"login.example.com","verification":{"methods":[]}}`)
	}
	return nil
}

func newCustomDomainWriteCLI(client *customDomainWriteClient, stdout *bytes.Buffer) *cli {
	return &cli{
		api: &auth0.API{HTTPClient: client},
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  stdout,
		},
	}
}

func TestCustomDomainsListCmdInvalidQuery(t *testing.T) {
	cli := &cli{
		renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: io.Discard},
		api:      &auth0.API{},
	}

	cmd := listCustomDomainsCmd(cli)
	cmd.SetArgs([]string{"--query", "not-valid-json"})

	assert.ErrorContains(t, cmd.Execute(), "invalid --query value")
}

func TestCustomDomainsCreateCmdData(t *testing.T) {
	t.Run("sends the validated payload to POST /custom-domains", func(t *testing.T) {
		client := &customDomainWriteClient{}
		cmd := createCustomDomainCmd(newCustomDomainWriteCLI(client, &bytes.Buffer{}))
		// The leaf command runs without the root PreRun, so clear the required-flag
		// annotations here (stdin is non-TTY under test) to reach RunE with only --data.
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"domain":"login.example.com","type":"auth0_managed_certs"}`})

		require.NoError(t, cmd.Execute())
		assert.Equal(t, http.MethodPost, client.gotMethod)
		assert.Contains(t, client.gotURI, "/custom-domains")
	})

	t.Run("surfaces a schema validation failure before calling the API", func(t *testing.T) {
		client := &customDomainWriteClient{}
		cmd := createCustomDomainCmd(newCustomDomainWriteCLI(client, &bytes.Buffer{}))
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"domain":123,"type":"auth0_managed_certs"}`})

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schema validation")
		assert.Empty(t, client.gotMethod, "must not reach the API when validation fails")
	})
}

func TestCustomDomainsUpdateCmdData(t *testing.T) {
	client := &customDomainWriteClient{}
	cmd := updateCustomDomainCmd(newCustomDomainWriteCLI(client, &bytes.Buffer{}))
	prepareInteractivity(cmd)
	cmd.SetArgs([]string{"cd_123", "--data", `{"tls_policy":"recommended"}`})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, http.MethodPatch, client.gotMethod)
	assert.Contains(t, client.gotURI, "/custom-domains/cd_123")
}
