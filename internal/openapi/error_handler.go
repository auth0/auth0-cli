package openapi

import (
	"fmt"
	"strings"

	"github.com/auth0/go-auth0/management"
	"github.com/getkin/kin-openapi/openapi3"
)

// EnhanceError appends the expected request schema to a 400 Bad Request error.
// Non-400 errors are returned unchanged.
func (sm *SchemaManager) EnhanceError(err error, method, path string) error {
	if err == nil {
		return nil
	}

	// Only enhance 400 Bad Request errors from the management API.
	mgmtErr, ok := err.(management.Error)
	if !ok || mgmtErr.Status() != 400 {
		return err
	}

	// Return the original error if the schema can't supply a hint.
	operation, opErr := FindOperation(sm.doc, method, path)
	if opErr != nil {
		return err
	}
	requestSchema := GetRequestSchema(operation)
	if requestSchema == nil {
		return err
	}

	schemaInfo := formatSchemaInfo(requestSchema.Value, operation)
	return fmt.Errorf("%s\n\n%s", err.Error(), schemaInfo)
}

// formatSchemaInfo renders the expected request schema, reusing the shared
// renderer so the 400 hint matches '--schema' output and stays $ref-free.
func formatSchemaInfo(schema *openapi3.Schema, operation *openapi3.Operation) string {
	var sb strings.Builder

	sb.WriteString("Expected Request Schema:\n")
	sb.WriteString("=======================\n\n")

	if operation.Summary != "" {
		fmt.Fprintf(&sb, "Operation: %s\n\n", operation.Summary)
	}

	sb.WriteString(formatSchema(schema, ""))

	return sb.String()
}
