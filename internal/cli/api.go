package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/iostream"
	"github.com/auth0/auth0-cli/internal/prompt"
)

const apiDocsURL = "https://auth0.com/docs/api/management/v2"

var apiFlags = apiCmdFlags{
	Data: Flag{
		Name:         "RawData",
		LongForm:     "data",
		ShortForm:    "d",
		Help:         "JSON data payload to send with the request. Pass inline JSON, @file to read from a file, or @- to read from stdin. Data can also be piped in instead of using this flag.",
		IsRequired:   false,
		AlwaysPrompt: false,
	},
	QueryParams: Flag{
		Name:         "QueryParams",
		LongForm:     "query",
		ShortForm:    "q",
		Help:         "Query params to send with the request. A comma-separated value is sent as a single param, for example -q \"fields=a,b,c\". Repeat the flag to send a param more than once, for example -q \"fields=a\" -q \"fields=b\".",
		IsRequired:   false,
		AlwaysPrompt: false,
	},
}

var apiValidMethods = []string{
	http.MethodGet,
	http.MethodPost,
	http.MethodPut,
	http.MethodPatch,
	http.MethodDelete,
}

type (
	apiCmdFlags struct {
		Data        Flag
		QueryParams Flag
	}

	apiCmdInputs struct {
		renderer *display.Renderer

		RawMethod      string
		RawURI         string
		RawData        string
		RawQueryParams []string
		RevealSecrets  bool
		Method         string
		URL            *url.URL
		Data           any
	}
)

func apiCmd(cli *cli) *cobra.Command {
	inputs := apiCmdInputs{
		renderer: cli.renderer,
	}

	cmd := &cobra.Command{
		Use:   "api <method> <url-path>",
		Args:  cobra.RangeArgs(0, 2),
		Short: "Makes an authenticated HTTP request to the Auth0 Management API",
		Long: fmt.Sprintf(
			`Makes an authenticated HTTP request to the [Auth0 Management API](%s) and returns the response as JSON.

Method argument is optional, defaults to %s for requests without data and %s for requests with data.

Additional scopes may need to be requested during authentication step via the %s flag. For example: %s.

Like the typed commands, secrets such as %s and %s are removed from the response unless you pass %s.`,
			apiDocsURL, "`GET`", "`POST`", "`--scopes`", "`auth0 login --scopes read:client_grants`",
			"`client_secret`", "`signing_keys`", "`--reveal-secrets`",
		),
		Example: `  auth0 api get "tenants/settings"
  auth0 api "stats/daily" -q "from=20221101" -q "to=20221118"
  auth0 api "clients" -q "fields=name,app_type,callbacks"
  auth0 api delete "actions/actions/<action-id>" --force
  auth0 api clients --data "{\"name\":\"ssoTest\",\"app_type\":\"sso_integration\"}"
  cat data.json | auth0 api post clients
  auth0 api get "clients/<client-id>" --reveal-secrets`,
		RunE: apiCmdRun(cli, &inputs),
	}

	cmd.SetUsageTemplate(apiUsageTemplate())
	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation when using the delete method.")
	// The response is always JSON. --json keeps the pretty, colorized default and
	// --json-compact emits a single dense line so an NDJSON reader gets one record
	// per line. There is no --csv, since an arbitrary API response is not tabular.
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")
	apiFlags.Data.RegisterString(cmd, &inputs.RawData, "")
	apiFlags.QueryParams.RegisterStringArray(cmd, &inputs.RawQueryParams, nil)
	revealSecrets.RegisterBool(cmd, &inputs.RevealSecrets, false)

	return cmd
}

// apiSecretKeys are the response fields removed from `auth0 api` output unless
// --reveal-secrets is set. They match what the typed `apps` commands hide.
var apiSecretKeys = []string{"client_secret", "signing_keys"}

// maskAPISecrets removes secret fields, at any depth, from a JSON response body.
// A body without them is returned untouched, so key order and formatting only
// change when something was actually removed.
func maskAPISecrets(rawBodyJSON []byte) ([]byte, bool, error) {
	found := false
	for _, key := range apiSecretKeys {
		if bytes.Contains(rawBodyJSON, []byte(`"`+key+`"`)) {
			found = true
			break
		}
	}
	if !found {
		return rawBodyJSON, false, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(rawBodyJSON))
	decoder.UseNumber()

	var document any
	if err := decoder.Decode(&document); err != nil {
		// Not a JSON document we can walk (the caller prints it as is).
		return rawBodyJSON, false, nil
	}

	if !removeAPISecretKeys(document) {
		return rawBodyJSON, false, nil
	}

	var masked bytes.Buffer
	encoder := json.NewEncoder(&masked)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(document); err != nil {
		return nil, false, fmt.Errorf("failed to mask secrets in the response: %w", err)
	}

	return bytes.TrimSpace(masked.Bytes()), true, nil
}

