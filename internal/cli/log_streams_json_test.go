package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/openapi"
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

func TestLogStreamsUpdateCmdResolvesPayloadBeforeReading(t *testing.T) {
	for _, tc := range logStreamTypeCases {
		t.Run(string(tc.streamType), func(t *testing.T) {
			// The mock has no Read expectation, so reading the stream fails the test.
			stub := &jsonWriteStub{}
			c := newJSONWriteCLI(stub)
			c.api.LogStream = mock.NewMockLogStreamAPI(gomock.NewController(t))
			cmd := tc.update(c)
			prepareInteractivity(cmd)
			cmd.SetArgs([]string{"lst_123"})

			withPipedStdin(t, "@payload.json", func() {
				assert.ErrorContains(t, cmd.Execute(), "not a file reference")
			})
			assert.Empty(t, stub.gotMethod)
		})
	}
}

func TestLogStreamsListCmdSchema(t *testing.T) {
	doc, err := openapi.LoadDocFromData([]byte(`{"openapi":"3.0.0","info":{"title":"fixture","version":"1"},
		"paths":{"/log-streams":{"get":{"operationId":"get_log-streams","summary":"Get log streams"}}}}`))
	require.NoError(t, err)
	prev := newSchemaManager
	newSchemaManager = func() (*openapi.SchemaManager, error) { return openapi.NewSchemaManagerFromDoc(doc), nil }
	t.Cleanup(func() { newSchemaManager = prev })

	var out bytes.Buffer
	cmd := listLogStreamsCmd(&cli{renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: &out}})
	cmd.SetArgs([]string{"--schema", "--json-compact"})

	require.NoError(t, cmd.Execute())
	var printed map[string]interface{}
	require.NoError(t, json.Unmarshal(out.Bytes(), &printed))
	assert.Equal(t, http.MethodGet, printed["method"])
	assert.Equal(t, "/log-streams", printed["path"])
}

func TestLogStreamsListCmdQuery(t *testing.T) {
	for _, flag := range []string{"--query", "-q"} {
		t.Run(flag, func(t *testing.T) {
			var gotMethod, gotPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath = r.Method, r.URL.Path
				_, _ = w.Write([]byte(`[{"id":"lst_123"}]`))
			}))
			t.Cleanup(server.Close)

			var out bytes.Buffer
			cmd := listLogStreamsCmd(&cli{
				api:      &auth0.API{HTTPClient: &mockHTTPClientAPI{baseURL: server.URL}},
				renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: &out},
			})
			cmd.SetArgs([]string{flag, `{}`})

			require.NoError(t, cmd.Execute())
			assert.Equal(t, http.MethodGet, gotMethod)
			assert.Equal(t, "/api/v2/log-streams", gotPath)
			assert.Contains(t, out.String(), "lst_123")
		})
	}
}
