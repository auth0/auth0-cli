package openapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// sortedPropertyNames returns a schema's property names in deterministic
// (alphabetical) order. Go randomizes map iteration, so ranging Properties
// directly would render fields in a different order on every run.
func sortedPropertyNames(props openapi3.Schemas) []string {
	names := make([]string, 0, len(props))
	for name := range props {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// SchemaManager provides access to OpenAPI operation schemas and request validation.
type SchemaManager struct {
	doc *openapi3.T
}

// NewSchemaManager creates a new schema manager backed by the cached OpenAPI document.
func NewSchemaManager() (*SchemaManager, error) {
	doc, err := GetDoc()
	if err != nil {
		return nil, err
	}
	return &SchemaManager{doc: doc}, nil
}

// NewSchemaManagerFromDoc creates a schema manager backed by an already-loaded
// document instead of fetching one. Tests use it with a fixture document so the
// unit suite validates locally and never reaches the network.
func NewSchemaManagerFromDoc(doc *openapi3.T) *SchemaManager {
	return &SchemaManager{doc: doc}
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

	if requestSchema := GetRequestSchema(operation); requestSchema != nil && requestSchema.Value != nil {
		result.RequestSchema = requestSchema.Value
	}

	if result.RequestSchema == nil {
		if qpSchema := GetQueryParamSchema(operation); qpSchema != nil {
			result.RequestSchema = qpSchema
			result.IsQueryParamSchema = true
		}
	}

	return result, nil
}

// OperationSchema contains schema information for an API operation.
type OperationSchema struct {
	OperationID        string
	Summary            string
	Description        string
	Method             string
	Path               string
	RequestSchema      *openapi3.Schema
	IsQueryParamSchema bool
}

// FormatAsJSON formats the schema as JSON for display.
func (op *OperationSchema) FormatAsJSON() (string, error) {
	output := map[string]interface{}{
		"operation_id": op.OperationID,
		"summary":      op.Summary,
		"description":  op.Description,
		"method":       op.Method,
		"path":         op.Path,
	}

	if op.RequestSchema != nil {
		key := "request_schema"
		if op.IsQueryParamSchema {
			key = "query_params_schema"
		}
		output[key] = schemaToMap(op.RequestSchema)
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FormatAsText formats the schema as human-readable text.
func (op *OperationSchema) FormatAsText() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Operation: %s\n", op.Summary)
	fmt.Fprintf(&sb, "Endpoint: %s %s\n", op.Method, op.Path)
	if op.Description != "" {
		fmt.Fprintf(&sb, "Description: %s\n", op.Description)
	}
	sb.WriteString("\n")

	if op.RequestSchema != nil {
		if op.IsQueryParamSchema {
			sb.WriteString("Query Parameters:\n")
		} else {
			sb.WriteString("Request Payload:\n")
		}
		sb.WriteString(strings.Repeat("=", 80))
		sb.WriteString("\n\n")
		sb.WriteString(formatSchema(op.RequestSchema, ""))
	} else {
		sb.WriteString("No request body required for this operation.\n")
	}

	return sb.String()
}

// ValidateRequest checks a --data payload against the operation's OpenAPI
// request schema by visiting the parsed JSON with kin-openapi (VisitJSON).
// JSON syntax is always checked first, so malformed input is reported as
// StatusInvalid even when the operation defines no schema to validate against.
func (sm *SchemaManager) ValidateRequest(method, path string, body []byte) (*ValidationResult, error) {
	result := &ValidationResult{Status: StatusValid, Errors: []string{}}

	operation, err := FindOperation(sm.doc, method, path)
	if err != nil {
		return nil, fmt.Errorf("operation not found: %w", err)
	}

	// Parse the JSON body first. Well-formedness is independent of having a
	// schema, so malformed input fails locally even on schema-less operations
	// instead of being shipped to the API as opaque bytes.
	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		result.Status = StatusInvalid
		result.addFieldErrors([]FieldError{{Reason: fmt.Sprintf("Invalid JSON: %v", err)}})
		return result, nil
	}

	requestSchema := GetRequestSchema(operation)
	if requestSchema == nil || requestSchema.Value == nil {
		result.Status = StatusNoSchema
		return result, nil
	}

	// Validate against schema. MultiErrors collects every validation failure
	// instead of stopping at the first, so the caller sees all issues at once.
	// Some spec patterns use lookarounds, which RE2 can't compile; see compilePattern.
	// IP and CIDR formats need validators for their oneOf branches; see ipStringFormats.
	if err := requestSchema.Value.VisitJSON(data,
		openapi3.MultiErrors(),
		openapi3.SetSchemaRegexCompiler(compilePattern),
		openapi3.WithStringFormatValidators(ipStringFormats),
	); err != nil {
		result.Status = StatusInvalid
		result.addFieldErrors(fieldValidationErrors(err))
		return result, nil
	}

	return result, nil
}

// FieldError is a single structured validation failure. Field is a JSONPath-style
// location within the payload ("payload" for a whole-body error, empty when the
// failure is not tied to a location, e.g. malformed JSON); Reason is the message.
type FieldError struct {
	Field  string `json:"field,omitempty"`
	Reason string `json:"reason"`
}

// ValidationStatus describes what happened when a payload was checked against
// the operation's OpenAPI request schema.
type ValidationStatus int

const (
	// StatusUnknown is the zero value and means no outcome was recorded. It exists
	// so a ValidationResult{} left unset fails closed (never reads as StatusValid)
	// rather than silently passing a payload through as validated.
	StatusUnknown ValidationStatus = iota
	// StatusValid means a request schema existed and the payload satisfied it.
	StatusValid
	// StatusInvalid means a request schema existed and the payload failed it (see Errors).
	StatusInvalid
	// StatusNoSchema means the operation defines no request schema, so the payload
	// was not checked and is sent as-is.
	StatusNoSchema
)

// String renders a FieldError as "field: reason", or just the reason when the
// error is not tied to a field.
func (e FieldError) String() string {
	if e.Field == "" {
		return e.Reason
	}
	return e.Field + ": " + e.Reason
}

// ValidationResult contains the result of schema validation. Status reports
// whether a schema existed and whether the payload satisfied it; Errors holds the
// human-readable strings; FieldErrors holds the same failures in structured form
// for the JSON error envelope's "details" field.
type ValidationResult struct {
	Status      ValidationStatus
	Errors      []string
	FieldErrors []FieldError
}

// addFieldErrors appends structured failures and keeps the human-readable Errors
// slice in sync, so both views describe the same set of failures.
func (r *ValidationResult) addFieldErrors(fes []FieldError) {
	for _, fe := range fes {
		r.FieldErrors = append(r.FieldErrors, fe)
		r.Errors = append(r.Errors, fe.String())
	}
}

// fieldValidationErrors turns a kin-openapi validation error into concise,
// structured field errors, reading SchemaError's fields so raw "$ref" entries
// never leak.
func fieldValidationErrors(err error) []FieldError {
	var out []FieldError

	var multiErr openapi3.MultiError
	if errors.As(err, &multiErr) {
		for _, e := range multiErr {
			out = append(out, fieldValidationErrors(e)...)
		}
		return out
	}

	var schemaErr *openapi3.SchemaError
	if errors.As(err, &schemaErr) {
		location := jsonPath(schemaErr.JSONPointer())
		reason := schemaErr.Reason
		if reason == "" {
			reason = fmt.Sprintf("does not match schema constraint %q", schemaErr.SchemaField)
		}
		return []FieldError{{Field: location, Reason: reason}}
	}

	return []FieldError{{Reason: err.Error()}}
}

// jsonPath renders JSON Pointer segments as a JSONPath-style query, e.g.
// ["supported_triggers","0","id"] → "supported_triggers[0].id" ("payload" if empty).
func jsonPath(segments []string) string {
	if len(segments) == 0 {
		return "payload"
	}
	var sb strings.Builder
	for i, seg := range segments {
		switch {
		case isArrayIndex(seg):
			fmt.Fprintf(&sb, "[%s]", seg)
		case isSimpleIdentifier(seg):
			if i > 0 {
				sb.WriteByte('.')
			}
			sb.WriteString(seg)
		default:
			// Keys with dots, spaces, etc. use bracket-quoted notation.
			fmt.Fprintf(&sb, "[%q]", seg)
		}
	}
	return sb.String()
}

// isArrayIndex reports whether seg is a non-negative integer (an array index).
func isArrayIndex(seg string) bool {
	if seg == "" {
		return false
	}
	for _, r := range seg {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// isSimpleIdentifier reports whether seg can be written with dot notation
// (letters, digits, underscores; not starting with a digit).
func isSimpleIdentifier(seg string) bool {
	if seg == "" {
		return false
	}
	for i, r := range seg {
		isLetter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_'
		isDigit := r >= '0' && r <= '9'
		if i == 0 && !isLetter {
			return false
		}
		if !isLetter && !isDigit {
			return false
		}
	}
	return true
}

// recursiveSchemaNote replaces a schema that contains itself (e.g. a flow action
// whose params hold more flow actions), so rendering terminates.
const recursiveSchemaNote = "Recursive reference to an enclosing schema."

// composition is one schema composition keyword and its branches.
type composition struct {
	key   string // JSON key, e.g. "oneOf".
	label string // Text heading, e.g. "One of".
	refs  openapi3.SchemaRefs
}

// compositionsOf returns the schema's oneOf, anyOf and allOf branches.
func compositionsOf(schema *openapi3.Schema) []composition {
	return []composition{
		{"oneOf", "One of", schema.OneOf},
		{"anyOf", "Any of", schema.AnyOf},
		{"allOf", "All of", schema.AllOf},
	}
}

// variantName names a composition branch by its title or its $ref's last segment.
func variantName(ref *openapi3.SchemaRef) string {
	if ref.Value != nil && ref.Value.Title != "" {
		return ref.Value.Title
	}
	return ref.Ref[strings.LastIndex(ref.Ref, "/")+1:]
}

// schemaToMap converts an OpenAPI schema to a map for JSON serialization.
func schemaToMap(schema *openapi3.Schema) map[string]interface{} {
	return schemaToMapGuarded(schema, map[*openapi3.Schema]bool{})
}

// schemaToMapGuarded is schemaToMap tracking the schemas on the current path, so a
// schema that contains itself renders once instead of recursing forever.
func schemaToMapGuarded(schema *openapi3.Schema, ancestors map[*openapi3.Schema]bool) map[string]interface{} {
	if ancestors[schema] {
		return map[string]interface{}{"description": recursiveSchemaNote}
	}
	ancestors[schema] = true
	defer delete(ancestors, schema)

	result := make(map[string]interface{})

	if schema.Type != nil {
		result["type"] = schema.Type.Slice()
	}

	if schema.Title != "" {
		result["title"] = schema.Title
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
				props[name] = schemaToMapGuarded(propRef.Value, ancestors)
			}
		}
		result["properties"] = props
	}

	if schema.Items != nil && schema.Items.Value != nil {
		result["items"] = schemaToMapGuarded(schema.Items.Value, ancestors)
	}

	for _, c := range compositionsOf(schema) {
		var variants []map[string]interface{}
		for _, ref := range c.refs {
			if ref.Value == nil {
				continue
			}
			variant := schemaToMapGuarded(ref.Value, ancestors)
			if name := variantName(ref); name != "" && variant["title"] == nil {
				variant["title"] = name
			}
			variants = append(variants, variant)
		}
		if len(variants) > 0 {
			result[c.key] = variants
		}
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
	return formatSchemaGuarded(schema, indent, map[*openapi3.Schema]bool{})
}

// writeCompositions lists each oneOf/anyOf/allOf branch by name, then its fields.
func writeCompositions(sb *strings.Builder, schema *openapi3.Schema, indent string, ancestors map[*openapi3.Schema]bool) {
	for _, c := range compositionsOf(schema) {
		if len(c.refs) == 0 {
			continue
		}
		fmt.Fprintf(sb, "%s%s:\n", indent, c.label)
		for i, ref := range c.refs {
			if ref.Value == nil {
				continue
			}
			fmt.Fprintf(sb, "%s  Variant %d", indent, i+1)
			if name := variantName(ref); name != "" {
				fmt.Fprintf(sb, ": %s", name)
			}
			sb.WriteString("\n")
			sb.WriteString(formatSchemaGuarded(ref.Value, indent+"    ", ancestors))
		}
	}
}

// formatSchemaGuarded is formatSchema tracking the schemas on the current path, so
// a schema that contains itself renders once instead of recursing forever.
func formatSchemaGuarded(schema *openapi3.Schema, indent string, ancestors map[*openapi3.Schema]bool) string {
	if ancestors[schema] {
		return indent + recursiveSchemaNote + "\n"
	}
	ancestors[schema] = true
	defer delete(ancestors, schema)

	var sb strings.Builder

	switch {
	case schema.Type != nil && schema.Type.Is("object"):
		// Required fields.
		if len(schema.Required) > 0 {
			fmt.Fprintf(&sb, "%sRequired fields:\n", indent)
			for _, fieldName := range schema.Required {
				if propRef, ok := schema.Properties[fieldName]; ok && propRef.Value != nil {
					sb.WriteString(formatFieldGuarded(fieldName, propRef.Value, indent+"  ", ancestors))
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
		sort.Strings(optionalFields)

		if len(optionalFields) > 0 {
			fmt.Fprintf(&sb, "%sOptional fields:\n", indent)
			for _, fieldName := range optionalFields {
				if propRef, ok := schema.Properties[fieldName]; ok && propRef.Value != nil {
					sb.WriteString(formatFieldGuarded(fieldName, propRef.Value, indent+"  ", ancestors))
				}
			}
		}
	case schema.Type != nil && schema.Type.Is("array") && schema.Items != nil && schema.Items.Value != nil:
		// A top-level array body (e.g. PATCH /connections/{id}/clients takes an
		// array of {client_id, status}). Describe the array, then recurse into the
		// item schema so its fields surface instead of a bare "Type: array".
		fmt.Fprintf(&sb, "%sType: array\n", indent)
		if schema.Description != "" {
			fmt.Fprintf(&sb, "%sDescription: %s\n", indent, schema.Description)
		}
		fmt.Fprintf(&sb, "%sEach item:\n", indent)
		sb.WriteString(formatSchemaGuarded(schema.Items.Value, indent+"  ", ancestors))
	default:
		// Non-object type.
		if schema.Type != nil {
			types := schema.Type.Slice()
			fmt.Fprintf(&sb, "%sType: %s\n", indent, strings.Join(types, "|"))
		}
		if schema.Description != "" {
			fmt.Fprintf(&sb, "%sDescription: %s\n", indent, schema.Description)
		}
	}

	writeCompositions(&sb, schema, indent, ancestors)

	return sb.String()
}

// formatField formats a single field with its type and description.
func formatField(name string, schema *openapi3.Schema, indent string) string {
	return formatFieldGuarded(name, schema, indent, map[*openapi3.Schema]bool{})
}

// formatFieldGuarded is formatField tracking the schemas on the current path, so a
// field whose schema contains itself renders once instead of recursing forever.
func formatFieldGuarded(name string, schema *openapi3.Schema, indent string, ancestors map[*openapi3.Schema]bool) string {
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

	if ancestors[schema] {
		fmt.Fprintf(&sb, "%s  %s\n", indent, recursiveSchemaNote)
		return sb.String()
	}
	ancestors[schema] = true
	defer delete(ancestors, schema)

	// If field is an object with properties, show nested structure.
	if schema.Type != nil && schema.Type.Is("object") && len(schema.Properties) > 0 {
		fmt.Fprintf(&sb, "%s  Properties:\n", indent)
		for _, propName := range sortedPropertyNames(schema.Properties) {
			if propRef := schema.Properties[propName]; propRef.Value != nil {
				sb.WriteString(formatFieldGuarded(propName, propRef.Value, indent+"    ", ancestors))
			}
		}
	}

	// If field is an array with object items, show item structure.
	if schema.Type != nil && schema.Type.Is("array") && schema.Items != nil && schema.Items.Value != nil {
		itemSchema := schema.Items.Value
		if itemSchema.Type != nil && itemSchema.Type.Is("object") && len(itemSchema.Properties) > 0 {
			fmt.Fprintf(&sb, "%s  Item properties:\n", indent)
			for _, propName := range sortedPropertyNames(itemSchema.Properties) {
				if propRef := itemSchema.Properties[propName]; propRef.Value != nil {
					sb.WriteString(formatFieldGuarded(propName, propRef.Value, indent+"    ", ancestors))
				}
			}
		}
		if len(itemSchema.OneOf)+len(itemSchema.AnyOf)+len(itemSchema.AllOf) > 0 {
			fmt.Fprintf(&sb, "%s  Each item:\n", indent)
			writeCompositions(&sb, itemSchema, indent+"    ", ancestors)
		}
	}

	writeCompositions(&sb, schema, indent+"  ", ancestors)

	return sb.String()
}