func removeAPISecretKeys(value any) bool {
	removed := false

	switch typed := value.(type) {
	case map[string]any:
		for _, key := range apiSecretKeys {
			if _, ok := typed[key]; ok {
				delete(typed, key)
				removed = true
			}
		}
		for _, child := range typed {
			if removeAPISecretKeys(child) {
				removed = true
			}
		}
	case []any:
		for _, child := range typed {
			if removeAPISecretKeys(child) {
				removed = true
			}
		}
	}

	return removed
}

// formatAPIResponse renders a raw JSON response body for stdout. Under
// --json-compact it emits a single dense line with no color so an NDJSON reader
// gets one record per line; otherwise it returns a 2-space-indented, colorized
// document for a human. The trailing newline is added by
// renderer.OutputPreformattedJSON.
func formatAPIResponse(format display.OutputFormat, rawBodyJSON []byte) (string, error) {
	if format == display.OutputFormatJSONCompact {
		var compactJSON bytes.Buffer
		if err := json.Compact(&compactJSON, rawBodyJSON); err != nil {
			return "", fmt.Errorf("failed to prepare json output: %w", err)
		}

		return compactJSON.String(), nil
	}

	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, rawBodyJSON, "", "  "); err != nil {
		return "", fmt.Errorf("failed to prepare json output: %w", err)
	}

	return ansi.ColorizeJSON(prettyJSON.String()), nil
}

func apiUsageTemplate() string {
	return fmt.Sprintf(
		`%s
  %s

%s
  %s

%s`,
		ansi.Bold("Auth0 Management API Docs:"),
		apiDocsURL,
		ansi.Bold("Available Methods:"),
		strings.ToLower(strings.Join(apiValidMethods, ", ")),
		resourceUsageTemplate(),
	)
}

func apiCmdRun(cli *cli, inputs *apiCmdInputs) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}

		if err := inputs.fromArgs(args, cli.tenant); err != nil {
			return fmt.Errorf("failed to parse command inputs: %w", err)
		}

		if suggestion := suggestTypedCommand(inputs.Method, inputs.RawURI); suggestion != "" {
			cli.renderer.Warnf(
				"This endpoint has a dedicated command: `%s`. "+
					"It offers schema discovery (--schema), input validation (--data), and structured output (--json). "+
					"Running the raw request anyway.",
				suggestion,
			)
		}

		if inputs.Method == http.MethodDelete && !cli.force && cli.agentMode {
			return errDestructiveNoConfirm
		}

		if inputs.Method == http.MethodDelete && !cli.force && canPrompt(cmd) {
			message := "Are you sure you want to proceed? Deleting is a destructive action."
			if confirmed := prompt.Confirm(message); !confirmed {
				return nil
			}
		}

		var response *http.Response
		if err := ansi.Waiting(func() error {
			request, err := cli.api.HTTPClient.NewRequest(
				cmd.Context(),
				inputs.Method,
				inputs.URL.String(),
				inputs.Data,
			)
			if err != nil {
				return err
			}

			if cli.debug {
				cli.renderer.Infof("Sending the following request: %+v", map[string]any{
					"method":  request.Method,
					"url":     request.URL.String(),
					"payload": inputs.Data,
				})
			}

			response, err = cli.api.HTTPClient.Do(request)
			return err
		}); err != nil {
			return fmt.Errorf("failed to send request: %w", err)
		}
		defer func() {
			_ = response.Body.Close()
		}()

		rawBodyJSON, err := io.ReadAll(response.Body)
		if err != nil {
			return err
		}

		// Read the body once above, then classify from the bytes. Decoding the
		// stream here would drain it, so a 403 that is not insufficient-scope would
		// otherwise lose its real error body and fall back to bare "Forbidden".
		if err := isInsufficientScopeError(response.StatusCode, rawBodyJSON); err != nil {
			return err
		}

		if response.StatusCode >= http.StatusBadRequest {
			err := newAPIResponseError(response.StatusCode, response.Header, rawBodyJSON)
			if isWrongMethod404(inputs.Method, response.StatusCode, rawBodyJSON) {
				return apiResponseHintError{err: err, hint: apiWrongMethodHintText}
			}
			return err
		}

		if len(rawBodyJSON) == 0 {
			if cli.debug {
				cli.renderer.Infof("Response body is empty.")
			}
			return nil
		}

		if !inputs.RevealSecrets {
			masked, didMask, err := maskAPISecrets(rawBodyJSON)
			if err != nil {
				return err
			}
			if didMask {
				rawBodyJSON = masked
				cli.renderer.Infof("Secrets were removed from the response. Use --reveal-secrets to include them.")
			}
		}

		output, err := formatAPIResponse(cli.renderer.Format, rawBodyJSON)
		if err != nil {
			return err
		}

		cli.renderer.OutputPreformattedJSON(output)

		return nil
	}
}

