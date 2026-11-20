package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/openapi"
)

// attackProtectionFixtureSchemaDoc mirrors the $ref layout of the live spec for the
// four attack protection PATCH operations, so validation runs offline and resolves refs.
const attackProtectionFixtureSchemaDoc = `{
  "openapi": "3.0.0",
  "info": {"title": "fixture", "version": "1.0.0"},
  "paths": {
    "/attack-protection/bot-detection": {"patch": {"operationId": "patch_bot-detection", "requestBody": {"content": {"application/json": {"schema": {
      "$ref": "#/components/schemas/UpdateBotDetectionSettingsRequestContent"}}}}}},
    "/attack-protection/breached-password-detection": {"patch": {"operationId": "patch_breached-password-detection", "requestBody": {"content": {"application/json": {"schema": {
      "$ref": "#/components/schemas/UpdateBreachedPasswordDetectionSettingsRequestContent"}}}}}},
    "/attack-protection/brute-force-protection": {"patch": {"operationId": "patch_brute-force-protection", "requestBody": {"content": {"application/json": {"schema": {
      "$ref": "#/components/schemas/UpdateBruteForceSettingsRequestContent"}}}}}},
    "/attack-protection/suspicious-ip-throttling": {"patch": {"operationId": "patch_suspicious-ip-throttling", "requestBody": {"content": {"application/json": {"schema": {
      "$ref": "#/components/schemas/UpdateSuspiciousIPThrottlingSettingsRequestContent"}}}}}}
  },
  "components": {"schemas": {
    "UpdateBotDetectionSettingsRequestContent": {"type": "object", "additionalProperties": false, "properties": {
      "bot_detection_level": {"$ref": "#/components/schemas/BotDetectionLevelEnum"},
      "allowlist": {"type": "array", "items": {"$ref": "#/components/schemas/BotDetectionIPAddressOrCidrBlock"}},
      "monitoring_mode_enabled": {"$ref": "#/components/schemas/BotDetectionMonitoringModeEnabled"}}},
    "BotDetectionLevelEnum": {"type": "string", "enum": ["low", "medium", "high"]},
    "BotDetectionIPAddressOrCidrBlock": {"type": "string", "oneOf": [
      {"$ref": "#/components/schemas/BotDetectionIPv4"},
      {"$ref": "#/components/schemas/BotDetectionIPv6"},
      {"$ref": "#/components/schemas/BotDetectionCidrBlock"},
      {"$ref": "#/components/schemas/BotDetectionIPv6CidrBlock"}]},
    "BotDetectionIPv4": {"type": "string", "format": "ipv4"},
    "BotDetectionIPv6": {"type": "string", "format": "ipv6"},
    "BotDetectionCidrBlock": {"type": "string", "format": "cidr"},
    "BotDetectionIPv6CidrBlock": {"type": "string", "format": "ipv6_cidr"},
    "BotDetectionMonitoringModeEnabled": {"type": "boolean"},

    "UpdateBreachedPasswordDetectionSettingsRequestContent": {"type": "object", "additionalProperties": false, "properties": {
      "enabled": {"type": "boolean"},
      "method": {"$ref": "#/components/schemas/BreachedPasswordDetectionMethodEnum"},
      "stage": {"$ref": "#/components/schemas/BreachedPasswordDetectionStage"}}},
    "BreachedPasswordDetectionMethodEnum": {"type": "string", "enum": ["standard", "enhanced"]},
    "BreachedPasswordDetectionStage": {"type": "object", "properties": {
      "pre-change-password": {"$ref": "#/components/schemas/BreachedPasswordDetectionPreChangePasswordStage"}}},
    "BreachedPasswordDetectionPreChangePasswordStage": {"type": "object", "properties": {
      "shields": {"type": "array", "items": {"$ref": "#/components/schemas/BreachedPasswordDetectionPreChangePasswordShieldsEnum"}}}},
    "BreachedPasswordDetectionPreChangePasswordShieldsEnum": {"type": "string", "enum": ["block", "admin_notification"]},

    "UpdateBruteForceSettingsRequestContent": {"type": "object", "additionalProperties": false, "properties": {
      "enabled": {"type": "boolean"},
      "max_attempts": {"type": "integer"},
      "mode": {"$ref": "#/components/schemas/BruteForceProtectionModeEnum"}}},
    "BruteForceProtectionModeEnum": {"type": "string", "enum": ["count_per_identifier_and_ip", "count_per_identifier"]},

    "UpdateSuspiciousIPThrottlingSettingsRequestContent": {"type": "object", "additionalProperties": false, "properties": {
      "enabled": {"type": "boolean"},
      "stage": {"$ref": "#/components/schemas/SuspiciousIPThrottlingStage"}}},
    "SuspiciousIPThrottlingStage": {"type": "object", "properties": {
      "pre-login": {"$ref": "#/components/schemas/SuspiciousIPThrottlingPreLoginStage"}}},
    "SuspiciousIPThrottlingPreLoginStage": {"type": "object", "properties": {
      "max_attempts": {"type": "integer"},
      "rate": {"type": "integer"}}}
  }}
}`

