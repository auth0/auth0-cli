package openapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// SchemaManager provides centralized access to OpenAPI schemas.
// It loads the schema once and provides methods to inspect and validate requests.
type SchemaManager struct {
	doc *openapi3.T
}

// NewSchemaManager creates a new schema manager.
// The schema is loaded once and cached for the lifetime of the manager.
func NewSchemaManager() (*SchemaManager, error) {
	doc, err := GetDoc()
	if err != nil {
		return nil, err
	}
	return &SchemaManager{doc: doc}, nil
}

// GetOperationSchema returns the schema information for an operation.
func (sm *SchemaManager) GetOperationSchema(method, path string) (*OperationSchema, error) {
	operation, err := FindOperation(sm.doc, method, path)
	if err != nil {
		return nil, err
	}

	result := &OperationSchema{
		OperationID: operation.OperationID,
		Summary:     operation.Summary,
		Description: operation.Description,
		Method:      strings.ToUpper(method),
		Path:        path,
	}

	// Get request schema only - agents only need to know what to send.
	if requestSchema := GetRequestSchema(operation); requestSchema != nil && requestSchema.Value != nil {
		result.RequestSchema = requestSchema.Value
	}

	return result, nil
}

// OperationSchema contains schema information for an API operation.
// Focus is on request payload - what agents need to send.
type OperationSchema struct {
	OperationID   string
	Summary       string
	Description   string
	Method        string
	Path          string
	RequestSchema *openapi3.Schema
}

// FormatAsJSON formats the schema as JSON for display.
func (os *OperationSchema) FormatAsJSON() (string, error) {
	output := map[string]interface{}{
		"operation_id": os.OperationID,
		"summary":      os.Summary,
		"description":  os.Description,
		"method":       os.Method,
		"path":         os.Path,
	}

	if os.RequestSchema != nil {
		output["request_schema"] = schemaToMap(os.RequestSchema)
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FormatAsText formats the schema as human-readable text.
func (os *OperationSchema) FormatAsText() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Operation: %s\n", os.Summary)
	fmt.Fprintf(&sb, "Endpoint: %s %s\n", os.Method, os.Path)
	if os.Description != "" {
		fmt.Fprintf(&sb, "Description: %s\n", os.Description)
	}
	sb.WriteString("\n")

	if os.RequestSchema != nil {
		sb.WriteString("Request Payload:\n")
		sb.WriteString(strings.Repeat("=", 80))
		sb.WriteString("\n\n")
		sb.WriteString(formatSchema(os.RequestSchema, ""))
	} else {
		sb.WriteString("No request body required for this operation.\n")
	}

	return sb.String()
}

// ValidateRequest validates a request using openapi3filter.
func (sm *SchemaManager) ValidateRequest(method, path string, body []byte) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:  true,
		Errors: []string{},
	}

	operation, err := FindOperation(sm.doc, method, path)
	if err != nil {
		return nil, fmt.Errorf("operation not found: %w", err)
	}

	requestSchema := GetRequestSchema(operation)
	if requestSchema == nil || requestSchema.Value == nil {
		// No schema to validate against.
		return result, nil
	}

	// Parse the JSON body.
	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("Invalid JSON: %v", err))
		return result, nil
	}

	// Validate against schema.
	if err := requestSchema.Value.VisitJSON(data); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, formatValidationError(err)...)
		return result, nil
	}

	return result, nil
}

// ValidationResult contains the result of schema validation.
type ValidationResult struct {
	Valid  bool
	Errors []string
}

// formatValidationError turns a kin-openapi validation error into concise,
// user-facing messages. It reads the structured fields of *openapi3.SchemaError
// (field pointer + reason) instead of the default Error(), which dumps the raw
// schema — including unresolved "$ref" entries. Direct the user to '--schema'
// for the fully resolved schema.
func formatValidationError(err error) []string {
	var messages []string

	var multiErr openapi3.MultiError
	if errors.As(err, &multiErr) {
		for _, e := range multiErr {
			messages = append(messages, formatValidationError(e)...)
		}
		return messages
	}

	var schemaErr *openapi3.SchemaError
	if errors.As(err, &schemaErr) {
		location := "/" + strings.Join(schemaErr.JSONPointer(), "/")
		reason := schemaErr.Reason
		if reason == "" {
			reason = fmt.Sprintf("does not match schema constraint %q", schemaErr.SchemaField)
		}
		return []string{fmt.Sprintf("Field %q: %s", location, reason)}
	}

	return []string{err.Error()}
}

// schemaToMap converts an OpenAPI schema to a map for JSON serialization.
func schemaToMap(schema *openapi3.Schema) map[string]interface{} {
	result := make(map[string]interface{})

	if schema.Type != nil {
		result["type"] = schema.Type.Slice()
	}

	if schema.Description != "" {
		result["description"] = schema.Description
	}

	if len(schema.Required) > 0 {
		result["required"] = schema.Required
	}

	if len(schema.Properties) > 0 {
		props := make(map[string]interface{})
		for name, propRef := range schema.Properties {
			if propRef.Value != nil {
				props[name] = schemaToMap(propRef.Value)
			}
		}
		result["properties"] = props
	}

	if schema.Items != nil && schema.Items.Value != nil {
		result["items"] = schemaToMap(schema.Items.Value)
	}

	if len(schema.Enum) > 0 {
		result["enum"] = schema.Enum
	}

	if schema.Default != nil {
		result["default"] = schema.Default
	}

	if schema.MinLength != 0 {
		result["minLength"] = schema.MinLength
	}

	if schema.MaxLength != nil {
		result["maxLength"] = *schema.MaxLength
	}

	if schema.Pattern != "" {
		result["pattern"] = schema.Pattern
	}

	if schema.MinItems != 0 {
		result["minItems"] = schema.MinItems
	}

	if schema.MaxItems != nil {
		result["maxItems"] = *schema.MaxItems
	}

	return result
}

