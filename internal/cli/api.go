package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
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
		Help:         "Query params to send with the request. Repeat the flag to send a param more than once, for example -q \"fields=a\" -q \"fields=b\".",
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
		Method         string
		URL            *url.URL
		Data           interface{}
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

Additional scopes may need to be requested during authentication step via the %s flag. For example: %s.`,
			apiDocsURL, "`GET`", "`POST`", "`--scopes`", "`auth0 login --scopes read:client_grants`",
		),
		Example: `  auth0 api get "tenants/settings"
  auth0 api "stats/daily" -q "from=20221101" -q "to=20221118"
  auth0 api delete "actions/actions/<action-id>" --force
  auth0 api clients --data "{\"name\":\"ssoTest\",\"app_type\":\"sso_integration\"}"
  cat data.json | auth0 api post clients`,
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

	return cmd
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
				cli.renderer.Infof("Sending the following request: %+v", map[string]interface{}{
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
			return newAPIResponseError(response.StatusCode, response.Header, rawBodyJSON)
		}

		if len(rawBodyJSON) == 0 {
			if cli.debug {
				cli.renderer.Infof("Response body is empty.")
			}
			return nil
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
	for _, validMethod := range apiValidMethods {
		if i.RawMethod == validMethod {
			i.Method = i.RawMethod
			return nil
		}
	}

	return fmt.Errorf(
		"invalid method given: %s, accepting only %s",
		i.RawMethod,
		strings.Join(apiValidMethods, ", "),
	)
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
			return fmt.Errorf("invalid JSON data provided: %w", err)
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
				return nil, fmt.Errorf("no data received on stdin")
			}
			return data, nil
		case strings.HasPrefix(i.RawData, "@"):
			path := i.RawData[1:]
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("failed to read data file %q: %w", path, err)
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
		return fmt.Errorf("invalid uri given: %w", err)
	}

	params := endpoint.Query()
	for _, raw := range i.RawQueryParams {
		// Split each value on commas so the historical comma-separated multi-pair
		// form (-q "from=1,to=2") still expands to multiple params. A repeated flag
		// (-q "fields=a" -q "fields=b") works too, since every occurrence is kept.
		for _, pair := range strings.Split(raw, ",") {
			key, value, found := strings.Cut(pair, "=")
			if !found {
				return fmt.Errorf("invalid query parameter %q: expected key=value", pair)
			}
			// Add (not Set) so a repeated key sends every value instead of the last
			// one overwriting the rest.
			params.Add(key, value)
		}
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

	return fmt.Errorf(
		"request failed because access token lacks scope: %s.\n "+
			"If authenticated via client credentials, add this scope to the designated client. "+
			"If authenticated as a user, request this scope during login by running `auth0 login --scopes %s`",
		recommendedScopeToAdd,
		recommendedScopeToAdd,
	)
}