// useAttackProtectionFixtureSchema points --data validation at the fixture for the test.
// It swaps a package global, so callers must not use t.Parallel().
func useAttackProtectionFixtureSchema(t *testing.T) {
	t.Helper()
	doc, err := openapi.LoadDocFromData([]byte(attackProtectionFixtureSchemaDoc))
	require.NoError(t, err)

	prev := newSchemaManager
	newSchemaManager = func() (*openapi.SchemaManager, error) {
		return openapi.NewSchemaManagerFromDoc(doc), nil
	}
	t.Cleanup(func() { newSchemaManager = prev })
}

var attackProtectionUpdateCases = []struct {
	name     string
	newCmd   func(*cli) *cobra.Command
	path     string
	data     string
	response string
	// Invalid targets a field behind a $ref, so the error proves refs were resolved.
	invalid      string
	invalidField string
	flagArgs     []string
	// SchemaMarker sits behind a $ref, so --schema output only has it once refs resolve.
	schemaMarker string
}{
	{
		name:         "bot-detection",
		newCmd:       updateBotDetectionCmd,
		path:         "/attack-protection/bot-detection",
		data:         `{"bot_detection_level":"medium","monitoring_mode_enabled":false}`,
		response:     `{"bot_detection_level":"medium","challenge_password_policy":"never","challenge_passwordless_policy":"never","challenge_password_reset_policy":"never","allowlist":[],"monitoring_mode_enabled":false}`,
		invalid:      `{"bot_detection_level":"extreme"}`,
		invalidField: "bot_detection_level",
		flagArgs:     []string{"--bot-detection-level", "low"},
		schemaMarker: "medium",
	},
	{
		name:         "breached-password-detection",
		newCmd:       updateBreachedPasswordDetectionCmd,
		path:         "/attack-protection/breached-password-detection",
		data:         `{"method":"enhanced"}`,
		response:     `{"enabled":false,"shields":[],"admin_notification_frequency":[],"method":"enhanced","stage":{"pre-change-password":{"shields":[]},"pre-user-registration":{"shields":[]}}}`,
		invalid:      `{"stage":{"pre-change-password":{"shields":["user_notification"]}}}`,
		invalidField: `stage["pre-change-password"].shields[0]`,
		flagArgs:     []string{"--enabled"},
		schemaMarker: "pre-change-password",
	},
	{
		name:         "brute-force-protection",
		newCmd:       updateBruteForceProtectionCmd,
		path:         "/attack-protection/brute-force-protection",
		data:         `{"max_attempts":7}`,
		response:     `{"enabled":false,"shields":["block","user_notification"],"allowlist":[],"mode":"count_per_identifier_and_ip","max_attempts":7}`,
		invalid:      `{"mode":"per_ip"}`,
		invalidField: "mode",
		flagArgs:     []string{"--max-attempts", "3"},
		schemaMarker: "count_per_identifier",
	},
	{
		name:         "suspicious-ip-throttling",
		newCmd:       updateSuspiciousIPThrottlingCmd,
		path:         "/attack-protection/suspicious-ip-throttling",
		data:         `{"stage":{"pre-login":{"max_attempts":99,"rate":864000}}}`,
		response:     `{"enabled":false,"shields":["admin_notification","block"],"allowlist":[],"stage":{"pre-login":{"max_attempts":99,"rate":864000},"pre-user-registration":{"max_attempts":50,"rate":1200}}}`,
		invalid:      `{"stage":{"pre-login":{"rate":"fast"}}}`,
		invalidField: `stage["pre-login"].rate`,
		flagArgs:     []string{"--enabled"},
		schemaMarker: "pre-login",
	},
}

func TestAttackProtectionUpdateCmdSchema(t *testing.T) {
	useAttackProtectionFixtureSchema(t)

	for _, tc := range attackProtectionUpdateCases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			// No API client: --schema must return before anything touches cli.api.
			c := &cli{renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: &out}}
			cmd := tc.newCmd(c)
			cmd.SetArgs([]string{"--schema", "--json"})

			require.NoError(t, cmd.Execute())

			var schema struct {
				Method        string                 `json:"method"`
				Path          string                 `json:"path"`
				RequestSchema map[string]interface{} `json:"request_schema"`
			}
			require.NoError(t, json.Unmarshal(out.Bytes(), &schema))
			assert.Equal(t, http.MethodPatch, schema.Method)
			assert.Equal(t, tc.path, schema.Path)
			assert.NotEmpty(t, schema.RequestSchema)
			assert.Contains(t, out.String(), tc.schemaMarker)
			assert.NotContains(t, out.String(), "$ref")
			assert.NotContains(t, out.String(), "#/components")
		})
	}
}

