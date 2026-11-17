package openapi

import (
	"encoding/json"
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

// arrayBodyFixtureDoc models an operation whose request body is a top-level array
// of objects, like PATCH /connections/{id}/clients (an array of {client_id, status}).
const arrayBodyFixtureDoc = `{
  "openapi": "3.0.0",
  "info": {"title": "fixture", "version": "1.0.0"},
  "paths": {
    "/connections/{id}/clients": {
      "patch": {
        "operationId": "patch_enabled_clients",
        "summary": "Update enabled clients",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "type": "array",
                "minItems": 1,
                "items": {
                  "type": "object",
                  "required": ["client_id", "status"],
                  "properties": {
                    "client_id": {"type": "string"},
                    "status": {"type": "boolean"}
                  }
                }
              }
            }
          }
        }
      }
    }
  }
}`

func TestFormatAsText_TopLevelArrayBody(t *testing.T) {
	doc, err := LoadDocFromData([]byte(arrayBodyFixtureDoc))
	require.NoError(t, err)

	opSchema, err := NewSchemaManagerFromDoc(doc).GetOperationSchema("PATCH", "/connections/{id}/clients")
	require.NoError(t, err)

	textOutput := opSchema.FormatAsText()

	// A top-level array body must descend into its item schema so the fields
	// surface, rather than stopping at a bare "Type: array".
	assert.Contains(t, textOutput, "Request Payload:")
	assert.Contains(t, textOutput, "Type: array")
	assert.Contains(t, textOutput, "client_id")
	assert.Contains(t, textOutput, "status")
}

func TestFormatAsJSON_TopLevelArrayBody(t *testing.T) {
	doc, err := LoadDocFromData([]byte(arrayBodyFixtureDoc))
	require.NoError(t, err)

	opSchema, err := NewSchemaManagerFromDoc(doc).GetOperationSchema("PATCH", "/connections/{id}/clients")
	require.NoError(t, err)

	jsonOutput, err := opSchema.FormatAsJSON()
	require.NoError(t, err)

	assert.Contains(t, jsonOutput, "request_schema")
	assert.Contains(t, jsonOutput, "client_id")
	assert.Contains(t, jsonOutput, "status")
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

			if tt.expectValid {
				assert.Equal(t, StatusValid, result.Status)
			} else {
				assert.Equal(t, StatusInvalid, result.Status)
				assert.NotEmpty(t, result.Errors)
			}
		})
	}
}

func TestValidateRequestReportsStatus(t *testing.T) {
	manager, err := NewSchemaManager()
	require.NoError(t, err)

	// An operation with a request schema: the payload is actually checked.
	result, err := manager.ValidateRequest("POST", "/actions/actions", []byte(`{
		"name": "my-action",
		"supported_triggers": [{"id": "post-login", "version": "v3"}],
		"code": "module.exports = () => {}"
	}`))
	require.NoError(t, err)
	assert.Equal(t, StatusValid, result.Status, "an operation with a request schema whose payload passes is StatusValid")

	// An operation that resolves but defines no request body schema: reported
	// as StatusNoSchema (nothing to check against, sent as-is).
	result, err = manager.ValidateRequest("DELETE", "/actions/actions/{id}", []byte(`{}`))
	require.NoError(t, err)
	assert.Equal(t, StatusNoSchema, result.Status, "an operation without a request schema is StatusNoSchema")
}

// TestValidateRequestChecksJSONWithoutSchema pins the fix for the case where a
// schema-less operation skipped the JSON syntax check: malformed --data must fail
// locally as StatusInvalid rather than being shipped to the API as opaque bytes.
func TestValidateRequestChecksJSONWithoutSchema(t *testing.T) {
	const doc = `{
		"openapi": "3.0.0",
		"info": {"title": "fixture", "version": "1.0.0"},
		"paths": {"/deploy": {"post": {"operationId": "post_deploy"}}}
	}`
	parsed, err := LoadDocFromData([]byte(doc))
	require.NoError(t, err)
	manager := NewSchemaManagerFromDoc(parsed)

	// Well-formed JSON against a schema-less op: nothing to check, sent as-is.
	result, err := manager.ValidateRequest("POST", "/deploy", []byte(`{"anything": true}`))
	require.NoError(t, err)
	assert.Equal(t, StatusNoSchema, result.Status)

	// Malformed JSON against the same schema-less op: caught locally.
	result, err = manager.ValidateRequest("POST", "/deploy", []byte(`{"broken":`))
	require.NoError(t, err)
	assert.Equal(t, StatusInvalid, result.Status, "malformed JSON must fail even without a schema")
	require.NotEmpty(t, result.Errors)
	assert.Contains(t, result.Errors[0], "Invalid JSON")
}

