package cli

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/poppy/setup"
)

// poppyExperimentalWarning closes the help text of every Poppy command.
const poppyExperimentalWarning = "**Warning:** This command is experimental and is subject to change in future versions."

var (
	poppyCompanyDomain = Flag{
		Name:     "Company Domain",
		LongForm: "company-domain",
		Help:     "The Company's website host, such as shop.example. With several company domains, picks the one this app serves.",
	}
	poppyAPI = Flag{
		Name:     "API",
		LongForm: "api",
		Help:     "A Poppy API, by ID or identifier. Repeat for several.",
	}
	poppyAPIURL = Flag{
		Name:     "API URL",
		LongForm: "api-url",
		Help:     "Where an API's OpenAPI document lives, as <identifier>=<openapi url>. Repeat for several. Asked if the config file has no entry for the API, with <origin>/openapi.json as the default when the identifier is a URL.",
	}
	poppyApp = Flag{
		Name:     "App",
		LongForm: "app",
		Help:     "Client ID or name of the web app used for the browser join.",
	}
	poppyScope = Flag{
		Name:     "Scope",
		LongForm: "scope",
		Help:     "The description of a scope that has none, as name=\"description\". With one API, also adds a scope the API lacks. Repeat for several. Existing descriptions are never edited.",
	}
	poppyHint = Flag{
		Name:     "Hint",
		LongForm: "hint",
		Help:     "Hints for a scope, as name=changes:irreversible,personal_data:true. Repeat for several.",
	}
	poppyLoginURI = Flag{
		Name:     "Login URI",
		LongForm: "login-uri",
		Help:     "The web app's Application Login URI (initiate_login_uri): the page on your site that starts a sign-in, where the browser join sends people. Asked if the app has none.",
	}
	poppyCompanyName = Flag{
		Name:     "Company Name",
		LongForm: "company-name",
		Help:     "The Company's name for the config file. Needed when the tenant has no friendly name and the config file has no name.",
	}
	poppyOut = Flag{
		Name:     "Out",
		LongForm: "out",
		Help:     "Path of the config file.",
	}
	poppyDryRun = Flag{
		Name:     "Dry Run",
		LongForm: "dry-run",
		Help:     "Print the plan and change nothing.",
	}
	poppySkipDoctor = Flag{
		Name:     "Skip Doctor",
		LongForm: "skip-doctor",
		Help:     "Do not run the doctor checks after setup.",
	}
	poppyWebUnscoped = Flag{
		Name:     "Web Unscoped",
		LongForm: "web-unscoped",
		Help:     "Set web.unscoped in the config file: true when your website treats poppy:browser-sign-in as the account's full access on the website. auth0 poppy doctor then reminds you to check the description of poppy:browser-sign-in. Asked in a terminal when the config file has no value.",
	}
	poppyAlwaysIssueRefreshTokens = Flag{
		Name:     "Always Issue Refresh Tokens",
		LongForm: "always-issue-refresh-tokens",
		Help:     "Create a post-login Action that always issues refresh tokens to Poppy agents, deploy it, and bind it as the last Action of the post-login flow. Existing bindings keep their order. Setup never removes the Action. Needs the read:actions, create:actions and update:actions scopes. Asked in a terminal when the flag is not given. Without it, or with =false, the post-login flow is left alone.",
	}
	poppySetupNoAPI = Flag{
		Name:     "No API",
		LongForm: "no-api",
		Help:     "Set up no API: agents use the website only. Sets auth.disabled in the config file and leaves the APIs alone. Cannot be used with --api, --api-url, --scope, --hint or --no-web. In a terminal, the command asks when neither --api nor this flag is given and the config file does not have auth.disabled.",
	}
	poppySetupNoWeb = Flag{
		Name:     "No Web",
		LongForm: "no-web",
		Help:     "Set up no browser sign-in: agents use APIs only. Sets web.disabled in the config file and leaves the web app alone. Cannot be used with --app, --login-uri, --web-unscoped or --no-api. In a terminal, the command asks when neither --app nor this flag is given and the config file does not have web.disabled.",
	}
	poppyConfigNoAPI = Flag{
		Name:     "No API",
		LongForm: "no-api",
		Help:     "Leave the APIs out: agents use the website only. Sets auth.disabled in the config file. Cannot be used with --api, --api-url, --hint or --no-web. In a terminal, the command asks when neither --api nor this flag is given, the config file does not have auth.disabled, and the tenant has a Poppy API.",
	}
	poppyConfigNoWeb = Flag{
		Name:     "No Web",
		LongForm: "no-web",
		Help:     "Leave the browser sign-in out: agents use APIs only. Sets web.disabled in the config file. Cannot be used with --app, --web-unscoped or --no-api. In a terminal, the command asks when neither --app nor this flag is given, the config file does not have web.disabled, and an app has the browser join on.",
	}
	poppyOverwriteConfig = Flag{
		Name:     "Overwrite Config",
		LongForm: "overwrite-config",
		Help:     "Overwrite a config file that belongs to a different issuer.",
	}
	poppyConfig = Flag{
		Name:     "Config",
		LongForm: "config",
		Help:     "Config file to check. Used only if it exists. Without this flag, and without the default file, the poppy.json your website serves is checked instead.",
	}
)

func poppyCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "poppy",
		Short: "Set up your company for personal agents",
		Long: poppyLong("Prepares your company for personal agents, using the Personal Agent Protocol (Poppy).\n\n" +
			"Poppy settings are spread across the tenant, an API and a web app, and the discovery files on your website must repeat Auth0's values exactly.\n\n" +
			"These commands set that up, write one config file (`poppy.json`) for your web and API SDKs, and check the whole chain before an agent arrives."),
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(poppySetupCmd(cli))
	cmd.AddCommand(poppyConfigCmd(cli))
	cmd.AddCommand(poppyDoctorCmd(cli))

	return cmd
}

// poppyLong marks a command's help text as experimental.
func poppyLong(description string) string {
	return "(Experimental) " + description + "\n\n" + poppyExperimentalWarning
}

// registerPoppyOutputFlags adds the machine-readable output flags.
func registerPoppyOutputFlags(cmd *cobra.Command, cli *cli) {
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")
}

// checkPoppyScopes fails before any change when the saved login lacks a scope
// the command needs. In env auth mode the saved login is not the one in use, so
// it is not consulted, and neither is it when there is none to inspect: the
// scope error of the API itself is mapped instead (see poppyError).
func (c *cli) checkPoppyScopes(required []string) error {
	if envAuthEnabled(os.Getenv) {
		return nil
	}

	tenant, err := c.Config.GetTenant(c.tenant)
	if err != nil {
		return nil
	}

	granted := scopesFromToken(tenant.GetAccessToken())
	if granted == nil {
		return nil
	}

	if missing := missingScopes(granted, required); len(missing) > 0 {
		return authError{
			err:    fmt.Errorf("missing scope %s. Run: auth0 login --scopes %s", strings.Join(missing, ", "), strings.Join(missing, ",")),
			reason: "missing_scopes",
		}
	}

	return nil
}

// poppyUnavailableError reports a 403 that is not a missing scope: the tenant
// does not have a feature the Poppy commands need.
type poppyUnavailableError struct {
	err   error
	debug bool
}

func (e poppyUnavailableError) Error() string {
	message := "the tenant may lack Agent as Principal or the Poppy beta; contact support"
	if e.debug {
		message += fmt.Sprintf(" (status %d)", http.StatusForbidden)
	}

	return message
}

func (e poppyUnavailableError) Unwrap() error { return e.err }

// forbiddenScopeError maps a 403 from the Management API that names a missing scope
// to the login command that adds it. It returns nil for a 403 of another kind,
// and forbidden tells whether the error was a 403 at all.
func forbiddenScopeError(err error) (forbidden bool, scopeErr error) {
	var apiErr *core.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusForbidden {
		return false, nil
	}

	var body []byte
	if inner := apiErr.Unwrap(); inner != nil {
		body = []byte(inner.Error())
	}

	return true, isInsufficientScopeError(apiErr.StatusCode, body)
}

