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
		Help:     "JSON payload for the operation, as a JSON string or file path (@file.json). Can also be piped via stdin.",
	}
)

// DataJSONHandler handles the --data flag for create/update commands.
type DataJSONHandler struct {
	cli     *cli
	manager *openapi.SchemaManager
}

// NewDataJSONHandler creates a new data JSON handler.
func NewDataJSONHandler(c *cli) (*DataJSONHandler, error) {
	manager, err := openapi.NewSchemaManager()
	if err != nil {
		return nil, err
	}
	return &DataJSONHandler{
		cli:     c,
		manager: manager,
	}, nil
}

// ParseAndValidate parses JSON input and optionally validates it against the schema.
func (h *DataJSONHandler) ParseAndValidate(inputStr, method, path string, target interface{}) error {
	jsonData, err := h.readJSONInput(inputStr)
	if err != nil {
		return fmt.Errorf("failed to read JSON input: %w", err)
	}

	result, err := h.manager.ValidateRequest(method, path, jsonData)
	if err != nil {
		return fmt.Errorf("schema validation error: %w", err)
	}

	if !result.Valid {
		return fmt.Errorf("schema validation failed:\n%s", formatValidationErrors(result.Errors))
	}

	if err := json.Unmarshal(jsonData, target); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	return nil
}

// readJSONInput reads JSON from various input sources.
func (h *DataJSONHandler) readJSONInput(input string) ([]byte, error) {
	if input == "" {
		return nil, fmt.Errorf("no input provided")
	}

	if len(input) > 0 && input[0] == '@' { // @file.
		return os.ReadFile(input[1:])
	}

	return []byte(input), nil // Inline JSON.
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

// ResolveData resolves the JSON payload from --data (inline JSON or @file) or
// piped stdin; provided is false when neither is given. JSON input is a
// whole-payload alternative to the individual flags and cannot be combined with them.
func ResolveData(c *cli, cmd *cobra.Command) (payload string, provided bool, err error) {
	if HasData(cmd) {
		flagValue, _ := GetData(cmd)
		// --data takes precedence over piped stdin.
		if len(iostream.PipedInput()) > 0 {
			c.renderer.Warnf(
				"JSON data was provided via both --data and piped input. " +
					"The Auth0 CLI will use the data from --data.",
			)
		}
		return flagValue, true, nil
	}

	pipedPayload := iostream.PipedInput()
	if len(pipedPayload) == 0 {
		return "", false, nil
	}

	// Piped JSON is the whole payload; it cannot be combined with input flags.
	if conflicting := setInputFlagNames(cmd); len(conflicting) > 0 {
		return "", false, fmt.Errorf(
			"cannot combine piped JSON input with individual flags (%s); "+
				"provide the whole payload as JSON or use the flags, not both",
			strings.Join(conflicting, ", "),
		)
	}

	return string(pipedPayload), true, nil
}

// GetData gets the value of the --data flag.
func GetData(cmd *cobra.Command) (string, error) {
	return cmd.Flags().GetString("data")
}
