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

// userWriteClient captures the method and URI runJSONWrite sends, so a --data
// test can assert the command built the right resource path.
type userWriteClient struct {
	rawHTTPClientStub
	gotMethod string
	gotURI    string
}

// Request records the method and URI, then returns nil so runJSONWrite can
// unmarshal the sent payload back for rendering.
func (c *userWriteClient) Request(_ context.Context, method string, uri string, _ interface{}, _ ...management.RequestOption) error {
	c.gotMethod = method
	c.gotURI = uri
	return nil
}

func newUserWriteCLI(client *userWriteClient, stdout *bytes.Buffer) *cli {
	return &cli{
		api: &auth0.API{HTTPClient: client},
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  stdout,
		},
	}
}

func TestUsersCreateCmdData(t *testing.T) {
	t.Run("sends the validated payload to POST /users", func(t *testing.T) {
		client := &userWriteClient{}
		cmd := createUserCmd(newUserWriteCLI(client, &bytes.Buffer{}))
		// The leaf command runs without the root PreRun, so clear the required-flag
		// annotations here (stdin is non-TTY under test) to reach RunE with only --data.
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"connection":"Username-Password-Authentication","email":"json@example.com","password":"Str0ngP@ssw0rd!"}`})

		require.NoError(t, cmd.Execute())
		assert.Equal(t, http.MethodPost, client.gotMethod)
		assert.Contains(t, client.gotURI, "/users")
	})

	t.Run("surfaces a schema validation failure before calling the API", func(t *testing.T) {
		client := &userWriteClient{}
		cmd := createUserCmd(newUserWriteCLI(client, &bytes.Buffer{}))
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"connection":123,"email":"json@example.com"}`})

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schema validation")
		assert.Empty(t, client.gotMethod, "must not reach the API when validation fails")
	})
}

func TestUsersUpdateCmdData(t *testing.T) {
	client := &userWriteClient{}
	cmd := updateUserCmd(newUserWriteCLI(client, &bytes.Buffer{}))
	prepareInteractivity(cmd)
	cmd.SetArgs([]string{"auth0|abc123", "--data", `{"user_metadata":{"team":"eng"}}`})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, http.MethodPatch, client.gotMethod)
	assert.Contains(t, client.gotURI, "/users/auth0|abc123")
}
