package registry

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/auth0/auth0-cli/internal/plugins"
)

// schemaURL identifies the embedded schema. The schema is only ever loaded
// from IndexSchema and is never fetched from this address.
const schemaURL = "https://raw.githubusercontent.com/auth0/auth0-cli-plugins/main/schema/index.v1.schema.json"

// indexSchema holds the embedded JSON Schema.
//
//go:embed schema/index.schema.json
var indexSchema []byte

// IndexSchema returns the JSON Schema (draft 2020-12) that describes the
// index.json of a plugin registry. It is embedded so that the CLI and the
// plugin catalog can publish and enforce the exact same file. The caller gets
// its own copy of the bytes, so changing them cannot affect validation.
func IndexSchema() []byte {
	return slices.Clone(indexSchema)
}

// schemas holds the compiled schemas that the registry client validates with.
type schemas struct {
	index        *jsonschema.Schema
	versionsFile *jsonschema.Schema
}

// compileSchemas compiles the embedded schema on first use and reuses the
// result.
var compileSchemas = sync.OnceValues(func() (schemas, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(indexSchema))
	if err != nil {
		return schemas{}, fmt.Errorf("failed to read the registry index schema: %w", err)
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(schemaURL, doc); err != nil {
		return schemas{}, fmt.Errorf("failed to load the registry index schema: %w", err)
	}

	index, err := compiler.Compile(schemaURL)
	if err != nil {
		return schemas{}, fmt.Errorf("failed to compile the registry index schema: %w", err)
	}

	versionsFile, err := compiler.Compile(schemaURL + "#/$defs/versionsFile")
	if err != nil {
		return schemas{}, fmt.Errorf("failed to compile the registry plugin file schema: %w", err)
	}

	return schemas{index: index, versionsFile: versionsFile}, nil
})

// decode validates data against schema and decodes it into a T. What names the
// file that data came from. A problem is returned as an error that wraps
// ErrInvalidIndex.
func decode[T any](schema *jsonschema.Schema, data []byte, what string) (T, error) {
	var decoded T

	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return decoded, newInvalidError(what, []string{fmt.Sprintf("(root): not valid JSON: %v", err)})
	}

	if err := plugins.RejectDuplicateKeys(data); err != nil {
		return decoded, newInvalidError(what, []string{err.Error()})
	}

	if err := schema.Validate(instance); err != nil {
		problems := plugins.SchemaProblems(err)
		if len(problems) == 0 {
			return decoded, fmt.Errorf("failed to validate %s: %w", what, err)
		}

		return decoded, newInvalidError(what, problems)
	}

	if problems := rejectCaseVariants(instance, reflect.TypeFor[T]()); len(problems) > 0 {
		return decoded, newInvalidError(what, problems)
	}

	if err := json.Unmarshal(data, &decoded); err != nil {
		return decoded, newInvalidError(what, []string{fmt.Sprintf("(root): could not be decoded: %v", err)})
	}

	return decoded, nil
}