func TestValidateRequestActionUpdatePath(t *testing.T) {
	manager, err := NewSchemaManager()
	require.NoError(t, err)

	body := []byte(`{"name": "my-action", "runtime": "node22"}`)

	// Templated path: the operation resolves and the body validates.
	result, err := manager.ValidateRequest("PATCH", "/actions/actions/{id}", body)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, StatusValid, result.Status, "templated path should validate; errors: %v", result.Errors)

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
			require.Equal(t, StatusInvalid, result.Status)
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

func TestValidateRequestPopulatesStructuredFieldErrors(t *testing.T) {
	manager, err := NewSchemaManager()
	require.NoError(t, err)

	t.Run("a schema violation carries a JSONPath field and a reason", func(t *testing.T) {
		result, err := manager.ValidateRequest(
			"POST", "/actions/actions",
			[]byte(`{"name": "x", "supported_triggers": [{"id": "not-a-trigger", "version": "v3"}]}`),
		)
		require.NoError(t, err)
		require.Equal(t, StatusInvalid, result.Status)
		require.NotEmpty(t, result.FieldErrors)

		// FieldErrors and the human-readable Errors describe the same failures.
		require.Len(t, result.Errors, len(result.FieldErrors))
		for i, fe := range result.FieldErrors {
			assert.Equal(t, fe.String(), result.Errors[i])
		}

		var found bool
		for _, fe := range result.FieldErrors {
			if fe.Field == "supported_triggers[0].id" {
				found = true
				assert.NotEmpty(t, fe.Reason)
				assert.NotContains(t, fe.Reason, "$ref")
			}
		}
		assert.True(t, found, "expected a field error located at supported_triggers[0].id")
	})

	t.Run("malformed JSON yields a field error with no location", func(t *testing.T) {
		result, err := manager.ValidateRequest("POST", "/actions/actions", []byte(`{`))
		require.NoError(t, err)
		require.Equal(t, StatusInvalid, result.Status)
		require.Len(t, result.FieldErrors, 1)
		assert.Empty(t, result.FieldErrors[0].Field)
		assert.Contains(t, result.FieldErrors[0].Reason, "Invalid JSON")
	})
}

func TestFieldErrorString(t *testing.T) {
	assert.Equal(t, "name: is required", FieldError{Field: "name", Reason: "is required"}.String())
	assert.Equal(t, "Invalid JSON: x", FieldError{Reason: "Invalid JSON: x"}.String())
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
	require.Equal(t, StatusInvalid, result.Status)

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

// compositionFixtureDoc covers the shapes behind POST /event-streams (top-level
// oneOf), PATCH /event-streams/{id} (oneOf field) and flows (a recursive oneOf).
const compositionFixtureDoc = `{
  "openapi": "3.0.0",
  "info": {"title": "fixture", "version": "1.0.0"},
  "paths": {
    "/pets": {"post": {"operationId": "post_pet", "requestBody": {"content": {"application/json": {"schema": {
      "oneOf": [{"$ref": "#/components/schemas/Cat"}, {"$ref": "#/components/schemas/Dog"}]}}}}}},
    "/owners/{id}": {"patch": {"operationId": "patch_owner", "requestBody": {"content": {"application/json": {"schema": {
      "type": "object",
      "properties": {
        "pet": {"oneOf": [{"$ref": "#/components/schemas/Cat"}, {"$ref": "#/components/schemas/Dog"}]},
        "pets": {"type": "array", "items": {"oneOf": [{"$ref": "#/components/schemas/Cat"}, {"$ref": "#/components/schemas/Dog"}]}}
      }}}}}}},
    "/nodes": {"post": {"operationId": "post_node", "requestBody": {"content": {"application/json": {"schema": {
      "$ref": "#/components/schemas/Node"}}}}}},
    "/mixed": {"post": {"operationId": "post_mixed", "requestBody": {"content": {"application/json": {"schema": {
      "type": "object",
      "properties": {
        "merged": {"allOf": [{"$ref": "#/components/schemas/Cat"}, {"type": "object", "properties": {"age": {"type": "integer"}}}]},
        "either": {"anyOf": [{"$ref": "#/components/schemas/Cat"}, {"$ref": "#/components/schemas/Dog"}]},
        "tagged": {"type": "array", "items": {"type": "object", "properties": {"tag": {"type": "string"}},
          "oneOf": [{"$ref": "#/components/schemas/Cat"}, {"$ref": "#/components/schemas/Dog"}]}}
      }}}}}}}
  },
  "components": {"schemas": {
    "Cat": {"type": "object", "required": ["name"], "properties": {"name": {"type": "string"}, "meows": {"type": "boolean"}}},
    "Dog": {"type": "object", "required": ["name"], "properties": {"name": {"type": "string"}, "barks": {"type": "boolean"}}},
    "Node": {"type": "object", "properties": {
      "label": {"type": "string"},
      "next": {"oneOf": [{"$ref": "#/components/schemas/Node"}, {"type": "string"}]}}}
  }}
}`

func compositionSchema(t *testing.T, method, path string) *OperationSchema {
	t.Helper()
	doc, err := LoadDocFromData([]byte(compositionFixtureDoc))
	require.NoError(t, err)
	opSchema, err := NewSchemaManagerFromDoc(doc).GetOperationSchema(method, path)
	require.NoError(t, err)
	return opSchema
}

func TestFormatAsJSON_Composition(t *testing.T) {
	variants := func(t *testing.T, node interface{}) []map[string]interface{} {
		t.Helper()
		raw, ok := node.(map[string]interface{})["oneOf"].([]interface{})
		require.True(t, ok, "missing oneOf")
		out := make([]map[string]interface{}, len(raw))
		for i, v := range raw {
			out[i] = v.(map[string]interface{})
		}
		return out
	}
	decode := func(t *testing.T, opSchema *OperationSchema) map[string]interface{} {
		t.Helper()
		jsonOutput, err := opSchema.FormatAsJSON()
		require.NoError(t, err)
		var out map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(jsonOutput), &out))
		return out["request_schema"].(map[string]interface{})
	}

	t.Run("top-level oneOf lists each variant with its fields", func(t *testing.T) {
		pets := variants(t, decode(t, compositionSchema(t, "POST", "/pets")))
		cat, dog := pets[0], pets[1]
		assert.Equal(t, "Cat", cat["title"])
		assert.Equal(t, []interface{}{"name"}, cat["required"])
		assert.Contains(t, cat["properties"], "meows")
		assert.Equal(t, "Dog", dog["title"])
		assert.Contains(t, dog["properties"], "barks")
	})

	t.Run("oneOf inside a field and inside array items", func(t *testing.T) {
		props := decode(t, compositionSchema(t, "PATCH", "/owners/{id}"))["properties"].(map[string]interface{})
		assert.Equal(t, "Cat", variants(t, props["pet"])[0]["title"])
		assert.Equal(t, "Dog", variants(t, props["pets"].(map[string]interface{})["items"])[1]["title"])
	})

	t.Run("a recursive schema terminates", func(t *testing.T) {
		props := decode(t, compositionSchema(t, "POST", "/nodes"))["properties"].(map[string]interface{})
		next := variants(t, props["next"])
		assert.Equal(t, recursiveSchemaNote, next[0]["description"])
		assert.Equal(t, []interface{}{"string"}, next[1]["type"])
	})
}

