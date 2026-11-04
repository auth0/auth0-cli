package cli

import (
	"net/http"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
)

func TestWithLogStreamType(t *testing.T) {
	for name, tc := range map[string]struct{ data, want, wantErr string }{
		"adds the missing type":    {`{"name":"s"}`, `{"name":"s","type":"datadog"}`, ""},
		"keeps a matching type":    {`{"name":"s","type":"datadog"}`, `{"name":"s","type":"datadog"}`, ""},
		"rejects a different type": {`{"name":"s","type":"http"}`, "", `this command creates a "datadog" log stream`},
		"rejects a non-object":     {`["s"]`, "", "must be a JSON object"},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := withLogStreamType(tc.data, logStreamTypeDatadog)
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.JSONEq(t, tc.want, got)
		})
	}
}

// logStreamTypeCases pairs each log stream subcommand with its type, a valid sink
// for that type and a stream type that differs from it.
var logStreamTypeCases = []struct {
	streamType logStreamType
	sink       string
	other      logStreamType
	create     func(*cli) *cobra.Command
	update     func(*cli) *cobra.Command
}{
	{logStreamTypeAmazonEventBridge, `{"awsAccountId":"123456789012","awsRegion":"us-east-1"}`, logStreamTypeHTTP, createLogStreamsAmazonEventBridgeCmd, updateLogStreamsAmazonEventBridgeCmd},
	{logStreamTypeAzureEventGrid, `{"azureSubscriptionId":"00000000-0000-0000-0000-000000000000","azureRegion":"eastus","azureResourceGroup":"rg"}`, logStreamTypeHTTP, createLogStreamsAzureEventGridCmd, updateLogStreamsAzureEventGridCmd},
	{logStreamTypeHTTP, `{"httpEndpoint":"https://example.com/logs"}`, logStreamTypeDatadog, createLogStreamsCustomWebhookCmd, updateLogStreamsCustomWebhookCmd},
	{logStreamTypeDatadog, `{"datadogRegion":"us","datadogApiKey":"k"}`, logStreamTypeHTTP, createLogStreamsDatadogCmd, updateLogStreamsDatadogCmd},
	{logStreamTypeSplunk, `{"splunkDomain":"example.splunkcloud.com","splunkToken":"t","splunkPort":"8088","splunkSecure":true}`, logStreamTypeHTTP, createLogStreamsSplunkCmd, updateLogStreamsSplunkCmd},
	{logStreamTypeSumo, `{"sumoSourceAddress":"https://endpoint.collection.sumologic.com/receiver/v1/http/t"}`, logStreamTypeHTTP, createLogStreamsSumoLogicCmd, updateLogStreamsSumoLogicCmd},
}

func TestLogStreamsCreateCmdData(t *testing.T) {
	for _, tc := range logStreamTypeCases {
		t.Run(string(tc.streamType)+" fills in the type and posts to /log-streams", func(t *testing.T) {
			stub := &jsonWriteStub{}
			cmd := tc.create(newJSONWriteCLI(stub))
			// The leaf command runs without the root PreRun, so clear the required-flag
			// annotations here (stdin is non-TTY under test) to reach RunE with only --data.
			prepareInteractivity(cmd)
			cmd.SetArgs([]string{"--data", `{"name":"s","sink":` + tc.sink + `}`})

			require.NoError(t, cmd.Execute())
			assert.Equal(t, http.MethodPost, stub.gotMethod)
			assert.Contains(t, stub.gotURI, "/log-streams")
			assert.Contains(t, stub.gotBody, `"type":"`+string(tc.streamType)+`"`)
		})

		t.Run(string(tc.streamType)+" rejects a payload for a different type", func(t *testing.T) {
			stub := &jsonWriteStub{}
			cmd := tc.create(newJSONWriteCLI(stub))
			prepareInteractivity(cmd)
			cmd.SetArgs([]string{"--data", `{"name":"s","type":"` + string(tc.other) + `"}`})

			assert.ErrorContains(t, cmd.Execute(), `creates a "`+string(tc.streamType)+`" log stream`)
			assert.Empty(t, stub.gotMethod)
		})
	}
}

func TestLogStreamsUpdateCmdData(t *testing.T) {
	newCLI := func(t *testing.T, stub *jsonWriteStub, existingType logStreamType) *cli {
		ctrl := gomock.NewController(t)
		logStreamAPI := mock.NewMockLogStreamAPI(ctrl)
		logStreamAPI.EXPECT().
			Read(gomock.Any(), "lst_123").
			Return(&management.LogStream{ID: auth0.String("lst_123"), Type: auth0.String(string(existingType))}, nil)

		c := newJSONWriteCLI(stub)
		c.api.LogStream = logStreamAPI
		return c
	}

	for _, tc := range logStreamTypeCases {
		t.Run(string(tc.streamType)+" sends the payload to PATCH /log-streams/{id}", func(t *testing.T) {
			stub := &jsonWriteStub{}
			cmd := tc.update(newCLI(t, stub, tc.streamType))
			prepareInteractivity(cmd)
			cmd.SetArgs([]string{"lst_123", "--data", `{"status":"paused"}`})

			require.NoError(t, cmd.Execute())
			assert.Equal(t, http.MethodPatch, stub.gotMethod)
			assert.Contains(t, stub.gotURI, "/log-streams/lst_123")
			assert.JSONEq(t, `{"status":"paused"}`, stub.gotBody)
		})

		t.Run(string(tc.streamType)+" rejects a stream of a different type", func(t *testing.T) {
			stub := &jsonWriteStub{}
			cmd := tc.update(newCLI(t, stub, tc.other))
			prepareInteractivity(cmd)
			cmd.SetArgs([]string{"lst_123", "--data", `{"status":"paused"}`})

			assert.ErrorContains(t, cmd.Execute(), `instead of "`+string(tc.streamType)+`"`)
			assert.Empty(t, stub.gotMethod)
		})
	}
}
