package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
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

// rulesFixtureSchemaDoc mirrors the $ref layout of the live spec for the rule operations,
// so --data validation, the piped-input warnings and --schema run offline.
const rulesFixtureSchemaDoc = `{
  "openapi": "3.0.0",
  "info": {"title": "fixture", "version": "1.0.0"},
  "paths": {
    "/rules": {
      "get": {"operationId": "get_rules", "parameters": [
        {"name": "page", "in": "query", "schema": {"type": "integer"}},
        {"name": "per_page", "in": "query", "schema": {"type": "integer"}},
        {"name": "include_totals", "in": "query", "schema": {"type": "boolean"}},
        {"name": "enabled", "in": "query", "schema": {"type": "boolean"}}]},
      "post": {"operationId": "post_rules", "requestBody": {"content": {"application/json": {"schema": {
        "$ref": "#/components/schemas/CreateRuleRequestContent"}}}}}
    },
    "/rules/{id}": {
      "patch": {"operationId": "patch_rules_by_id", "requestBody": {"content": {"application/json": {"schema": {
        "$ref": "#/components/schemas/UpdateRuleRequestContent"}}}}}
    }
  },
  "components": {"schemas": {
    "CreateRuleRequestContent": {"type": "object", "additionalProperties": false, "required": ["name", "script"], "properties": {
      "name": {"type": "string", "pattern": "^[a-zA-Z0-9]([ \\-a-zA-Z0-9]*[a-zA-Z0-9])?$"},
      "script": {"type": "string", "minLength": 1},
      "order": {"type": "number"},
      "enabled": {"type": "boolean"}}},
    "UpdateRuleRequestContent": {"type": "object", "additionalProperties": false, "properties": {
      "name": {"type": "string", "pattern": "^[a-zA-Z0-9]([ \\-a-zA-Z0-9]*[a-zA-Z0-9])?$"},
      "script": {"type": "string", "minLength": 1},
      "order": {"type": "number"},
      "enabled": {"type": "boolean"}}}
  }}
}`

// useRulesFixtureSchema points --data validation, the piped-input check and --schema at
// the fixture for the test. It swaps a package global, so callers must not use t.Parallel().
func useRulesFixtureSchema(t *testing.T) {
	t.Helper()
	doc, err := openapi.LoadDocFromData([]byte(rulesFixtureSchemaDoc))
	require.NoError(t, err)

	prev := newSchemaManager
	newSchemaManager = func() (*openapi.SchemaManager, error) {
		return openapi.NewSchemaManagerFromDoc(doc), nil
	}
	t.Cleanup(func() { newSchemaManager = prev })
}

const ruleResponse = `{"id":"rul_123","name":"my-rule","script":"function () {}","order":1,"enabled":false}`

// newRulePipeCLI wires a cli whose rule SDK calls hit ruleAPI and whose warnings are
// captured, for the legacy piped-input path that does not go through runJSONWrite.
func newRulePipeCLI(ruleAPI *mock.MockRuleAPI) (*cli, *bytes.Buffer) {
	var messages bytes.Buffer
	return &cli{
		api:      &auth0.API{Rule: ruleAPI},
		renderer: &display.Renderer{MessageWriter: &messages, ResultWriter: io.Discard},
	}, &messages
}

func TestRulesCmdSchema(t *testing.T) {
	useRulesFixtureSchema(t)

	tests := []struct {
		name      string
		newCmd    func(*cli) *cobra.Command
		method    string
		path      string
		schemaKey string
		// Marker sits behind a $ref (or is a query parameter), so it only appears once resolved.
		marker string
	}{
		{"list", listRulesCmd, http.MethodGet, "/rules", "query_params_schema", "include_totals"},
		{"create", createRuleCmd, http.MethodPost, "/rules", "request_schema", "script"},
		{"update", updateRuleCmd, http.MethodPatch, "/rules/{id}", "request_schema", "script"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			// No API client and no ID: --schema must return before anything touches cli.api.
			c := &cli{renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: &out}}
			cmd := tc.newCmd(c)
			// The leaf runs without the root PreRun, so clear the required flags that create declares.
			prepareInteractivity(cmd)
			cmd.SetArgs([]string{"--schema", "--json"})

			require.NoError(t, cmd.Execute())

			var schema map[string]interface{}
			require.NoError(t, json.Unmarshal(out.Bytes(), &schema))
			assert.Equal(t, tc.method, schema["method"])
			assert.Equal(t, tc.path, schema["path"])
			assert.NotEmpty(t, schema[tc.schemaKey])
			assert.Contains(t, out.String(), tc.marker)
			assert.NotContains(t, out.String(), "$ref")
			assert.NotContains(t, out.String(), "#/components")
		})
	}
}

