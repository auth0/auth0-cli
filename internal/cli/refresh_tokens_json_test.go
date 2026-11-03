package cli

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefreshTokensUpdateCmdData(t *testing.T) {
	t.Run("sends the validated payload to PATCH /refresh-tokens/{id}", func(t *testing.T) {
		client := &brandingWriteClient{}
		cmd := updateRefreshTokenCmd(newBrandingWriteCLI(client, &bytes.Buffer{}))
		// The leaf command runs without the root PreRun, so clear the required-flag
		// annotations here (stdin is non-TTY under test) to reach RunE with only --data.
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"rt_123", "--data", `{"refresh_token_metadata":{"key":"value"}}`})

		require.NoError(t, cmd.Execute())
		assert.Equal(t, http.MethodPatch, client.gotMethod)
		assert.Contains(t, client.gotURI, "/refresh-tokens/rt_123")
	})

	t.Run("surfaces a schema validation failure before calling the API", func(t *testing.T) {
		client := &brandingWriteClient{}
		cmd := updateRefreshTokenCmd(newBrandingWriteCLI(client, &bytes.Buffer{}))
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"rt_123", "--data", `{"refresh_token_metadata":"not-an-object"}`})

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schema validation")
		assert.Empty(t, client.gotMethod, "must not reach the API when validation fails")
	})
}
