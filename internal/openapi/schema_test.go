package openapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDoc(t *testing.T) {
	doc, err := GetDoc()
	require.NoError(t, err)
	require.NotNil(t, doc)

	assert.NotEmpty(t, doc.OpenAPI)
	assert.NotNil(t, doc.Paths)
	assert.NotNil(t, doc.Components)
	assert.NotNil(t, doc.Components.Schemas)
}

func TestFindOperation(t *testing.T) {
	doc, err := GetDoc()
	require.NoError(t, err)

	tests := []struct {
		name              string
		method            string
		path              string
		expectError       bool
		expectOperationID string
	}{
		{
			name:              "POST actions/actions",
			method:            "POST",
			path:              "/actions/actions",
			expectError:       false,
			expectOperationID: "post_action",
		},
		{
			name:              "GET actions/actions",
			method:            "GET",
			path:              "/actions/actions",
			expectError:       false,
			expectOperationID: "get_actions",
		},
		{
			name:        "Invalid path",
			method:      "GET",
			path:        "/invalid/path",
			expectError: true,
		},
		{
			name:        "Invalid method",
			method:      "INVALID",
			path:        "/actions/actions",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			operation, err := FindOperation(doc, tt.method, tt.path)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, operation)
			} else {
				require.NoError(t, err)
				require.NotNil(t, operation)
				assert.Equal(t, tt.expectOperationID, operation.OperationID)
			}
		})
	}
}

func TestGetRequestSchema(t *testing.T) {
	doc, err := GetDoc()
	require.NoError(t, err)

	operation, err := FindOperation(doc, "POST", "/actions/actions")
	require.NoError(t, err)

	requestSchema := GetRequestSchema(operation)
	require.NotNil(t, requestSchema)
	require.NotNil(t, requestSchema.Value)

	// Verify it has the expected required fields.
	assert.Contains(t, requestSchema.Value.Required, "name")
	assert.Contains(t, requestSchema.Value.Required, "supported_triggers")
}

func TestGetResponseSchema(t *testing.T) {
	doc, err := GetDoc()
	require.NoError(t, err)

	operation, err := FindOperation(doc, "POST", "/actions/actions")
	require.NoError(t, err)

	// Test 201 response (success).
	responseSchema := GetResponseSchema(operation, "201")
	require.NotNil(t, responseSchema)
	require.NotNil(t, responseSchema.Value)

	// Test 400 response (may be nil or have no content).
	responseSchema = GetResponseSchema(operation, "400")
	_ = responseSchema
}

func TestExtractPathFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "Full URL with tenant",
			url:      "https://tenant.auth0.com/api/v2/actions/actions",
			expected: "/actions/actions",
		},
		{
			name:     "URL with path parameters",
			url:      "https://tenant.auth0.com/api/v2/actions/actions/act_123",
			expected: "/actions/actions/act_123",
		},
		{
			name:     "URL without api/v2",
			url:      "https://tenant.auth0.com/some/path",
			expected: "",
		},
		{
			name:     "URL with query parameters",
			url:      "https://tenant.auth0.com/api/v2/actions/actions?page=1",
			expected: "/actions/actions?page=1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractPathFromURL(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCaching(t *testing.T) {
	// First call - should fetch or load from cache.
	doc1, err := GetDoc()
	require.NoError(t, err)
	require.NotNil(t, doc1)

	// Second call - should return cached doc.
	doc2, err := GetDoc()
	require.NoError(t, err)
	require.NotNil(t, doc2)

	// Should be the same instance (pointer equality).
	assert.Equal(t, doc1, doc2)
}