func TestAttackProtectionUpdateCmdData(t *testing.T) {
	useAttackProtectionFixtureSchema(t)

	for _, tc := range attackProtectionUpdateCases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &jsonWriteStub{response: tc.response}
			cmd := tc.newCmd(newJSONWriteCLI(stub))
			prepareInteractivity(cmd)
			cmd.SetArgs([]string{"--data", tc.data})

			require.NoError(t, cmd.Execute())
			assert.Equal(t, http.MethodPatch, stub.gotMethod)
			assert.Equal(t, "https://example.test/api/v2"+tc.path, stub.gotURI)
			assert.Equal(t, tc.data, stub.gotBody, "the payload must be sent verbatim")
		})
	}
}

func TestAttackProtectionUpdateCmdPipedData(t *testing.T) {
	useAttackProtectionFixtureSchema(t)

	for _, tc := range attackProtectionUpdateCases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &jsonWriteStub{response: tc.response}
			cmd := tc.newCmd(newJSONWriteCLI(stub))
			prepareInteractivity(cmd)
			cmd.SetArgs([]string{})

			withPipedStdin(t, tc.data, func() {
				require.NoError(t, cmd.Execute())
			})
			assert.Equal(t, http.MethodPatch, stub.gotMethod)
			assert.Equal(t, "https://example.test/api/v2"+tc.path, stub.gotURI)
			assert.Equal(t, tc.data, stub.gotBody)
		})
	}
}

func TestAttackProtectionUpdateCmdDataRejectsInvalidSchema(t *testing.T) {
	useAttackProtectionFixtureSchema(t)

	for _, tc := range attackProtectionUpdateCases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &jsonWriteStub{}
			cmd := tc.newCmd(newJSONWriteCLI(stub))
			prepareInteractivity(cmd)
			cmd.SetArgs([]string{"--data", tc.invalid})

			err := cmd.Execute()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "schema validation failed")
			assert.Contains(t, err.Error(), tc.invalidField)
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

// The fixture keeps the live spec's allowlist oneOf, whose branches differ only by
// IP and CIDR format, so these cases fail if the format validators are removed.
func TestBotDetectionUpdateCmdDataValidatesAllowlist(t *testing.T) {
	useAttackProtectionFixtureSchema(t)

	t.Run("sends an allowlist of IPs and CIDRs as-is", func(t *testing.T) {
		data := `{"allowlist":["198.51.100.42","10.0.0.0/24","2001:db8::1","2001:db8::/32"]}`
		stub := &jsonWriteStub{response: data}
		cmd := updateBotDetectionCmd(newJSONWriteCLI(stub))
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", data})

		require.NoError(t, cmd.Execute())
		assert.Equal(t, http.MethodPatch, stub.gotMethod)
		assert.Equal(t, data, stub.gotBody)
	})

	t.Run("rejects an invalid entry before the API", func(t *testing.T) {
		stub := &jsonWriteStub{}
		cmd := updateBotDetectionCmd(newJSONWriteCLI(stub))
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"allowlist":["198.51.100.42","198.51.100.0/33"]}`})

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schema validation failed")
		assert.Contains(t, err.Error(), "allowlist[1]")
		assert.NotContains(t, err.Error(), "allowlist[0]")
		assert.NotContains(t, err.Error(), "more than one schema")
		assert.Empty(t, stub.gotMethod, "must not reach the API when validation fails")
	})
}

func TestAttackProtectionDataConflictsWithFlags(t *testing.T) {
	for _, tc := range attackProtectionUpdateCases {
		t.Run(tc.name+" with --data", func(t *testing.T) {
			stub := &jsonWriteStub{}
			cmd := tc.newCmd(newJSONWriteCLI(stub))
			prepareInteractivity(cmd)
			cmd.SetArgs(append([]string{"--data", tc.data}, tc.flagArgs...))

			assert.ErrorContains(t, cmd.Execute(), "none of the others can be")
			assert.Empty(t, stub.gotMethod, "must not reach the API when flags conflict")
		})

		t.Run(tc.name+" with piped input", func(t *testing.T) {
			stub := &jsonWriteStub{}
			cmd := tc.newCmd(newJSONWriteCLI(stub))
			prepareInteractivity(cmd)
			cmd.SetArgs(tc.flagArgs)

			withPipedStdin(t, tc.data, func() {
				assert.ErrorContains(t, cmd.Execute(), "cannot combine piped JSON input with individual flags")
			})
			assert.Empty(t, stub.gotMethod, "must not reach the API when flags conflict")
		})
	}
}