func TestRulesListCmdInvalidQuery(t *testing.T) {
	c := &cli{
		renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: io.Discard},
		api:      &auth0.API{},
	}

	cmd := listRulesCmd(c)
	cmd.SetArgs([]string{"--query", "not-valid-json"})

	assert.ErrorContains(t, cmd.Execute(), "invalid --query value")
}

func TestRulesWriteCmdData(t *testing.T) {
	useRulesFixtureSchema(t)

	tests := []struct {
		name   string
		newCmd func(*cli) *cobra.Command
		args   []string
		method string
		uri    string
	}{
		{
			name:   "create",
			newCmd: createRuleCmd,
			args:   []string{"--data", `{"name":"my-rule","script":"function () {}","enabled":false}`},
			method: http.MethodPost,
			uri:    "https://example.test/api/v2/rules",
		},
		{
			name:   "update",
			newCmd: updateRuleCmd,
			args:   []string{"rul_123", "--data", `{"enabled":false,"order":2}`},
			method: http.MethodPatch,
			uri:    "https://example.test/api/v2/rules/rul_123",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stub := &jsonWriteStub{response: ruleResponse}
			cmd := tc.newCmd(newJSONWriteCLI(stub))
			prepareInteractivity(cmd)
			cmd.SetArgs(tc.args)

			require.NoError(t, cmd.Execute())
			assert.Equal(t, tc.method, stub.gotMethod)
			assert.Equal(t, tc.uri, stub.gotURI)
			assert.Equal(t, tc.args[len(tc.args)-1], stub.gotBody, "the payload must be sent verbatim")
		})
	}
}

func TestRulesWriteCmdDataRejectsInvalidSchema(t *testing.T) {
	useRulesFixtureSchema(t)

	tests := []struct {
		name       string
		newCmd     func(*cli) *cobra.Command
		args       []string
		wantFields []string
	}{
		{
			name:       "create with a bad name and no script",
			newCmd:     createRuleCmd,
			args:       []string{"--data", `{"name":"bad_name!","enabled":true}`},
			wantFields: []string{"name", "script"},
		},
		{
			name:       "update with an id in the payload",
			newCmd:     updateRuleCmd,
			args:       []string{"rul_123", "--data", `{"id":"rul_123","enabled":false}`},
			wantFields: []string{`property "id" is unsupported`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stub := &jsonWriteStub{}
			cmd := tc.newCmd(newJSONWriteCLI(stub))
			prepareInteractivity(cmd)
			cmd.SetArgs(tc.args)

			err := cmd.Execute()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "schema validation failed")
			for _, field := range tc.wantFields {
				assert.Contains(t, err.Error(), field)
			}
			assert.Empty(t, stub.gotMethod, "must not reach the API when validation fails")

			var vErr validationError
			require.ErrorAs(t, err, &vErr)
			for _, text := range []string{err.Error(), string(vErr.details)} {
				assert.NotContains(t, text, "$ref")
				assert.NotContains(t, text, "#/components")
			}
		})
	}
}

func TestRulesDataConflictsWithFlags(t *testing.T) {
	tests := map[string]struct {
		newCmd func(*cli) *cobra.Command
		args   []string
	}{
		"create": {createRuleCmd, []string{"--data", `{"name":"my-rule","script":"x"}`, "--name", "other"}},
		"update": {updateRuleCmd, []string{"rul_123", "--data", `{"enabled":false}`, "--enabled=true"}},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			stub := &jsonWriteStub{}
			cmd := tc.newCmd(newJSONWriteCLI(stub))
			prepareInteractivity(cmd)
			cmd.SetArgs(tc.args)

			assert.ErrorContains(t, cmd.Execute(), "none of the others can be")
			assert.Empty(t, stub.gotMethod, "must not reach the API when flags conflict")
		})
	}
}

