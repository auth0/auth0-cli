package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
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

// ReadAndValidate reads the JSON input and validates it against the schema,
// returning the raw bytes so the caller can send them to the API unchanged
// (no SDK struct round-trip that would drop fields or apply omitempty).
func (h *DataJSONHandler) ReadAndValidate(inputStr, method, path string) (json.RawMessage, error) {
	jsonData, err := h.readJSONInput(inputStr)
	if err != nil {
		return nil, validationError{err: fmt.Errorf("failed to read JSON input: %w", err)}
	}

	result, err := h.manager.ValidateRequest(method, path, jsonData)
	if err != nil {
		return nil, validationError{err: fmt.Errorf("schema validation error: %w", err)}
	}

	if !result.Valid {
		return nil, validationError{
			err:     fmt.Errorf("schema validation failed:\n%s", formatValidationErrors(result.Errors)),
			details: marshalFieldErrors(result.FieldErrors),
		}
	}

	return jsonData, nil
}

// marshalFieldErrors renders structured validation failures for the JSON error
// envelope's "details" field, returning nil (which omits the field) when there
// are none or marshaling fails.
func marshalFieldErrors(fieldErrors []openapi.FieldError) json.RawMessage {
	if len(fieldErrors) == 0 {
		return nil
	}

	raw, err := json.Marshal(fieldErrors)
	if err != nil {
		return nil
	}

	return raw
}

// readJSONInput reads JSON from various input sources.
func (h *DataJSONHandler) readJSONInput(input string) ([]byte, error) {
	if input == "" {
		return nil, fmt.Errorf("no input provided")
	}

	// "@-" and "-" read the payload from stdin, the common agent/script idiom for
	// piping a body while still passing the flag explicitly.
	if input == "@-" || input == "-" {
		data, err := iostream.PipedInput()
		if err != nil {
			return nil, err
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("no data received on stdin")
		}
		return data, nil
	}

	if input[0] == '@' { // @file.
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
func ResolveData(cmd *cobra.Command) (payload string, provided bool, err error) {
	// --data wins and is used as-is; stdin is not read, so a create/update with
	// --data never blocks on an open stdin pipe.
	if HasData(cmd) {
		flagValue, _ := GetData(cmd)
		return flagValue, true, nil
	}

	pipedPayload, err := iostream.PipedInput()
	if err != nil {
		return "", false, err
	}
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

// jsonWriteSpec describes a create/update driven by a --data JSON payload.
//
// SchemaPath MUST be the OpenAPI-keyed path template (e.g. "/actions/actions/{id}"),
// never a concrete path. The kin-openapi Paths.Find helper matches templated paths
// only when their template-variable counts are equal, so "/actions/actions/act_123"
// (0 vars) would never resolve against the stored "/actions/actions/{id}" (1 var).
// Build the actual request URL separately in URI (e.g. via cli.api.HTTPClient.URI(...)).
type jsonWriteSpec struct {
	Method     string // HTTP method, e.g. http.MethodPost / http.MethodPatch.
	SchemaPath string // OpenAPI-keyed path template used for validation + error hints.
	URI        string // Fully-qualified request URL.
	Data       string // Raw --data value (inline JSON, @file, or piped payload).
	SchemaCmd  string // Command to suggest in the "--schema" hint, e.g. "auth0 actions create".
}

// runJSONWrite validates a --data payload against the OpenAPI schema, sends it to
// the Management API verbatim (no SDK struct round-trip, like `auth0 api`), and
// returns the response decoded into *T for rendering. New resources reuse this by
// supplying their spec and the management type; the API's own semantics (e.g. PATCH
// preserving unspecified fields) apply to the exact bytes sent.
func runJSONWrite[T any](cli *cli, cmd *cobra.Command, spec jsonWriteSpec) (*T, error) {
	handler, err := NewDataJSONHandler(cli)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize JSON handler: %w", err)
	}

	payload, err := handler.ReadAndValidate(spec.Data, spec.Method, spec.SchemaPath)
	if err != nil {
		cli.renderer.Infof("Run '%s --schema' to see the expected schema.", spec.SchemaCmd)
		return nil, err
	}

	if err := ansi.Waiting(func() error {
		return cli.api.HTTPClient.Request(cmd.Context(), spec.Method, spec.URI, &payload)
	}); err != nil {
		// In JSON/agent mode the multi-line "Expected Request Schema" dump is noise
		// inside the error envelope's message; the failure is already classified as
		// validation via the 400 status and the schema stays discoverable through
		// --schema. Keep the API's own error clean for machines and enrich it with
		// the schema hint only for humans.
		if cli.wantsJSONError() {
			return nil, err
		}
		return nil, enhanceAPIError(err, spec.Method, spec.SchemaPath)
	}

	out := new(T)
	if err := json.Unmarshal(payload, out); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}
	return out, nil
}
