package openapi

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSchemaManager(t *testing.T) {
	manager, err := NewSchemaManager()
	require.NoError(t, err)
	require.NotNil(t, manager)
	require.NotNil(t, manager.doc)
}

func TestGetOperationSchema(t *testing.T) {
	manager, err := NewSchemaManager()
	require.NoError(t, err)

	tests := []struct {
		name              string
		method            string
		path              string
		expectError       bool
		expectRequestBody bool
	}{
		{
			name:              "POST /actions/actions",
			method:            "POST",
			path:              "/actions/actions",
			expectError:       false,
			expectRequestBody: true,
		},
		{
			name:              "GET /actions/actions",
			method:            "GET",
			path:              "/actions/actions",
			expectError:       false,
			expectRequestBody: true, // Synthesized from query params.
		},
		{
			name:              "PATCH /actions/actions/{id}",
			method:            "PATCH",
			path:              "/actions/actions/{id}",
			expectError:       false,
			expectRequestBody: true,
		},
		{
			name:        "Invalid path",
			method:      "GET",
			path:        "/invalid/path",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opSchema, err := manager.GetOperationSchema(tt.method, tt.path)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, opSchema)
			} else {
				require.NoError(t, err)
				require.NotNil(t, opSchema)

				assert.Equal(t, tt.method, opSchema.Method)
				assert.Equal(t, tt.path, opSchema.Path)
				assert.NotEmpty(t, opSchema.OperationID)
				assert.NotEmpty(t, opSchema.Summary)

				if tt.expectRequestBody {
					assert.NotNil(t, opSchema.RequestSchema)
				}
			}
		})
	}
}

func TestGetOperationSchema_IsQueryParamSchema(t *testing.T) {
	manager, err := NewSchemaManager()
	require.NoError(t, err)

	t.Run("GET sets IsQueryParamSchema and uses query_params_schema key", func(t *testing.T) {
		opSchema, err := manager.GetOperationSchema("GET", "/actions/actions")
		require.NoError(t, err)

		assert.True(t, opSchema.IsQueryParamSchema)
		assert.NotNil(t, opSchema.RequestSchema)
	})

	t.Run("POST does not set IsQueryParamSchema", func(t *testing.T) {
		opSchema, err := manager.GetOperationSchema("POST", "/actions/actions")
		require.NoError(t, err)

		assert.False(t, opSchema.IsQueryParamSchema)
		assert.NotNil(t, opSchema.RequestSchema)
	})
}

func TestFormatAsJSON_QueryParams(t *testing.T) {
	manager, err := NewSchemaManager()
	require.NoError(t, err)

	opSchema, err := manager.GetOperationSchema("GET", "/actions/actions")
	require.NoError(t, err)

	jsonOutput, err := opSchema.FormatAsJSON()
	require.NoError(t, err)

	assert.Contains(t, jsonOutput, "query_params_schema")
	assert.NotContains(t, jsonOutput, "request_schema")
	assert.Contains(t, jsonOutput, "operation_id")
	assert.Contains(t, jsonOutput, "summary")
}

func TestFormatAsText_QueryParams(t *testing.T) {
	manager, err := NewSchemaManager()
	require.NoError(t, err)

	opSchema, err := manager.GetOperationSchema("GET", "/actions/actions")
	require.NoError(t, err)

	textOutput := opSchema.FormatAsText()

	assert.Contains(t, textOutput, "Query Parameters:")
	assert.NotContains(t, textOutput, "Request Payload:")
	assert.NotContains(t, textOutput, "No request body required")
}

func TestFormatAsJSON(t *testing.T) {
	manager, err := NewSchemaManager()
	require.NoError(t, err)

	opSchema, err := manager.GetOperationSchema("POST", "/actions/actions")
	require.NoError(t, err)

	jsonOutput, err := opSchema.FormatAsJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, jsonOutput)

	// Verify it's valid JSON.
	assert.Contains(t, jsonOutput, "operation_id")
	assert.Contains(t, jsonOutput, "summary")
	assert.Contains(t, jsonOutput, "request_schema")
}

func TestFormatAsText(t *testing.T) {
	manager, err := NewSchemaManager()
	require.NoError(t, err)

	opSchema, err := manager.GetOperationSchema("POST", "/actions/actions")
	require.NoError(t, err)

	textOutput := opSchema.FormatAsText()
	assert.NotEmpty(t, textOutput)

	// Verify it contains expected sections.
	assert.Contains(t, textOutput, "Operation:")
	assert.Contains(t, textOutput, "Endpoint:")
	assert.Contains(t, textOutput, "Request Payload:")
	assert.Contains(t, textOutput, "Required fields:")
	assert.Contains(t, textOutput, "name")
	assert.Contains(t, textOutput, "supported_triggers")
}

func TestValidateRequest(t *testing.T) {
	manager, err := NewSchemaManager()
	require.NoError(t, err)

	tests := []struct {
		name        string
		method      string
		path        string
		body        string
		expectValid bool
	}{
		{
			name:   "Valid action creation",
			method: "POST",
			path:   "/actions/actions",
			body: `{
				"name": "my-action",
				"supported_triggers": [{"id": "post-login", "version": "v3"}],
				"code": "module.exports = () => {}"
			}`,
			expectValid: true,
		},
		{
			name:   "Missing required field",
			method: "POST",
			path:   "/actions/actions",
			body: `{
				"code": "module.exports = () => {}"
			}`,
			expectValid: false,
		},
		{
			name:        "Invalid JSON",
			method:      "POST",
			path:        "/actions/actions",
			body:        `{invalid`,
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := manager.ValidateRequest(tt.method, tt.path, []byte(tt.body))
			require.NoError(t, err)
			require.NotNil(t, result)

			assert.Equal(t, tt.expectValid, result.Valid)

			if !tt.expectValid {
				assert.NotEmpty(t, result.Errors)
			}
		})
	}
}

