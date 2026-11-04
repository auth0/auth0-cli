package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/config"
	"github.com/auth0/auth0-cli/internal/display"
)

type rawHTTPClientStub struct {
	method   string
	payload  interface{}
	response json.RawMessage
}

func (s *rawHTTPClientStub) NewRequest(
	ctx context.Context,
	method string,
	uri string,
	payload interface{},
	_ ...management.RequestOption,
) (*http.Request, error) {
	s.method = method
	s.payload = payload
	return http.NewRequestWithContext(ctx, method, uri, nil)
}

func (s *rawHTTPClientStub) Do(_ *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(string(s.response))),
	}, nil
}

func (s *rawHTTPClientStub) Request(
	context.Context,
	string,
	string,
	interface{},
	...management.RequestOption,
) error {
	return nil
}

func (s *rawHTTPClientStub) URI(path ...string) string {
	return "https://example.test/api/v2/" + strings.Join(path, "/")
}

func newRawTestCLI(stub *rawHTTPClientStub, stdout *bytes.Buffer) *cli {
	return &cli{
		api: &auth0.API{HTTPClient: stub},
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  stdout,
		},
	}
}

func TestApplyRawNameOverride(t *testing.T) {
	body := json.RawMessage(`{"name":"Original","actions":[{"id":"a1","type":"HTTP"}]}`)

	got, err := applyRawNameOverride(body, "Renamed")
	require.NoError(t, err)

	var obj map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(got, &obj))
	assert.JSONEq(t, `"Renamed"`, string(obj["name"]))
	assert.Contains(t, string(obj["actions"]), `"HTTP"`)
}

func TestApplyRawNameOverrideNoopWhenEmpty(t *testing.T) {
	body := json.RawMessage(`{"actions":[]}`)

	got, err := applyRawNameOverride(body, "")
	require.NoError(t, err)
	assert.Equal(t, body, got)
}

func TestApplyRawNameOverrideRejectsNonObject(t *testing.T) {
	_, err := applyRawNameOverride(json.RawMessage(`[]`), "New")
	assert.ErrorContains(t, err, "cannot unmarshal array")
}

func TestFormatBuilderPageURL(t *testing.T) {
	cfg := &config.Config{
		Tenants: config.Tenants{
			"example.us.auth0.com":   {Name: "example"},
			"my-tenant.eu.auth0.com": {Name: "my-tenant"},
			"dev-tti06f6y.auth0.com": {Name: "dev-tti06f6y"},
			"no-name.us.auth0.com":   {Name: ""},
		},
	}

	tests := []struct {
		name     string
		tenant   string
		path     string
		expected string
	}{
		{
			name:     "builds a flow edit URL",
			tenant:   "example.us.auth0.com",
			path:     "flows/af_123/edit",
			expected: "https://forms.auth0.com/tenants/us/example/flows/af_123/edit",
		},
		{
			name:     "builds a vault app URL in a non-us region",
			tenant:   "my-tenant.eu.auth0.com",
			path:     "vault/apps/HTTP/edit",
			expected: "https://forms.auth0.com/tenants/eu/my-tenant/vault/apps/HTTP/edit",
		},
		{
			name:     "defaults to us for a three-part PUS1 domain",
			tenant:   "dev-tti06f6y.auth0.com",
			path:     "vault/apps/JWT/edit",
			expected: "https://forms.auth0.com/tenants/us/dev-tti06f6y/vault/apps/JWT/edit",
		},
		{
			name:     "returns empty when the path is missing",
			tenant:   "example.us.auth0.com",
			path:     "",
			expected: "",
		},
		{
			name:     "returns empty when the tenant is missing",
			tenant:   "",
			path:     "flows/af_123/edit",
			expected: "",
		},
		{
			name:     "returns empty when the domain has too few parts",
			tenant:   "invalid",
			path:     "flows/af_123/edit",
			expected: "",
		},
		{
			name:     "returns empty when the tenant name is unknown",
			tenant:   "no-name.us.auth0.com",
			path:     "flows/af_123/edit",
			expected: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, formatBuilderPageURL(test.tenant, cfg, test.path))
		})
	}
}

