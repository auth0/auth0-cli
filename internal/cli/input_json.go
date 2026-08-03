package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/iostream"
	"github.com/auth0/auth0-cli/internal/openapi"
)

var (
	inputJSON = Flag{
		Name:      "InputJSON",
		LongForm:  "input-json",
		ShortForm: "j",
		Help:      "JSON input for the operation. Can be a JSON string, file path (@file.json), or '-' for stdin.",
	}
)

// InputJSONHandler handles --input-json flag for create/update commands.
type InputJSONHandler struct {
	cli     *cli
	manager *openapi.SchemaManager
}

// NewInputJSONHandler creates a new input JSON handler.
func NewInputJSONHandler(c *cli) (*InputJSONHandler, error) {
	manager, err := openapi.NewSchemaManager()
	if err != nil {
		return nil, err
	}
	return &InputJSONHandler{
		cli:     c,
		manager: manager,
	}, nil
}

// ParseAndValidate parses JSON input and optionally validates it against the schema.
func (h *InputJSONHandler) ParseAndValidate(inputStr, method, path string, target interface{}) error {
	// Read JSON data.
	jsonData, err := h.readJSONInput(inputStr)
	if err != nil {
		return fmt.Errorf("failed to read JSON input: %w", err)
	}

	// Validate against schema.
	result, err := h.manager.ValidateRequest(method, path, jsonData)
	if err != nil {
		return fmt.Errorf("schema validation error: %w", err)
	}

	if !result.Valid {
		return fmt.Errorf("schema validation failed:\n%s", formatValidationErrors(result.Errors))
	}

	// Unmarshal into target.
	if err := json.Unmarshal(jsonData, target); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	return nil
}

// ParseWithoutValidation parses JSON input without schema validation.
// Useful when you want to accept any valid JSON.
func (h *InputJSONHandler) ParseWithoutValidation(inputStr string, target interface{}) error {
	jsonData, err := h.readJSONInput(inputStr)
	if err != nil {
		return fmt.Errorf("failed to read JSON input: %w", err)
	}

	if err := json.Unmarshal(jsonData, target); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	return nil
}

// readJSONInput reads JSON from various input sources.
func (h *InputJSONHandler) readJSONInput(input string) ([]byte, error) {
	if input == "" {
		return nil, fmt.Errorf("no input provided")
	}

	// Check if it's stdin.
	if input == "-" {
		return iostream.PipedInput(), nil
	}

	// Check if it's a file path (starts with @).
	if len(input) > 0 && input[0] == '@' {
		filePath := input[1:]
		return os.ReadFile(filePath)
	}

	// Otherwise, treat it as inline JSON.
	return []byte(input), nil
}

// formatValidationErrors formats validation errors in a user-friendly way.
func formatValidationErrors(errors []string) string {
	if len(errors) == 0 {
		return ""
	}

	result := ""
	for i, err := range errors {
		result += fmt.Sprintf("%d. %s\n", i+1, err)
	}
	return result
}

// HasInputJSON checks if the --input-json flag is set.
func HasInputJSON(cmd *cobra.Command) bool {
	flag := cmd.Flags().Lookup("input-json")
	return flag != nil && flag.Changed
}

// GetInputJSON gets the value of the --input-json flag.
func GetInputJSON(cmd *cobra.Command) (string, error) {
	return cmd.Flags().GetString("input-json")
}