func TestValidateRequestActionUpdatePath(t *testing.T) {
	manager, err := NewSchemaManager()
	require.NoError(t, err)

	body := []byte(`{"name": "my-action", "runtime": "node22"}`)

	// Templated path: the operation resolves and the body validates.
	result, err := manager.ValidateRequest("PATCH", "/actions/actions/{id}", body)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Valid, "templated path should validate; errors: %v", result.Errors)

	// Concrete-ID path: the operation cannot be found, so ValidateRequest errors.
	// This is exactly the trap that broke `auth0 actions update --data`.
	_, err = manager.ValidateRequest("PATCH", "/actions/actions/act_123", body)
	assert.Error(t, err, "concrete-ID path must not resolve against the templated schema")
}

func TestValidateRequestErrorsAreResolved(t *testing.T) {
	manager, err := NewSchemaManager()
	require.NoError(t, err)

	tests := []struct {
		name         string
		body         string
		wantContains string
	}{
		{
			name:         "Wrong type on a $ref array field",
			body:         `{"name": "x", "supported_triggers": "not-an-array"}`,
			wantContains: `supported_triggers`,
		},
		{
			name:         "Missing required field",
			body:         `{"name": "x", "code": "module.exports = () => {}"}`,
			wantContains: "supported_triggers",
		},
		{
			name:         "Bad enum inside a nested $ref item — JSONPath location",
			body:         `{"name": "x", "supported_triggers": [{"id": "not-a-trigger", "version": "v3"}]}`,
			wantContains: `supported_triggers[0].id`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := manager.ValidateRequest("POST", "/actions/actions", []byte(tt.body))
			require.NoError(t, err)
			require.False(t, result.Valid)
			require.NotEmpty(t, result.Errors)

			joined := strings.Join(result.Errors, "\n")
			// The raw kin-openapi error dumps the schema with unresolved
			// "$ref" entries; our formatter must never surface those.
			assert.NotContains(t, joined, "$ref")
			assert.NotContains(t, joined, "#/components/schemas")
			assert.Contains(t, joined, tt.wantContains)
		})
	}
}

func TestJSONPath(t *testing.T) {
	tests := []struct {
		name     string
		segments []string
		want     string
	}{
		{name: "root", segments: nil, want: "payload"},
		{name: "single key", segments: []string{"name"}, want: "name"},
		{name: "nested keys", segments: []string{"config", "url"}, want: "config.url"},
		{name: "array index", segments: []string{"supported_triggers", "0", "id"}, want: "supported_triggers[0].id"},
		{name: "index then object", segments: []string{"items", "2", "meta", "key"}, want: "items[2].meta.key"},
		{name: "non-identifier key", segments: []string{"a.b"}, want: `["a.b"]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, jsonPath(tt.segments))
		})
	}
}

func TestValidateRequestReportsAllErrors(t *testing.T) {
	manager, err := NewSchemaManager()
	require.NoError(t, err)

	// An empty body is missing both required fields; validation must report
	// all of them, not stop at the first.
	result, err := manager.ValidateRequest("POST", "/actions/actions", []byte(`{}`))
	require.NoError(t, err)
	require.False(t, result.Valid)

	assert.GreaterOrEqual(t, len(result.Errors), 2)
	joined := strings.Join(result.Errors, "\n")
	assert.Contains(t, joined, "name")
	assert.Contains(t, joined, "supported_triggers")
}

func TestSchemaToMap(t *testing.T) {
	manager, err := NewSchemaManager()
	require.NoError(t, err)

	opSchema, err := manager.GetOperationSchema("POST", "/actions/actions")
	require.NoError(t, err)

	schemaMap := schemaToMap(opSchema.RequestSchema)
	assert.NotEmpty(t, schemaMap)

	// Should have required fields.
	required, ok := schemaMap["required"].([]string)
	assert.True(t, ok)
	assert.Contains(t, required, "name")
	assert.Contains(t, required, "supported_triggers")

	// Should have properties.
	props, ok := schemaMap["properties"].(map[string]interface{})
	assert.True(t, ok)
	assert.NotEmpty(t, props)

	// Check a specific property.
	nameProp, ok := props["name"].(map[string]interface{})
	assert.True(t, ok)
	assert.NotNil(t, nameProp["type"])
	assert.NotNil(t, nameProp["description"])
}

func TestFormatSchema(t *testing.T) {
	manager, err := NewSchemaManager()
	require.NoError(t, err)

	opSchema, err := manager.GetOperationSchema("POST", "/actions/actions")
	require.NoError(t, err)

	formatted := formatSchema(opSchema.RequestSchema, "")
	assert.NotEmpty(t, formatted)

	// Should contain required and optional sections.
	assert.Contains(t, formatted, "Required fields:")
	assert.Contains(t, formatted, "Optional fields:")

	// Should contain field names.
	assert.Contains(t, formatted, "name")
	assert.Contains(t, formatted, "supported_triggers")
	assert.Contains(t, formatted, "code")
}

func TestFormatField(t *testing.T) {
	manager, err := NewSchemaManager()
	require.NoError(t, err)

	opSchema, err := manager.GetOperationSchema("POST", "/actions/actions")
	require.NoError(t, err)

	nameField := opSchema.RequestSchema.Properties["name"]
	require.NotNil(t, nameField)
	require.NotNil(t, nameField.Value)

	formatted := formatField("name", nameField.Value, "  ")
	assert.NotEmpty(t, formatted)

	// Should contain field name and type.
	assert.Contains(t, formatted, "name")
	assert.Contains(t, formatted, "string")
	assert.Contains(t, formatted, "The name of an action")
}
