package cli

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVaultConnectionsListCmdInvalidQuery(t *testing.T) {
	cmd := listVaultConnectionsCmd(newJSONWriteCLI(&jsonWriteStub{}))
	cmd.SetArgs([]string{"--query", "not-valid-json"})

	assert.ErrorContains(t, cmd.Execute(), "invalid --query value")
}

func TestVaultConnectionsCreateCmdData(t *testing.T) {
	t.Run("sends the payload to POST /flows/vault/connections", func(t *testing.T) {
		stub := &jsonWriteStub{}
		cmd := createVaultConnectionCmd(newJSONWriteCLI(stub))
		// The leaf command runs without the root PreRun, so clear the required-flag
		// annotations here (stdin is non-TTY under test) to reach RunE with only --data.
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"name":"My Connection","app_id":"HTTP","setup":{"type":"BEARER","token":"t"}}`})

		require.NoError(t, cmd.Execute())
		assert.Equal(t, http.MethodPost, stub.gotMethod)
		assert.Contains(t, stub.gotURI, "/flows/vault/connections")
	})

	t.Run("surfaces a schema validation failure before calling the API", func(t *testing.T) {
		stub := &jsonWriteStub{}
		cmd := createVaultConnectionCmd(newJSONWriteCLI(stub))
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"name":123,"app_id":"HTTP"}`})

		assert.ErrorContains(t, cmd.Execute(), "schema validation")
		assert.Empty(t, stub.gotMethod, "must not reach the API when validation fails")
	})
}

func TestVaultConnectionsUpdateCmdData(t *testing.T) {
	stub := &jsonWriteStub{}
	cmd := updateVaultConnectionCmd(newJSONWriteCLI(stub))
	prepareInteractivity(cmd)
	cmd.SetArgs([]string{"ac_123", "--data", `{"name":"New Name"}`})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, http.MethodPatch, stub.gotMethod)
	assert.Contains(t, stub.gotURI, "/flows/vault/connections/ac_123")
}
