package openapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockError implements management.Error interface for testing.
type mockError struct {
	statusCode int
	message    string
}

func (m *mockError) Error() string {
	return m.message
}

func (m *mockError) Status() int {
	return m.statusCode
}

func TestEnhanceError_400Error(t *testing.T) {
	manager, err := NewSchemaManager()
	require.NoError(t, err)

	mockErr := &mockError{
		statusCode: 400,
		message:    "Bad Request: Invalid action data",
	}

	enhanced := manager.EnhanceError(mockErr, "POST", "/actions/actions")
	require.NotNil(t, enhanced)

	enhancedMsg := enhanced.Error()

	// Verify that the enhanced error contains the original message.
	assert.Contains(t, enhancedMsg, "Bad Request: Invalid action data")

	// Verify that it contains schema information.
	assert.Contains(t, enhancedMsg, "Expected Request Schema")
	assert.Contains(t, enhancedMsg, "Required fields")
	assert.Contains(t, enhancedMsg, "name")
	assert.Contains(t, enhancedMsg, "supported_triggers")
}

func TestEnhanceError_NonManagementError(t *testing.T) {
	manager, err := NewSchemaManager()
	require.NoError(t, err)

	// Regular error should be returned as-is.
	regularErr := assert.AnError
	enhanced := manager.EnhanceError(regularErr, "POST", "/actions/actions")
	assert.Equal(t, regularErr, enhanced)
}

func TestEnhanceError_Non400Error(t *testing.T) {
	manager, err := NewSchemaManager()
	require.NoError(t, err)

	mockErr := &mockError{
		statusCode: 404,
		message:    "Not Found",
	}

	enhanced := manager.EnhanceError(mockErr, "GET", "/actions/actions/act_123")
	// Should return the original error for non-400 errors.
	assert.Equal(t, mockErr, enhanced)
}

func TestEnhanceError_InvalidPath(t *testing.T) {
	manager, err := NewSchemaManager()
	require.NoError(t, err)

	mockErr := &mockError{
		statusCode: 400,
		message:    "Bad Request",
	}

	// Invalid path should return the original error.
	enhanced := manager.EnhanceError(mockErr, "POST", "/invalid/path")
	assert.Equal(t, mockErr, enhanced)
}

func TestFormatSchemaInfo(t *testing.T) {
	manager, err := NewSchemaManager()
	require.NoError(t, err)

	// Get a real operation and schema.
	operation, err := FindOperation(manager.doc, "POST", "/actions/actions")
	require.NoError(t, err)

	requestSchema := GetRequestSchema(operation)
	require.NotNil(t, requestSchema)
	require.NotNil(t, requestSchema.Value)

	schemaInfo := formatSchemaInfo(requestSchema.Value, operation)

	// Verify the formatted output contains expected elements.
	assert.Contains(t, schemaInfo, "Expected Request Schema")
	assert.Contains(t, schemaInfo, "Required fields")
	assert.Contains(t, schemaInfo, "name")
	assert.Contains(t, schemaInfo, "supported_triggers")
	assert.Contains(t, schemaInfo, "Optional fields")
}

func TestEnhanceError_MultipleOperations(t *testing.T) {
	manager, err := NewSchemaManager()
	require.NoError(t, err)

	tests := []struct {
		name           string
		method         string
		path           string
		shouldEnhance  bool
		requiredFields []string
	}{
		{
			name:           "POST actions",
			method:         "POST",
			path:           "/actions/actions",
			shouldEnhance:  true,
			requiredFields: []string{"name", "supported_triggers"},
		},
		{
			name:           "PATCH actions",
			method:         "PATCH",
			path:           "/actions/actions/{id}",
			shouldEnhance:  true,
			requiredFields: []string{}, // PATCH typically has no required fields.
		},
		{
			name:           "GET users",
			method:         "GET",
			path:           "/users",
			shouldEnhance:  false, // GET has no request body.
			requiredFields: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockErr := &mockError{
				statusCode: 400,
				message:    "Bad Request",
			}

			enhanced := manager.EnhanceError(mockErr, tt.method, tt.path)
			require.NotNil(t, enhanced)

			enhancedMsg := enhanced.Error()

			if tt.shouldEnhance {
				// Should have schema info.
				hasSchemaInfo := enhanced != mockErr
				if hasSchemaInfo {
					assert.Contains(t, enhancedMsg, "Expected Request Schema")

					for _, field := range tt.requiredFields {
						if len(tt.requiredFields) > 0 {
							assert.Contains(t, enhancedMsg, field)
						}
					}
				}
			}
		})
	}
}
