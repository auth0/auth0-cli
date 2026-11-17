package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/openapi"
)

// networkACLFixtureSchemaDoc mirrors the $ref layout of the live spec for the network ACL
// operations, including the format-only CIDR oneOf that makes local validation unusable.
const networkACLFixtureSchemaDoc = `{
  "openapi": "3.0.0",
  "info": {"title": "fixture", "version": "1.0.0"},
  "paths": {
    "/network-acls": {
      "get": {"operationId": "get_network-acls", "parameters": [
        {"name": "page", "in": "query", "schema": {"type": "integer"}},
        {"name": "per_page", "in": "query", "schema": {"type": "integer"}},
        {"name": "include_totals", "in": "query", "schema": {"type": "boolean"}}]},
      "post": {"operationId": "post_network-acls", "requestBody": {"content": {"application/json": {"schema": {
        "$ref": "#/components/schemas/CreateNetworkAclRequestContent"}}}}}
    },
    "/network-acls/{id}": {
      "patch": {"operationId": "patch_network-acls_by_id", "requestBody": {"content": {"application/json": {"schema": {
        "$ref": "#/components/schemas/UpdateNetworkAclRequestContent"}}}}}
    }
  },
  "components": {"schemas": {
    "CreateNetworkAclRequestContent": {"type": "object", "additionalProperties": false, "required": ["description", "active", "rule"], "properties": {
      "description": {"type": "string", "maxLength": 255},
      "active": {"type": "boolean"},
      "priority": {"type": "number", "minimum": 1, "maximum": 100},
      "rule": {"$ref": "#/components/schemas/NetworkAclRule"}}},
    "UpdateNetworkAclRequestContent": {"type": "object", "additionalProperties": false, "properties": {
      "description": {"type": "string", "maxLength": 255},
      "active": {"type": "boolean"},
      "priority": {"type": "number", "minimum": 1, "maximum": 100},
      "rule": {"$ref": "#/components/schemas/NetworkAclRule"}}},
    "NetworkAclRule": {"type": "object", "required": ["action", "scope"], "properties": {
      "action": {"$ref": "#/components/schemas/NetworkAclAction"},
      "scope": {"$ref": "#/components/schemas/NetworkAclRuleScopeEnum"},
      "match": {"$ref": "#/components/schemas/NetworkAclMatch"}}},
    "NetworkAclAction": {"type": "object", "properties": {"block": {"type": "boolean"}, "log": {"type": "boolean"}}},
    "NetworkAclRuleScopeEnum": {"type": "string", "enum": ["management", "authentication", "tenant", "dynamic_client_registration"]},
    "NetworkAclMatch": {"type": "object", "properties": {
      "ipv4_cidrs": {"type": "array", "items": {"$ref": "#/components/schemas/NetworkACLMatchIpv4Cidr"}}}},
    "NetworkACLMatchIpv4Cidr": {"type": "string", "oneOf": [
      {"type": "string", "format": "ipv4"},
      {"type": "string", "format": "cidr"}]}
  }}
}`

// useNetworkACLFixtureSchema points --data validation and --schema at the fixture for the test.
// It swaps a package global, so callers must not use t.Parallel().
func useNetworkACLFixtureSchema(t *testing.T) {
	t.Helper()
	doc, err := openapi.LoadDocFromData([]byte(networkACLFixtureSchemaDoc))
	require.NoError(t, err)

	prev := newSchemaManager
	newSchemaManager = func() (*openapi.SchemaManager, error) {
		return openapi.NewSchemaManagerFromDoc(doc), nil
	}
	t.Cleanup(func() { newSchemaManager = prev })
}

const networkACLResponse = `{"id":"acl_123","description":"Block office","active":false,"priority":10,` +
	`"rule":{"action":{"block":true},"scope":"tenant","match":{"ipv4_cidrs":["192.0.2.0/24","192.0.2.10"]}}}`

// networkACLCIDRData holds a CIDR and a plain address, which the CIDR oneOf rejects locally.
const networkACLCIDRData = `{"description":"Block office","active":false,"priority":10,` +
	`"rule":{"action":{"block":true},"scope":"tenant","match":{"ipv4_cidrs":["192.0.2.0/24","192.0.2.10"]}}}`

var networkACLWriteCases = []struct {
	name     string
	newCmd   func(*cli) *cobra.Command
	idArgs   []string
	method   string
	uri      string
	flagArgs []string
}{
	{
		name:     "create",
		newCmd:   createNetworkACLCmd,
		method:   http.MethodPost,
		uri:      "https://example.test/api/v2/network-acls",
		flagArgs: []string{"--priority", "3"},
	},
	{
		name:     "update",
		newCmd:   updateNetworkACLCmd,
		idArgs:   []string{"acl_123"},
		method:   http.MethodPatch,
		uri:      "https://example.test/api/v2/network-acls/acl_123",
		flagArgs: []string{"--active", "true"},
	},
}