func TestFormatAsText_Composition(t *testing.T) {
	t.Run("top-level oneOf", func(t *testing.T) {
		text := compositionSchema(t, "POST", "/pets").FormatAsText()
		assert.Contains(t, text, "One of:")
		assert.Contains(t, text, "Variant 1: Cat")
		assert.Contains(t, text, "- meows (boolean)")
		assert.Contains(t, text, "Variant 2: Dog")
		assert.Contains(t, text, "- barks (boolean)")
	})

	t.Run("oneOf inside a field and inside array items", func(t *testing.T) {
		text := compositionSchema(t, "PATCH", "/owners/{id}").FormatAsText()
		assert.Contains(t, text, "- pet\n")
		assert.Contains(t, text, "Each item:")
		assert.Equal(t, 2, strings.Count(text, "Variant 2: Dog"), "both pet and pets list their variants")
	})

	t.Run("a recursive schema terminates", func(t *testing.T) {
		assert.Contains(t, compositionSchema(t, "POST", "/nodes").FormatAsText(), recursiveSchemaNote)
	})
}

func TestFormatComposition_AnyOfAllOf(t *testing.T) {
	opSchema := compositionSchema(t, "POST", "/mixed")

	jsonOutput, err := opSchema.FormatAsJSON()
	require.NoError(t, err)
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(jsonOutput), &out))
	props := out["request_schema"].(map[string]interface{})["properties"].(map[string]interface{})
	branch := func(field, keyword string, i int) map[string]interface{} {
		return props[field].(map[string]interface{})[keyword].([]interface{})[i].(map[string]interface{})
	}
	assert.Equal(t, "Cat", branch("merged", "allOf", 0)["title"])
	assert.Contains(t, branch("merged", "allOf", 1)["properties"], "age")
	assert.Equal(t, "Dog", branch("either", "anyOf", 1)["title"])

	text := opSchema.FormatAsText()
	assert.Contains(t, text, "All of:")
	assert.Contains(t, text, "- age (integer)")
	assert.Contains(t, text, "Any of:")
	assert.Equal(t, 1, strings.Count(text, "- tag (string)"), "array item properties must not be listed twice")
}
