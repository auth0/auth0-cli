package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/buildinfo"
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
		Help:         "JSON data payload to send with the request. Data can be piped in as well instead of using this flag.",
		IsRequired:   false,
		AlwaysPrompt: false,
	},
	QueryParams: Flag{
		Name:         "QueryParams",
		LongForm:     "query",
		ShortForm:    "q",
		Help:         "Query params to send with the request.",
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
		RawQueryParams map[string]string
		Method         string
		URL            *url.URL
		Data           io.Reader
	}
)

func apiCmd(cli *cli) *cobra.Command {
	inputs := apiCmdInputs{
		renderer: cli.renderer,
	}

	cmd := &cobra.Command{
		Use:   "api <method> <url-path>",
		Args:  cobra.RangeArgs(1, 2),
		Short: "Makes an authenticated HTTP request to the Auth0 Management API",
		Long: fmt.Sprintf(
			`Makes an authenticated HTTP request to the Auth0 Management API and prints the response as JSON.

The method argument is optional, and when you don’t specify it, the command defaults to GET for requests without data and POST for requests with data.

%s  %s

%s  %s`,
			"Auth0 Management API Docs:\n", apiDocsURL,
			"Available Methods:\n", strings.ToLower(strings.Join(apiValidMethods, ", ")),
		),
		Example: `auth0 api "stats/daily" -q "from=20221101" -q "to=20221118"
auth0 api get "tenants/settings"
auth0 api clients --data "{\"name\":\"ssoTest\",\"app_type\":\"sso_integration\"}"
cat data.json | auth0 api post clients`,
		RunE: apiCmdRun(cli, &inputs),
	}

	cmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		command.Flags().MarkHidden("json")
		command.Parent().HelpFunc()(command, strings)
	})

	apiFlags.Data.RegisterString(cmd, &inputs.RawData, "")
	apiFlags.QueryParams.RegisterStringMap(cmd, &inputs.RawQueryParams, nil)

	return cmd
}

func apiCmdRun(cli *cli, inputs *apiCmdInputs) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := inputs.fromArgs(args, cli.tenant); err != nil {
			return fmt.Errorf("failed to parse command inputs: %w", err)
		}

		if inputs.Method == http.MethodDelete && !cli.force && canPrompt(cmd) {
			message := "Are you sure you want to proceed? Deleting is a destructive action."
			if confirmed := prompt.Confirm(message); !confirmed {
				return nil
			}
		}

		var response *http.Response
		if err := ansi.Waiting(func() error {
			request, err := http.NewRequestWithContext(
				cmd.Context(),
				inputs.Method,
				inputs.URL.String(),
				inputs.Data,
			)
			if err != nil {
				return err
			}

			bearerToken := cli.config.Tenants[cli.tenant].AccessToken
			request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", bearerToken))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("User-Agent", fmt.Sprintf("%s/%s", userAgent, strings.TrimPrefix(buildinfo.Version, "v")))

			if cli.debug {
				cli.renderer.Infof("[%s]: %s", request.Method, request.URL.String())
			}

			response, err = http.DefaultClient.Do(request)
			return err
		}); err != nil {
			return fmt.Errorf("failed to send request: %w", err)
		}
		defer response.Body.Close()

		rawBodyJSON, err := io.ReadAll(response.Body)
		if err != nil {
			return err
		}

		if len(rawBodyJSON) == 0 {
			if cli.debug {
				cli.renderer.Infof("Response body is empty.")
			}
			return nil
		}

		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, rawBodyJSON, "", "  "); err != nil {
			return fmt.Errorf("failed to prepare json output: %w", err)
		}

		cli.renderer.Output(ansi.ColorizeJSON(prettyJSON.String(), false))

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
	var data []byte

	if i.RawData != "" {
		data = []byte(i.RawData)
	}

	pipedRawData := iostream.PipedInput()
	if len(pipedRawData) > 0 && data == nil {
		data = pipedRawData
	}

	if len(pipedRawData) > 0 && len(i.RawData) > 0 {
		i.renderer.Warnf(
			"JSON data was passed using both the flag and as piped input. " +
				"The Auth0 CLI will use only the data from the flag.",
		)
	}

	if len(data) > 0 && !json.Valid(data) {
		return fmt.Errorf("invalid json data given: %s", data)
	}

	i.Data = bytes.NewReader(data)

	return nil
}

func (i *apiCmdInputs) validateAndSetEndpoint(domain string) error {
	endpoint, err := url.Parse(fmt.Sprintf("https://%s/api/v2/%s", domain, strings.Trim(i.RawURI, "/")))
	if err != nil {
		return fmt.Errorf("invalid uri given: %w", err)
	}

	params := endpoint.Query()
	for key, value := range i.RawQueryParams {
		params.Set(key, value)
	}
	endpoint.RawQuery = params.Encode()

	i.URL = endpoint

	return nil
}

func (i *apiCmdInputs) parseRaw(args []string) {
	lenArgs := len(args)
	if lenArgs == 1 {
		i.RawMethod = http.MethodGet
		if i.RawData != "" {
			i.RawMethod = http.MethodPost
		}
	} else {
		i.RawMethod = strings.ToUpper(args[0])
	}

	i.RawURI = args[lenArgs-1]
}