func TestNetworkACLCmdSchema(t *testing.T) {
	useNetworkACLFixtureSchema(t)

	tests := []struct {
		name      string
		newCmd    func(*cli) *cobra.Command
		method    string
		path      string
		schemaKey string
		// Marker sits behind a $ref (or is a query parameter), so it only appears once resolved.
		marker string
	}{
		{"list", listNetworkACLsCmd, http.MethodGet, "/network-acls", "query_params_schema", "include_totals"},
		{"create", createNetworkACLCmd, http.MethodPost, "/network-acls", "request_schema", "dynamic_client_registration"},
		{"update", updateNetworkACLCmd, http.MethodPatch, "/network-acls/{id}", "request_schema", "dynamic_client_registration"},
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

func TestNetworkACLListCmdInvalidQuery(t *testing.T) {
	c := &cli{
		renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: io.Discard},
		api:      &auth0.API{},
	}

	cmd := listNetworkACLsCmd(c)
	cmd.SetArgs([]string{"--query", "not-valid-json"})

	assert.ErrorContains(t, cmd.Execute(), "invalid --query value")
}

// The fixture keeps the live spec's format-only CIDR oneOf, so these cases fail again
// if local validation is turned back on for network ACL writes.
func TestNetworkACLWriteCmdData(t *testing.T) {
	useNetworkACLFixtureSchema(t)

	for _, tc := range networkACLWriteCases {
		t.Run(tc.name+" sends the payload as-is", func(t *testing.T) {
			stub := &jsonWriteStub{response: networkACLResponse}
			cmd := tc.newCmd(newJSONWriteCLI(stub))
			prepareInteractivity(cmd)
			cmd.SetArgs(append(append([]string{}, tc.idArgs...), "--data", networkACLCIDRData))

			require.NoError(t, cmd.Execute())
			assert.Equal(t, tc.method, stub.gotMethod)
			assert.Equal(t, tc.uri, stub.gotURI)
			assert.Equal(t, networkACLCIDRData, stub.gotBody)
		})

		t.Run(tc.name+" sends piped input as-is", func(t *testing.T) {
			stub := &jsonWriteStub{response: networkACLResponse}
			cmd := tc.newCmd(newJSONWriteCLI(stub))
			prepareInteractivity(cmd)
			cmd.SetArgs(append([]string{}, tc.idArgs...))

			withPipedStdin(t, networkACLCIDRData, func() {
				require.NoError(t, cmd.Execute())
			})
			assert.Equal(t, tc.method, stub.gotMethod)
			assert.Equal(t, tc.uri, stub.gotURI)
			assert.Equal(t, networkACLCIDRData, stub.gotBody)
		})
	}
}

func TestNetworkACLWriteCmdDataSkipsSchemaValidation(t *testing.T) {
	useNetworkACLFixtureSchema(t)

	for _, tc := range networkACLWriteCases {
		t.Run(tc.name+" leaves schema errors to the API", func(t *testing.T) {
			stub := &jsonWriteStub{err: errors.New("400 Bad Request: Payload validation error: 'Value 500 is greater than maximum 100' on property priority")}
			cmd := tc.newCmd(newJSONWriteCLI(stub))
			prepareInteractivity(cmd)
			cmd.SetArgs(append(append([]string{}, tc.idArgs...), "--data", `{"priority":500}`))

			err := cmd.Execute()
			require.Error(t, err)
			assert.Equal(t, tc.method, stub.gotMethod, "the payload must reach the API")
			assert.Contains(t, err.Error(), "greater than maximum 100")
			assert.NotContains(t, err.Error(), "Expected Request Schema")
		})

		t.Run(tc.name+" rejects a non-object payload before the API", func(t *testing.T) {
			stub := &jsonWriteStub{}
			cmd := tc.newCmd(newJSONWriteCLI(stub))
			prepareInteractivity(cmd)
			cmd.SetArgs(append(append([]string{}, tc.idArgs...), "--data", `["192.0.2.0/24"]`))

			assert.ErrorContains(t, cmd.Execute(), "must be a JSON object")
			assert.Empty(t, stub.gotMethod, "must not reach the API on a bad payload")
		})
	}
}

func TestNetworkACLDataConflictsWithFlags(t *testing.T) {
	for _, tc := range networkACLWriteCases {
		t.Run(tc.name+" with --data", func(t *testing.T) {
			stub := &jsonWriteStub{}
			cmd := tc.newCmd(newJSONWriteCLI(stub))
			prepareInteractivity(cmd)
			cmd.SetArgs(append(append(append([]string{}, tc.idArgs...), "--data", `{"priority":5}`), tc.flagArgs...))

			assert.ErrorContains(t, cmd.Execute(), "none of the others can be")
			assert.Empty(t, stub.gotMethod, "must not reach the API when flags conflict")
		})

		t.Run(tc.name+" with piped input", func(t *testing.T) {
			stub := &jsonWriteStub{}
			cmd := tc.newCmd(newJSONWriteCLI(stub))
			prepareInteractivity(cmd)
			cmd.SetArgs(append(append([]string{}, tc.idArgs...), tc.flagArgs...))

			withPipedStdin(t, `{"priority":5}`, func() {
				assert.ErrorContains(t, cmd.Execute(), "cannot combine piped JSON input with individual flags")
			})
			assert.Empty(t, stub.gotMethod, "must not reach the API when flags conflict")
		})
	}
}
