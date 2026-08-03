package openapi

import (
	"fmt"
	"strings"

	"github.com/auth0/go-auth0/management"
	"github.com/getkin/kin-openapi/openapi3"
)

// EnhanceError appends the expected request schema to an error when it is a
// 400 Bad Request. Non-400 errors are returned unchanged. It reuses the manager's
// already-loaded document, so no separate type is needed.
func (sm *SchemaManager) EnhanceError(err error, method, path string) error {
	if err == nil {
		return nil
	}

	// Check if it's a management API error with status code 400.
	mgmtErr, ok := err.(management.Error)
	if !ok || mgmtErr.Status() != 400 {
		return err
	}

	// Find the operation in the schema.
	operation, opErr := FindOperation(sm.doc, method, path)
	if opErr != nil {
		// If we can't find the operation, return the original error.
		return err
	}

	// Get the request schema.
	requestSchema := GetRequestSchema(operation)
	if requestSchema == nil {
		return err
	}

	// Build the enhanced error message.
	schemaInfo := formatSchemaInfo(requestSchema.Value, operation)
	enhancedMsg := fmt.Sprintf("%s\n\n%s", err.Error(), schemaInfo)

	return fmt.Errorf("%s", enhancedMsg)
}

// formatSchemaInfo formats schema information for display. It reuses the same
// resolved-schema renderer as the 'schema' output so both stay consistent and
// never surface unresolved "$ref" entries.
func formatSchemaInfo(schema *openapi3.Schema, operation *openapi3.Operation) string {
	var sb strings.Builder

	sb.WriteString("Expected Request Schema:\n")
	sb.WriteString("=======================\n\n")

	// Add operation summary if available.
	if operation.Summary != "" {
		fmt.Fprintf(&sb, "Operation: %s\n\n", operation.Summary)
	}

	sb.WriteString(formatSchema(schema, ""))

	return sb.String()
}
