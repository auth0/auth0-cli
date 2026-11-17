package cli

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlowsListCmdInvalidQuery(t *testing.T) {
	cmd := listFlowsCmd(newJSONWriteCLI(&jsonWriteStub{}))
	cmd.SetArgs([]string{"--query", "not-valid-json"})

	assert.ErrorContains(t, cmd.Execute(), "invalid --query value")
}

func TestFlowsCreateCmdData(t *testing.T) {
	t.Run("sends the payload to POST /flows", func(t *testing.T) {
		stub := &jsonWriteStub{}
		cmd := createFlowCmd(newJSONWriteCLI(stub))
		// The leaf command runs without the root PreRun, so clear the required-flag
		// annotations here (stdin is non-TTY under test) to reach RunE with only --data.
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"name":"My Flow","actions":[]}`})

		require.NoError(t, cmd.Execute())
		assert.Equal(t, http.MethodPost, stub.gotMethod)
		assert.Contains(t, stub.gotURI, "/flows")
		assert.JSONEq(t, `{"name":"My Flow","actions":[]}`, stub.gotBody)
	})

	t.Run("surfaces a schema validation failure before calling the API", func(t *testing.T) {
		stub := &jsonWriteStub{}
		cmd := createFlowCmd(newJSONWriteCLI(stub))
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"name":123}`})

		assert.ErrorContains(t, cmd.Execute(), "schema validation")
		assert.Empty(t, stub.gotMethod, "must not reach the API when validation fails")
	})

	t.Run("rejects --data combined with --name", func(t *testing.T) {
		stub := &jsonWriteStub{}
		cmd := createFlowCmd(newJSONWriteCLI(stub))
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"name":"My Flow"}`, "--name", "Other"})

		assert.ErrorContains(t, cmd.Execute(), "none of the others can be")
		assert.Empty(t, stub.gotMethod)
	})
}

func TestFlowsUpdateCmdData(t *testing.T) {
	stub := &jsonWriteStub{}
	cmd := updateFlowCmd(newJSONWriteCLI(stub))
	prepareInteractivity(cmd)
	cmd.SetArgs([]string{"af_123", "--data", `{"name":"New Name"}`})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, http.MethodPatch, stub.gotMethod)
	assert.Contains(t, stub.gotURI, "/flows/af_123")
}
