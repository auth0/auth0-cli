package cli

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventStreamsListCmdInvalidQuery(t *testing.T) {
	cmd := listEventStreamsCmd(newJSONWriteCLI(&jsonWriteStub{}))
	cmd.SetArgs([]string{"--query", "not-valid-json"})

	assert.ErrorContains(t, cmd.Execute(), "invalid --query value")
}

func TestEventStreamsCreateCmdData(t *testing.T) {
	t.Run("sends a bearer webhook payload as-is, which the published schema rejects", func(t *testing.T) {
		data := `{"name":"es","subscriptions":[{"event_type":"user.created"}],` +
			`"destination":{"type":"webhook","configuration":{"webhook_endpoint":"https://example.com",` +
			`"webhook_authorization":{"method":"bearer","token":"t"}}}}`
		stub := &jsonWriteStub{}
		cmd := createEventStreamCmd(newJSONWriteCLI(stub))
		// The leaf command runs without the root PreRun, so clear the required-flag
		// annotations here (stdin is non-TTY under test) to reach RunE with only --data.
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", data})

		require.NoError(t, cmd.Execute())
		assert.Equal(t, http.MethodPost, stub.gotMethod)
		assert.Contains(t, stub.gotURI, "/event-streams")
		assert.JSONEq(t, data, stub.gotBody)
	})

	t.Run("rejects a payload that is not a JSON object", func(t *testing.T) {
		stub := &jsonWriteStub{}
		cmd := createEventStreamCmd(newJSONWriteCLI(stub))
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `["es"]`})

		assert.ErrorContains(t, cmd.Execute(), "must be a JSON object")
		assert.Empty(t, stub.gotMethod)
	})
}

func TestEventStreamsUpdateCmdData(t *testing.T) {
	stub := &jsonWriteStub{}
	cmd := updateEventStreamCmd(newJSONWriteCLI(stub))
	prepareInteractivity(cmd)
	cmd.SetArgs([]string{"est_123", "--data", `{"status":"disabled"}`})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, http.MethodPatch, stub.gotMethod)
	assert.Contains(t, stub.gotURI, "/event-streams/est_123")
}
