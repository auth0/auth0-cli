package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/iostream"
	"github.com/auth0/auth0-cli/internal/openapi"
)

var (
	dataFlag = Flag{
		Name:     "Data",
		LongForm: "data",
		Help:     "JSON payload for the operation. Can be a JSON string, file path (@file.json), or '-' for stdin.",
	}
)

// InputJSONHandler handles the --data flag for create/update commands.
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
	lines := make([]string, len(errors))
	for i, err := range errors {
		lines[i] = fmt.Sprintf("%d. %s", i+1, err)
	}
	return strings.Join(lines, "\n")
}

// HasData checks if the --data flag is set.
func HasData(cmd *cobra.Command) bool {
	flag := cmd.Flags().Lookup("data")
	return flag != nil && flag.Changed
}

// ResolveData returns the request payload from --data or, failing that, piped
// stdin; provided is false when neither is present, so the caller can prompt.
func ResolveData(cmd *cobra.Command) (payload string, provided bool, err error) {
	if HasData(cmd) {
		flagValue, _ := GetData(cmd)
		return flagValue, true, nil
	}

	pipedPayload := iostream.PipedInput()
	if len(pipedPayload) == 0 {
		return "", false, nil
	}

	// A stdin payload obeys the same rule as --data — no granular input flags —
	// but MarkFlagsMutuallyExclusive can't see stdin, so enforce it here.
	if conflicting := setInputFlagNames(cmd); len(conflicting) > 0 {
		return "", false, fmt.Errorf(
			"cannot combine piped JSON input with input flags (%s); "+
				"provide the whole payload via stdin or use the flags, not both",
			strings.Join(conflicting, ", "),
		)
	}

	return string(pipedPayload), true, nil
}

// GetData gets the value of the --data flag.
func GetData(cmd *cobra.Command) (string, error) {
	return cmd.Flags().GetString("data")
}
