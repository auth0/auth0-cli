package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/openapi"
)

// tenantSettingsFixtureSchemaDoc mirrors the $ref layout of the live spec for
// PATCH /tenants/settings, so --data validation and --schema run offline.
const tenantSettingsFixtureSchemaDoc = `{
  "openapi": "3.0.0",
  "info": {"title": "fixture", "version": "1.0.0"},
  "paths": {
    "/tenants/settings": {"patch": {"operationId": "patch_settings", "requestBody": {"content": {"application/json": {"schema": {
      "$ref": "#/components/schemas/UpdateTenantSettingsRequestContent"}}}}}}
  },
  "components": {"schemas": {
    "UpdateTenantSettingsRequestContent": {"type": "object", "additionalProperties": false, "properties": {
      "friendly_name": {"type": "string"},
      "support_email": {"type": "string"},
      "idle_session_lifetime": {"type": "integer", "minimum": 1},
      "flags": {"$ref": "#/components/schemas/TenantSettingsFlags"}}},
    "TenantSettingsFlags": {"type": "object", "properties": {
      "enable_sso": {"type": "boolean"}}}
  }}
}`

// useTenantSettingsFixtureSchema points --data validation and --schema at the fixture
// for the test. It swaps a package global, so callers must not use t.Parallel().
func useTenantSettingsFixtureSchema(t *testing.T) {
	t.Helper()
	doc, err := openapi.LoadDocFromData([]byte(tenantSettingsFixtureSchemaDoc))
	require.NoError(t, err)

	prev := newSchemaManager
	newSchemaManager = func() (*openapi.SchemaManager, error) {
		return openapi.NewSchemaManagerFromDoc(doc), nil
	}
	t.Cleanup(func() { newSchemaManager = prev })
}

const (
	tenantSettingsData     = `{"friendly_name":"Acme","flags":{"enable_sso":true}}`
	tenantSettingsResponse = `{"friendly_name":"Acme","flags":{"enable_sso":true}}`
	tenantSettingsURI      = "https://example.test/api/v2/tenants/settings"
)

func TestTenantSettingsUpdateSetCmdSchema(t *testing.T) {
	useTenantSettingsFixtureSchema(t)

	var out bytes.Buffer
	// No API client: --schema must return before anything touches cli.api.
	c := &cli{renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: &out}}
	cmd := set(c)
	cmd.SetArgs([]string{"--schema", "--json"})

	require.NoError(t, cmd.Execute())

	var schema map[string]interface{}
	require.NoError(t, json.Unmarshal(out.Bytes(), &schema))
	assert.Equal(t, http.MethodPatch, schema["method"])
	assert.Equal(t, "/tenants/settings", schema["path"])
	assert.NotEmpty(t, schema["request_schema"])
	// The enable_sso field sits behind two $refs, so it only appears once they are resolved.
	assert.Contains(t, out.String(), "enable_sso")
	assert.NotContains(t, out.String(), "$ref")
	assert.NotContains(t, out.String(), "#/components")
}

