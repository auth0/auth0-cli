package cli

import (
	"net/http"
	"slices"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type jsonFlagsCase struct {
	name   string
	newCmd func(*cli) *cobra.Command
}

// The commands that gained --schema/--query (list) or --schema/--data (create, update)
// for flows, vault connections, event streams and log streams.
var (
	listJSONFlagsCmds = []jsonFlagsCase{
		{"flows list", listFlowsCmd},
		{"flows vault connections list", listVaultConnectionsCmd},
		{"event-streams list", listEventStreamsCmd},
	}

	createJSONFlagsCmds = []jsonFlagsCase{
		{"flows create", createFlowCmd},
		{"flows vault connections create", createVaultConnectionCmd},
		{"event-streams create", createEventStreamCmd},
		{"logs streams create eventbridge", createLogStreamsAmazonEventBridgeCmd},
		{"logs streams create eventgrid", createLogStreamsAzureEventGridCmd},
		{"logs streams create http", createLogStreamsCustomWebhookCmd},
		{"logs streams create datadog", createLogStreamsDatadogCmd},
		{"logs streams create splunk", createLogStreamsSplunkCmd},
		{"logs streams create sumo", createLogStreamsSumoLogicCmd},
	}

	updateJSONFlagsCmds = []jsonFlagsCase{
		{"flows update", updateFlowCmd},
		{"flows vault connections update", updateVaultConnectionCmd},
		{"event-streams update", updateEventStreamCmd},
		{"logs streams update eventbridge", updateLogStreamsAmazonEventBridgeCmd},
		{"logs streams update eventgrid", updateLogStreamsAzureEventGridCmd},
		{"logs streams update http", updateLogStreamsCustomWebhookCmd},
		{"logs streams update datadog", updateLogStreamsDatadogCmd},
		{"logs streams update splunk", updateLogStreamsSplunkCmd},
		{"logs streams update sumo", updateLogStreamsSumoLogicCmd},
	}
)

func TestJSONFlagsAreRegistered(t *testing.T) {
	for _, tc := range listJSONFlagsCmds {
		t.Run(tc.name, func(t *testing.T) {
			cmd := tc.newCmd(&cli{})
			assert.NotNil(t, cmd.Flags().Lookup("schema"), "missing --schema")
			assert.NotNil(t, cmd.Flags().Lookup("query"), "missing --query")
			assert.Nil(t, cmd.Flags().Lookup("data"), "list commands take no payload")
		})
	}

	for _, tc := range slices.Concat(createJSONFlagsCmds, updateJSONFlagsCmds) {
		t.Run(tc.name, func(t *testing.T) {
			cmd := tc.newCmd(&cli{})
			assert.NotNil(t, cmd.Flags().Lookup("schema"), "missing --schema")
			assert.NotNil(t, cmd.Flags().Lookup("data"), "missing --data")
			assert.Nil(t, cmd.Flags().Lookup("query"), "write commands take no query")
		})
	}
}

func TestJSONDataConflictsWithName(t *testing.T) {
	for _, tc := range slices.Concat(createJSONFlagsCmds, updateJSONFlagsCmds) {
		t.Run(tc.name, func(t *testing.T) {
			stub := &jsonWriteStub{}
			cmd := tc.newCmd(newJSONWriteCLI(stub))
			prepareInteractivity(cmd)
			cmd.SetArgs([]string{"--data", `{"name":"x"}`, "--name", "y"})

			require.ErrorContains(t, cmd.Execute(), "none of the others can be")
			assert.Empty(t, stub.gotMethod, "must not reach the API when flags conflict")
		})
	}
}

func TestJSONDataRejectsNonObjectOnCreate(t *testing.T) {
	for _, tc := range createJSONFlagsCmds {
		t.Run(tc.name, func(t *testing.T) {
			stub := &jsonWriteStub{}
			cmd := tc.newCmd(newJSONWriteCLI(stub))
			prepareInteractivity(cmd)
			cmd.SetArgs([]string{"--data", `["x"]`})

			// Schema-validated commands say "value must be an object", the others "must be a JSON object".
			assert.Regexp(t, `must be (a JSON|an) object`, cmd.Execute())
			assert.Empty(t, stub.gotMethod, "must not reach the API on a bad payload")
		})
	}
}

func TestJSONPipedPayload(t *testing.T) {
	for _, tc := range []struct {
		name       string
		newCmd     func(*cli) *cobra.Command
		args       []string
		payload    string
		wantMethod string
		wantURI    string
	}{
		{"flows create", createFlowCmd, nil, `{"name":"piped"}`, http.MethodPost, "/flows"},
		{"flows update", updateFlowCmd, []string{"af_1"}, `{"name":"piped"}`, http.MethodPatch, "/flows/af_1"},
		{"event-streams update", updateEventStreamCmd, []string{"est_1"}, `{"status":"disabled"}`, http.MethodPatch, "/event-streams/est_1"},
		{"logs streams create http", createLogStreamsCustomWebhookCmd, nil, `{"name":"s","sink":{"httpEndpoint":"https://example.com/logs"}}`, http.MethodPost, "/log-streams"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := &jsonWriteStub{}
			cmd := tc.newCmd(newJSONWriteCLI(stub))
			prepareInteractivity(cmd)
			cmd.SetArgs(tc.args)

			withPipedStdin(t, tc.payload, func() {
				require.NoError(t, cmd.Execute())
			})
			assert.Equal(t, tc.wantMethod, stub.gotMethod)
			assert.Contains(t, stub.gotURI, tc.wantURI)
		})
	}
}

func TestJSONPipedFileReferenceIsRejected(t *testing.T) {
	stub := &jsonWriteStub{}
	cmd := createFlowCmd(newJSONWriteCLI(stub))
	prepareInteractivity(cmd)

	withPipedStdin(t, "@secrets.json", func() {
		assert.ErrorContains(t, cmd.Execute(), "piped input must be a JSON payload")
	})
	assert.Empty(t, stub.gotMethod, "a piped file reference must never be read or sent")
}