// poppyError maps a 403 from the Management API to the login command for a
// missing scope, or to the tenant lacking a Poppy feature.
func (c *cli) poppyError(err error) error {
	var setupPartial *setup.PartialError
	if errors.As(err, &setupPartial) {
		setupPartial.Err = c.poppyError(setupPartial.Err)

		return err
	}

	forbidden, scopeErr := forbiddenScopeError(err)
	switch {
	case !forbidden:
		return err
	case scopeErr != nil:
		return scopeErr
	default:
		return poppyUnavailableError{err: err, debug: c.debug}
	}
}

// poppyInputError is a problem with the inputs, with the message on one line. What
// to do about it is not part of the message: it goes to the details of the JSON
// error, and to the hint that a person reads under the message.
type poppyInputError struct {
	input *setup.InputError
}

func (e poppyInputError) Error() string { return e.input.Message }

func (e poppyInputError) Unwrap() error { return e.input }

// poppyInputFailure turns an input error of the setup package into a CLI error that
// classifies as a validation failure with the reason of the input error.
func poppyInputFailure(input *setup.InputError) error {
	var err error = poppyInputError{input: input}
	if input.Fix == "" && len(input.Candidates) == 0 {
		return validationError{err: err, reason: input.Reason}
	}

	details, _ := json.Marshal(struct {
		Fix        string   `json:"fix,omitempty"`
		Candidates []string `json:"candidates,omitempty"`
	}{Fix: input.Fix, Candidates: input.Candidates})

	if input.Fix != "" {
		err = apiResponseHintError{err: err, hint: poppyFixHint(input.Fix)}
	}

	return validationError{err: err, details: details, reason: input.Reason}
}

// poppyFixHint is the hint line that follows the message of an error that says what
// to do about it. A fix over several lines keeps its lines under the first.
func poppyFixHint(fix string) string {
	const indent = "\n           "

	lines := strings.Split(fix, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}

	return "FIX  " + strings.Join(lines, indent)
}

// poppyMissingFlagsError ends a command that cannot ask with one error that names
// every flag still needed, with why, and an example of the command. The questions
// give the flags. The example is the command the user is running, as the inputs
// rebuild it, with a placeholder for each flag that is missing.
func poppyMissingFlagsError(in setup.Inputs, opts setup.CommandOptions, questions []setup.Question) usageError {
	type missing struct {
		Flag   string `json:"flag"`
		Reason string `json:"reason"`
	}

	names := make([]string, len(questions))
	flags := make([]missing, len(questions))
	example := setup.Command(in, opts)

	for i, question := range questions {
		names[i] = question.Flag
		flags[i] = missing{Flag: question.Flag, Reason: cmp.Or(question.Missing, question.Reason)}

		// A flag that spells out its value, such as --api-url <identifier>=<url>, is used as it is.
		if strings.Contains(question.Flag, " ") {
			example = append(example, question.Flag)
		} else {
			example = append(example, question.Flag+" <"+strings.ToLower(question.Prompt)+">")
		}
	}

	details, _ := json.Marshal(struct {
		Flags   []missing `json:"flags"`
		Example string    `json:"example"`
	}{Flags: flags, Example: strings.Join(example, " ")})

	return usageError{
		err:     errors.New("missing required flags: " + strings.Join(names, ", ")),
		details: details,
		reason:  "missing_required_flags",
	}
}

// poppyMissingValuesError turns the values that setup or config could not fill
// into the flags that give them. A value no flag sets (the description of an API,
// which comes from its name) is named in the message, with where to set it.
func poppyMissingValuesError(missing *config.MissingFieldsError, in setup.Inputs, opts setup.CommandOptions) error {
	questions, unset := setup.QuestionsForMissing(missing)

	if len(questions) > 0 {
		err := poppyMissingFlagsError(in, opts, questions)
		if len(unset) > 0 {
			err.err = fmt.Errorf("%w. No flag sets %s: %s", err.err, strings.Join(unset, ", "), poppyUnsetFix)
		}

		return err
	}

	return validationError{
		err:    fmt.Errorf("%w. No flag sets %s: %s", missing, strings.Join(unset, ", "), poppyUnsetFix),
		reason: "missing_config_values",
	}
}

// poppyUnsetFix says where a value that no flag sets comes from.
const poppyUnsetFix = "the description of an API is its name in Auth0, so give the API a name, or add the description to the apis entry of the config file"