func TestCreateFlowCmdScaffoldFromName(t *testing.T) {
	stub := &rawHTTPClientStub{
		response: json.RawMessage(`{"id":"flow_1","name":"My Flow","actions":[]}`),
	}
	stdout := &bytes.Buffer{}
	c := newRawTestCLI(stub, stdout)

	cmd := createFlowCmd(c)
	cmd.SetArgs([]string{"--name", "My Flow"})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, http.MethodPost, stub.method)
	require.IsType(t, json.RawMessage{}, stub.payload)
	assert.JSONEq(t, `{"name":"My Flow","actions":[]}`, string(stub.payload.(json.RawMessage)))
	assert.Contains(t, stdout.String(), "My Flow")
}

func TestCreateFlowCmdFromFilePreservesActions(t *testing.T) {
	body := []byte(`{"actions":[{"id":"a1","type":"HTTP","action":"SEND_REQUEST","params":{"method":"GET","url":"https://x.test"}}]}`)
	path := filepath.Join(t.TempDir(), "flow.json")
	require.NoError(t, os.WriteFile(path, body, 0600))

	stub := &rawHTTPClientStub{
		response: json.RawMessage(`{"id":"flow_2","name":"Rich Flow","actions":[{"id":"a1","type":"HTTP"}]}`),
	}
	stdout := &bytes.Buffer{}
	c := newRawTestCLI(stub, stdout)

	cmd := createFlowCmd(c)
	cmd.SetArgs([]string{"--actions-file", path, "--name", "Rich Flow"})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, http.MethodPost, stub.method)
	require.IsType(t, json.RawMessage{}, stub.payload)
	assert.Contains(t, string(stub.payload.(json.RawMessage)), `"SEND_REQUEST"`)
	// The name comes from --name and is injected into the body, not read from the file.
	assert.Contains(t, string(stub.payload.(json.RawMessage)), `"Rich Flow"`)
	assert.Contains(t, stdout.String(), "1 actions")
}

func TestCreateFlowCmdRejectsNameInActionsFile(t *testing.T) {
	body := []byte(`{"name":"Rich Flow","actions":[]}`)
	path := filepath.Join(t.TempDir(), "flow.json")
	require.NoError(t, os.WriteFile(path, body, 0600))

	stub := &rawHTTPClientStub{}
	stdout := &bytes.Buffer{}
	c := newRawTestCLI(stub, stdout)

	cmd := createFlowCmd(c)
	cmd.SetArgs([]string{"--actions-file", path, "--name", "Rich Flow"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not contain a top-level \"name\" field")
}

func TestCreateFlowCmdRequiresName(t *testing.T) {
	stub := &rawHTTPClientStub{}
	stdout := &bytes.Buffer{}
	c := newRawTestCLI(stub, stdout)

	cmd := createFlowCmd(c)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "flow name is required")
}

func TestUpdateFlowCmdNameOnlyMergePreservesActions(t *testing.T) {
	stub := &rawHTTPClientStub{
		response: json.RawMessage(`{"id":"flow_3","name":"New Name","actions":[{"id":"a1","type":"HTTP"}]}`),
	}
	stdout := &bytes.Buffer{}
	c := newRawTestCLI(stub, stdout)

	cmd := updateFlowCmd(c)
	cmd.SetArgs([]string{"flow_3", "--name", "New Name"})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, http.MethodPatch, stub.method)
	require.IsType(t, json.RawMessage{}, stub.payload)
	// A name-only merge must send only the name so the API preserves the actions graph.
	assert.JSONEq(t, `{"name":"New Name"}`, string(stub.payload.(json.RawMessage)))
}
