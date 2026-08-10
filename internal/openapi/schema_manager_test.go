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
			expectRequestBody: false, // GET has no request body.
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

func TestGetResourceOperations(t *testing.T) {
	manager, err := NewSchemaManager()
	require.NoError(t, err)

	// Test for "actions" resource.
	operations, err := manager.GetResourceOperations("actions")
	require.NoError(t, err)
	assert.NotEmpty(t, operations)

	// Verify we got multiple operations.
	assert.Greater(t, len(operations), 1)

	// Check that we have common operations.
	operationIDs := make([]string, len(operations))
	for i, op := range operations {
		operationIDs[i] = op.OperationID
	}

	assert.Contains(t, operationIDs, "get_actions")
	assert.Contains(t, operationIDs, "post_action")
}

func TestListAllOperations(t *testing.T) {
	manager, err := NewSchemaManager()
	require.NoError(t, err)

	operations := manager.ListAllOperations()
	assert.NotEmpty(t, operations)

	// Should have many operations.
	assert.Greater(t, len(operations), 50)

	// Verify structure.
	for _, op := range operations {
		assert.NotEmpty(t, op.Method)
		assert.NotEmpty(t, op.Path)
		assert.NotEmpty(t, op.OperationID)
		// Summary might be empty for some operations.
	}

	// Check for specific operations.
	found := false
	for _, op := range operations {
		if op.OperationID == "post_action" {
			found = true
			assert.Equal(t, "POST", op.Method)
			assert.Equal(t, "/actions/actions", op.Path)
			break
		}
	}
	assert.True(t, found, "Should find post_action operation")
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