func TestTenantSettingsUpdateSetCmdData(t *testing.T) {
	useTenantSettingsFixtureSchema(t)

	t.Run("sends --data verbatim to PATCH /tenants/settings", func(t *testing.T) {
		stub := &jsonWriteStub{response: tenantSettingsResponse}
		cmd := set(newJSONWriteCLI(stub))
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", tenantSettingsData})

		require.NoError(t, cmd.Execute())
		assert.Equal(t, http.MethodPatch, stub.gotMethod)
		assert.Equal(t, tenantSettingsURI, stub.gotURI)
		assert.Equal(t, tenantSettingsData, stub.gotBody)
	})

	t.Run("sends piped input verbatim", func(t *testing.T) {
		stub := &jsonWriteStub{response: tenantSettingsResponse}
		cmd := set(newJSONWriteCLI(stub))
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{})

		withPipedStdin(t, tenantSettingsData, func() {
			require.NoError(t, cmd.Execute())
		})
		assert.Equal(t, http.MethodPatch, stub.gotMethod)
		assert.Equal(t, tenantSettingsURI, stub.gotURI)
		assert.Equal(t, tenantSettingsData, stub.gotBody)
	})

	t.Run("rejects a schema failure before calling the API", func(t *testing.T) {
		stub := &jsonWriteStub{}
		cmd := set(newJSONWriteCLI(stub))
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"friendly_name":123,"flags":{"enable_sso":"yes"},"not_a_field":true}`})

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schema validation failed")
		for _, field := range []string{"friendly_name", "flags.enable_sso", `property "not_a_field" is unsupported`} {
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

	t.Run("rejects a non-object payload before calling the API", func(t *testing.T) {
		stub := &jsonWriteStub{}
		cmd := set(newJSONWriteCLI(stub))
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `["friendly_name"]`})

		assert.ErrorContains(t, cmd.Execute(), "value must be an object")
		assert.Empty(t, stub.gotMethod, "must not reach the API on a bad payload")
	})
}

func TestTenantSettingsUpdateSetCmdSettingNames(t *testing.T) {
	// The helper wires the SDK tenant mock (flag path) and the raw stub (JSON path), so a
	// test can tell which path ran; a mock without expectations fails if it is called.
	newSetCLI := func(t *testing.T, tenantAPI *mock.MockTenantAPI) (*cli, *jsonWriteStub) {
		t.Helper()
		stub := &jsonWriteStub{}
		return &cli{
			api:      &auth0.API{Tenant: tenantAPI, HTTPClient: stub},
			renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: io.Discard},
		}, stub
	}

	for name, pipe := range map[string]bool{"--data": false, "piped input": true} {
		t.Run("setting names cannot be combined with "+name, func(t *testing.T) {
			c, stub := newSetCLI(t, mock.NewMockTenantAPI(gomock.NewController(t)))
			cmd := set(c)
			prepareInteractivity(cmd)

			var err error
			if pipe {
				cmd.SetArgs([]string{"flags.enable_sso"})
				withPipedStdin(t, tenantSettingsData, func() { err = cmd.Execute() })
			} else {
				cmd.SetArgs([]string{"flags.enable_sso", "--data", tenantSettingsData})
				err = cmd.Execute()
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), "cannot combine setting names with a JSON payload")
			var uErr usageError
			require.ErrorAs(t, err, &uErr)
			assert.Equal(t, "incompatible_flags", uErr.reason)
			assert.Empty(t, stub.gotMethod, "must not reach the API on conflicting input")
		})
	}

	t.Run("setting names keep the flag path when nothing is piped", func(t *testing.T) {
		tenantAPI := mock.NewMockTenantAPI(gomock.NewController(t))
		var sent *management.Tenant
		tenantAPI.EXPECT().Update(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, tenant *management.Tenant, _ ...management.RequestOption) error {
				sent = tenant
				return nil
			})

		c, stub := newSetCLI(t, tenantAPI)
		cmd := set(c)
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"flags.enable_sso"})

		require.NoError(t, cmd.Execute())
		require.NotNil(t, sent)
		assert.True(t, sent.GetFlags().GetEnableSSO())
		assert.Empty(t, stub.gotMethod, "the JSON write path must not run")
	})
}

func TestTenantSettingsUpdateCmdPointsToSet(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantHint bool
		wantErr  string
	}{
		{name: "--data with a value", args: []string{"--data", "{}"}, wantHint: true},
		{name: "--data=value", args: []string{"--data={}"}, wantHint: true},
		{name: "--schema", args: []string{"--schema"}, wantHint: true},
		{name: "another unknown flag", args: []string{"--bogus"}, wantErr: "unknown flag: --bogus"},
		{name: "a mistyped subcommand stays an unknown command", args: []string{"foo", "--data", "{}"}, wantErr: `unknown command "foo"`},
		{name: "unset keeps its own unknown flag error", args: []string{"unset", "--data", "{}"}, wantErr: "unknown flag: --data"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := update(&cli{renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: io.Discard}})
			cmd.SetArgs(tc.args)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)

			err := cmd.Execute()
			require.Error(t, err)

			if tc.wantHint {
				assert.Equal(t, tenantSettingsJSONHint, err.Error())
				var uErr usageError
				assert.True(t, errors.As(err, &uErr), "the hint must stay a usage error")
				return
			}

			assert.NotEqual(t, tenantSettingsJSONHint, err.Error())
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