func (i *apiCmdInputs) fromArgs(args []string, domain string) error {
	i.parseRaw(args)

	if err := i.validateAndSetMethod(); err != nil {
		return err
	}

	if err := i.validateAndSetData(); err != nil {
		return err
	}

	return i.validateAndSetEndpoint(domain)
}

func (i *apiCmdInputs) validateAndSetMethod() error {
	if slices.Contains(apiValidMethods, i.RawMethod) {
		i.Method = i.RawMethod
		return nil
	}

	return usageError{
		err: fmt.Errorf(
			"invalid method given: %s, accepting only %s",
			i.RawMethod,
			strings.Join(apiValidMethods, ", "),
		),
		reason: "invalid_flag_value",
	}
}

func (i *apiCmdInputs) validateAndSetData() error {
	if i.Method == http.MethodGet {
		return nil
	}

	data, err := i.resolveData()
	if err != nil {
		return err
	}

	if len(data) > 0 {
		if err := json.Unmarshal(data, &i.Data); err != nil {
			return validationError{err: fmt.Errorf("invalid JSON data provided: %w", err), reason: "malformed_json"}
		}
	}

	return nil
}

// resolveData returns the request body bytes. The --data flag takes precedence
// and accepts inline JSON, @file to read from a file, or @- (or -) to read from
// stdin. When --data is unset the body is read from a stdin pipe. A --data value
// used as-is never blocks on an open, EOF-less pipe (the common agent/CI case),
// because stdin is only touched for the explicit @-/- form.
func (i *apiCmdInputs) resolveData() ([]byte, error) {
	if i.RawData != "" {
		switch {
		case i.RawData == "@-" || i.RawData == "-":
			data, err := iostream.PipedInput()
			if err != nil {
				return nil, err
			}
			if len(data) == 0 {
				return nil, validationError{err: fmt.Errorf("no data received on stdin"), reason: "missing_input"}
			}
			return data, nil
		case strings.HasPrefix(i.RawData, "@"):
			path := i.RawData[1:]
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, usageError{err: fmt.Errorf("failed to read data file %q: %w", path, err), reason: "invalid_flag_value"}
			}
			return data, nil
		default:
			return []byte(i.RawData), nil
		}
	}

	return iostream.PipedInput()
}

func (i *apiCmdInputs) validateAndSetEndpoint(domain string) error {
	endpoint, err := url.Parse(fmt.Sprintf("https://%s/api/v2/%s", domain, strings.Trim(i.RawURI, "/")))
	if err != nil {
		return usageError{err: fmt.Errorf("invalid uri given: %w", err), reason: "invalid_flag_value"}
	}

	// The server resolves "." and ".." segments (including percent-encoded ones,
	// which endpoint.Path holds decoded), so "../../oauth/token" would send the
	// Management API token outside /api/v2. Reject any URI that escapes it.
	if cleaned := path.Clean(endpoint.Path); cleaned != "/api/v2" && !strings.HasPrefix(cleaned, "/api/v2/") {
		return usageError{
			err:    fmt.Errorf("invalid uri given: %q resolves outside of the Management API (/api/v2)", i.RawURI),
			reason: "invalid_flag_value",
		}
	}

	params := endpoint.Query()
	for _, raw := range i.RawQueryParams {
		// Each -q value is exactly one query param, split once on the first "=".
		// Commas are kept literally as part of the value, so a comma-separated
		// list is sent as a single param (-q "fields=a,b,c" → fields=a,b,c),
		// which is how the Management API expects `fields`, `include_fields`, and
		// similar. A value may itself contain "=" (-q "q=name=John,city=NY"), and
		// repeating the flag sends multiple params (-q "from=1" -q "to=2").
		key, value, found := strings.Cut(raw, "=")
		if !found || key == "" {
			return usageError{err: fmt.Errorf("invalid query parameter %q: expected key=value", raw), reason: "invalid_flag_value"}
		}
		// Add (not Set) so a repeated key sends every value instead of the last
		// one overwriting the rest.
		params.Add(key, value)
	}
	endpoint.RawQuery = params.Encode()

	i.URL = endpoint

	return nil
}