// Plain piped JSON keeps the legacy behavior and only warns, until a future release
// turns these warnings into errors.
func TestRulesCreateCmdPipedInput(t *testing.T) {
	useRulesFixtureSchema(t)

	tests := []struct {
		name        string
		piped       string
		args        []string
		wantName    string
		wantWarns   []string
		unwantWarns []string
	}{
		{
			name:        "valid payload creates the rule without warnings",
			piped:       `{"name":"my-rule","script":"function () {}","enabled":false}`,
			wantName:    "my-rule",
			unwantWarns: []string{"does not match the rule schema", "Ignoring flags"},
		},
		{
			name:      "unknown field warns and still creates the rule",
			piped:     `{"name":"my-rule","script":"function () {}","extra":1}`,
			wantName:  "my-rule",
			wantWarns: []string{"does not match the rule schema", `property "extra" is unsupported`, "a future release will reject it"},
		},
		{
			name:      "flags are ignored with a warning",
			piped:     `{"name":"my-rule","script":"function () {}"}`,
			args:      []string{"--name", "other-name"},
			wantName:  "my-rule",
			wantWarns: []string{"Ignoring flags (name)"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			ruleAPI := mock.NewMockRuleAPI(ctrl)
			var created *management.Rule
			ruleAPI.EXPECT().Create(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, r *management.Rule, _ ...management.RequestOption) error {
					created = r
					return nil
				})

			c, messages := newRulePipeCLI(ruleAPI)
			cmd := createRuleCmd(c)
			prepareInteractivity(cmd)
			cmd.SetArgs(append([]string{}, tc.args...))

			withPipedStdin(t, tc.piped, func() {
				require.NoError(t, cmd.Execute())
			})
			require.NotNil(t, created)
			assert.Equal(t, tc.wantName, created.GetName())
			for _, warn := range tc.wantWarns {
				assert.Contains(t, messages.String(), warn)
			}
			for _, warn := range tc.unwantWarns {
				assert.NotContains(t, messages.String(), warn)
			}
		})
	}

	t.Run("malformed JSON keeps the legacy error", func(t *testing.T) {
		// No Create expectation: gomock fails the test if the API is called.
		c, messages := newRulePipeCLI(mock.NewMockRuleAPI(gomock.NewController(t)))
		cmd := createRuleCmd(c)
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{})

		withPipedStdin(t, `{"name":`, func() {
			assert.ErrorContains(t, cmd.Execute(), "failed to unmarshal JSON input")
		})
		assert.NotContains(t, messages.String(), "does not match the rule schema")
	})
}

// The payload "id" keeps priority over the argument, as before; the argument is used
// only when the payload has no "id".
func TestRulesUpdateCmdPipedInputRuleID(t *testing.T) {
	useRulesFixtureSchema(t)

	tests := []struct {
		name        string
		piped       string
		args        []string
		wantID      string
		wantWarns   []string
		unwantWarns []string
	}{
		{
			name:        "argument is used when the payload has no id",
			piped:       `{"order":3}`,
			args:        []string{"rul_arg"},
			wantID:      "rul_arg",
			unwantWarns: []string{"Ignoring the rule ID argument", "does not match the rule schema"},
		},
		{
			name:      "payload id wins over a different argument",
			piped:     `{"id":"rul_payload","order":4}`,
			args:      []string{"rul_arg"},
			wantID:    "rul_payload",
			wantWarns: []string{`Ignoring the rule ID argument "rul_arg"`, `property "id" is unsupported`},
		},
		{
			name:        "matching payload id and argument",
			piped:       `{"id":"rul_payload","order":5}`,
			args:        []string{"rul_payload"},
			wantID:      "rul_payload",
			wantWarns:   []string{`property "id" is unsupported`},
			unwantWarns: []string{"Ignoring the rule ID argument"},
		},
		{
			name:        "payload id without an argument",
			piped:       `{"id":"rul_payload","order":6}`,
			wantID:      "rul_payload",
			wantWarns:   []string{`property "id" is unsupported`},
			unwantWarns: []string{"Ignoring the rule ID argument"},
		},
		{
			name:      "flags are ignored with a warning",
			piped:     `{"order":7}`,
			args:      []string{"rul_arg", "--enabled=false"},
			wantID:    "rul_arg",
			wantWarns: []string{"Ignoring flags (enabled)"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			ruleAPI := mock.NewMockRuleAPI(ctrl)
			var sent *management.Rule
			ruleAPI.EXPECT().Update(gomock.Any(), tc.wantID, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ string, r *management.Rule, _ ...management.RequestOption) error {
					sent = r
					return nil
				})

			c, messages := newRulePipeCLI(ruleAPI)
			cmd := updateRuleCmd(c)
			prepareInteractivity(cmd)
			cmd.SetArgs(append([]string{}, tc.args...))

			withPipedStdin(t, tc.piped, func() {
				require.NoError(t, cmd.Execute())
			})
			require.NotNil(t, sent)
			assert.Nil(t, sent.ID, "the id must not be sent in the body")
			for _, warn := range tc.wantWarns {
				assert.Contains(t, messages.String(), warn)
			}
			for _, warn := range tc.unwantWarns {
				assert.NotContains(t, messages.String(), warn)
			}
		})
	}
}