// formatSchema formats a schema as human-readable text.
func formatSchema(schema *openapi3.Schema, indent string) string {
	var sb strings.Builder

	if schema.Type != nil && schema.Type.Is("object") {
		// Required fields.
		if len(schema.Required) > 0 {
			fmt.Fprintf(&sb, "%sRequired fields:\n", indent)
			for _, fieldName := range schema.Required {
				if propRef, ok := schema.Properties[fieldName]; ok && propRef.Value != nil {
					sb.WriteString(formatField(fieldName, propRef.Value, indent+"  "))
				}
			}
			sb.WriteString("\n")
		}

		// Optional fields.
		optionalFields := []string{}
		for fieldName := range schema.Properties {
			isRequired := false
			for _, req := range schema.Required {
				if req == fieldName {
					isRequired = true
					break
				}
			}
			if !isRequired {
				optionalFields = append(optionalFields, fieldName)
			}
		}

		if len(optionalFields) > 0 {
			fmt.Fprintf(&sb, "%sOptional fields:\n", indent)
			for _, fieldName := range optionalFields {
				if propRef, ok := schema.Properties[fieldName]; ok && propRef.Value != nil {
					sb.WriteString(formatField(fieldName, propRef.Value, indent+"  "))
				}
			}
		}
	} else {
		// Non-object type.
		if schema.Type != nil {
			types := schema.Type.Slice()
			fmt.Fprintf(&sb, "%sType: %s\n", indent, strings.Join(types, "|"))
		}
		if schema.Description != "" {
			fmt.Fprintf(&sb, "%sDescription: %s\n", indent, schema.Description)
		}
	}

	return sb.String()
}

// formatField formats a single field with its type and description.
func formatField(name string, schema *openapi3.Schema, indent string) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "%s- %s", indent, name)

	if schema.Type != nil {
		types := schema.Type.Slice()
		fmt.Fprintf(&sb, " (%s)", strings.Join(types, "|"))
	}

	if schema.Description != "" {
		fmt.Fprintf(&sb, ": %s", schema.Description)
	}

	if len(schema.Enum) > 0 {
		fmt.Fprintf(&sb, " [possible values: %v]", schema.Enum)
	}

	if schema.Default != nil {
		fmt.Fprintf(&sb, " (default: %v)", schema.Default)
	}

	sb.WriteString("\n")

	// If field is an object with properties, show nested structure.
	if schema.Type != nil && schema.Type.Is("object") && len(schema.Properties) > 0 {
		fmt.Fprintf(&sb, "%s  Properties:\n", indent)
		for propName, propRef := range schema.Properties {
			if propRef.Value != nil {
				sb.WriteString(formatField(propName, propRef.Value, indent+"    "))
			}
		}
	}

	// If field is an array with object items, show item structure.
	if schema.Type != nil && schema.Type.Is("array") && schema.Items != nil && schema.Items.Value != nil {
		itemSchema := schema.Items.Value
		if itemSchema.Type != nil && itemSchema.Type.Is("object") && len(itemSchema.Properties) > 0 {
			fmt.Fprintf(&sb, "%s  Item properties:\n", indent)
			for propName, propRef := range itemSchema.Properties {
				if propRef.Value != nil {
					sb.WriteString(formatField(propName, propRef.Value, indent+"    "))
				}
			}
		}
	}

	return sb.String()
}

// GetResourceOperations returns all operations for a resource (e.g., "actions").
func (sm *SchemaManager) GetResourceOperations(resource string) ([]*OperationSchema, error) {
	var operations []*OperationSchema

	// Common resource paths.
	basePath := fmt.Sprintf("/%s", resource)
	idPath := fmt.Sprintf("/%s/{id}", resource)

	// Try to find operations.
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		// Try base path.
		if op, err := sm.GetOperationSchema(method, basePath); err == nil {
			operations = append(operations, op)
		}

		// Try ID path.
		if op, err := sm.GetOperationSchema(method, idPath); err == nil {
			operations = append(operations, op)
		}
	}

	// Special cases for nested resources.
	specialPaths := []string{
		fmt.Sprintf("/%s/%s", resource, resource), // E.g., /actions/actions.
	}

	for _, path := range specialPaths {
		for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
			if op, err := sm.GetOperationSchema(method, path); err == nil {
				operations = append(operations, op)
			}
		}
	}

	if len(operations) == 0 {
		return nil, fmt.Errorf("no operations found for resource: %s", resource)
	}

	return operations, nil
}

// ListAllOperations returns all operations in the OpenAPI spec.
func (sm *SchemaManager) ListAllOperations() []OperationInfo {
	var operations []OperationInfo

	for path, pathItem := range sm.doc.Paths.Map() {
		for method, operation := range pathItem.Operations() {
			if operation != nil {
				operations = append(operations, OperationInfo{
					Method:      strings.ToUpper(method),
					Path:        path,
					OperationID: operation.OperationID,
					Summary:     operation.Summary,
					Tags:        operation.Tags,
				})
			}
		}
	}

	return operations
}

// OperationInfo contains basic information about an operation.
type OperationInfo struct {
	Method      string
	Path        string
	OperationID string
	Summary     string
	Tags        []string
}