func (i *apiCmdInputs) parseRaw(args []string) {
	lenArgs := len(args)
	if lenArgs == 1 {
		// A bare single-argument call defaults to GET and never reads stdin, so an
		// agent or CI run with an inherited, open, EOF-less stdin pipe cannot block.
		// POST is inferred only from an explicit --data value; to send a piped body
		// give a method (cat data.json | auth0 api post clients) or use --data @-.
		i.RawMethod = http.MethodGet
		if i.RawData != "" {
			i.RawMethod = http.MethodPost
		}
	} else {
		i.RawMethod = strings.ToUpper(args[0])
	}

	i.RawURI = args[lenArgs-1]
}

// newAPIResponseError turns non-2xx `auth0 api` responses into SDK management errors.
// This keeps `error_class` handling consistent with typed SDK commands.
func newAPIResponseError(statusCode int, header http.Header, body []byte) error {
	message := strings.TrimSpace(string(body))
	if message == "" {
		message = http.StatusText(statusCode)
	}

	return core.NewAPIError(statusCode, header, fmt.Errorf("API request failed: %s", message))
}

// isWrongMethod404 reports whether a PATCH likely used the wrong verb. Many
// Management API endpoints accept PUT but reject PATCH with a bare "Not Found"
// instead of a 405, so it is limited to PATCH and to the generic routing-miss
// body (empty, or "Not Found" with no errorCode); a resource-specific 404 or any
// other verb returns false.
func isWrongMethod404(method string, statusCode int, rawBody []byte) bool {
	if statusCode != http.StatusNotFound || method != http.MethodPatch {
		return false
	}

	if len(bytes.TrimSpace(rawBody)) == 0 {
		return true
	}

	var body struct {
		Message   string `json:"message"`
		ErrorCode string `json:"errorCode"`
	}
	if err := json.Unmarshal(rawBody, &body); err != nil {
		return false
	}

	message := strings.TrimSpace(body.Message)
	return body.ErrorCode == "" && (message == "" || strings.EqualFold(message, "Not Found"))
}

const apiWrongMethodHintText = "The endpoint returned a generic 404. If the path is correct, the HTTP method may " +
	"not be supported there — many Auth0 Management API endpoints accept PUT but not PATCH. " +
	"Verify both the path and the method."

// apiResponseHintError attaches an actionable hint to a non-2xx `auth0 api`
// response. Error() stays the terse API message so the JSON envelope's message
// field is clean; the hint travels in the envelope's details via ErrorDetails and
// is printed separately by renderErrorMessage. It unwraps to the underlying error
// so status classification is unchanged.
type apiResponseHintError struct {
	err  error
	hint string
}

func (e apiResponseHintError) Error() string { return e.err.Error() }

func (e apiResponseHintError) Unwrap() error { return e.err }

func (e apiResponseHintError) ErrorDetails() json.RawMessage {
	raw, err := json.Marshal(struct {
		Hint string `json:"hint"`
	}{Hint: e.hint})
	if err != nil {
		return nil
	}

	return raw
}

func isInsufficientScopeError(statusCode int, rawBody []byte) error {
	if statusCode != 403 {
		return nil
	}

	type ErrorBody struct {
		ErrorCode string `json:"errorCode"`
		Message   string `json:"message"`
	}

	var body ErrorBody
	if err := json.Unmarshal(rawBody, &body); err != nil {
		return nil
	}

	if body.ErrorCode != "insufficient_scope" {
		return nil
	}

	var recommendedScopeToAdd string

	parts := strings.Split(body.Message, "Insufficient scope, expected any of: ")
	if len(parts) > 1 {
		missingScopes := parts[1]
		recommendedScopeToAdd = strings.Split(missingScopes, ",")[0]
	} else {
		re := regexp.MustCompile(`scope: ([\w:,_\s]+)`)
		matches := re.FindStringSubmatch(body.Message)
		if len(matches) > 1 {
			recommendedScopeToAdd = matches[1]
		}
	}

	return authError{
		err: fmt.Errorf(
			"request failed because access token lacks scope: %s.\n "+
				"If authenticated via client credentials, add this scope to the designated client. "+
				"If authenticated as a user, request this scope during login by running `auth0 login --scopes %s`",
			recommendedScopeToAdd,
			recommendedScopeToAdd,
		),
		reason: "insufficient_scope",
	}
}
