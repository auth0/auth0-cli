package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/poppy/setup"
	"github.com/auth0/auth0-cli/internal/text"
)

// poppySetupCommandOptions are the options that rebuild the setup command being
// run, for the example in an error and for the line that applies a plan.
func (c *cli) poppySetupCommandOptions() setup.CommandOptions {
	return setup.CommandOptions{Tenant: c.tenant, Yes: c.force}
}

// poppyWriteScopes are the Management API scopes that setup may need to change the
// tenant. They are checked before the first question, unless the run is a dry run.
var poppyWriteScopes = []string{"update:tenant_settings", "update:resource_servers", "update:clients"}

type poppySetupOptions struct {
	DryRun bool
}

func poppySetupCmd(cli *cli) *cobra.Command {
	var (
		flags       setup.Flags
		opts        poppySetupOptions
		webUnscoped bool
	)

	cmd := &cobra.Command{
		Use:   "setup",
		Args:  cobra.NoArgs,
		Short: "Turn on Poppy for the tenant, an API and a web app",
		Long: poppyLong("Turns on the Poppy settings for the tenant, an existing API and an existing web app, and writes the config file.\n\n" +
			"The API and the web app must already exist. Their scopes, MFA policy and Actions are left alone, except that setup adds the scope `poppy:browser-sign-in` to each API that lacks it.\n\n" +
			"The command shows a plan first and marks the tenant-wide changes, then applies it once you confirm. Use `--dry-run` to see the plan only, and `--force` to skip the question, which is required without a terminal.\n\n" +
			"In a terminal, it lets you pick the API and the web app, and asks for the values that cannot be listed, such as the OpenAPI URL of an API, the Application Login URI of the web app, and the description of each scope that has none. Without a terminal, it names every missing flag at once.\n\n" +
			"Running it again changes nothing when the tenant already matches, or continues where a failed run stopped."),
		Example: `  auth0 poppy setup
  auth0 poppy setup --company-domain shop.example --api https://api.shop.example --app "Shop Web" --scope orders:read="See your orders and their status. Read only."
  auth0 poppy setup --company-domain shop.example --api https://api.shop.example --app "Shop Web" --dry-run
  auth0 poppy setup --company-domain shop.example --api https://api.shop.example --app "Shop Web" --force
  auth0 poppy setup --company-domain shop.example --api https://api.shop.example --app "Shop Web" --api-url https://api.shop.example=https://api.shop.example/openapi.json --login-uri https://shop.example/login --hint checkout=changes:irreversible,personal_data:true --force`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if poppyWebUnscoped.IsSet(cmd) {
				flags.WebUnscoped = &webUnscoped
			}

			return runPoppySetup(cmd, cli, flags, opts)
		},
	}

	poppyCompanyDomain.RegisterString(cmd, &flags.CompanyDomain, "")
	poppyAPI.RegisterStringArray(cmd, &flags.APIs, nil)
	poppyAPIURL.RegisterStringArray(cmd, &flags.APIURLs, nil)
	poppyApp.RegisterString(cmd, &flags.App, "")
	poppySetupNoAPI.RegisterBool(cmd, &flags.NoAPI, false)
	poppySetupNoWeb.RegisterBool(cmd, &flags.NoWeb, false)
	poppyScope.RegisterStringArray(cmd, &flags.Scopes, nil)
	poppyHint.RegisterStringArray(cmd, &flags.Hints, nil)
	poppyCompanyName.RegisterString(cmd, &flags.Name, "")
	poppyLoginURI.RegisterString(cmd, &flags.LoginURI, "")
	poppyOut.RegisterString(cmd, &flags.Out, config.DefaultFileName)
	poppyDryRun.RegisterBool(cmd, &opts.DryRun, false)
	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")
	poppyWebUnscoped.RegisterBool(cmd, &webUnscoped, false)
	poppyOverwriteConfig.RegisterBool(cmd, &flags.ForceConfig, false)
	registerPoppyOutputFlags(cmd, cli)

	return cmd
}

func runPoppySetup(cmd *cobra.Command, cli *cli, flags setup.Flags, opts poppySetupOptions) error {
	in, err := flags.Parse()
	if err != nil {
		return poppySetupError(err)
	}

	screen := cli.newPromptScreen("Poppy setup")
	screen.ForDomain(in.CompanyDomain)

	file, err := readPoppyConfigLenient(in.Out)
	if err != nil {
		return err
	}

	if err := in.UseConfigFile(file); err != nil {
		return poppySetupError(err)
	}

	// The scopes are checked before the first question, so that nobody answers
	// them all to be told the login falls short.
	scopes := setup.ReadScopes
	if !opts.DryRun {
		scopes = append(slices.Clone(scopes), poppyWriteScopes...)
	}

	if err := cli.checkPoppyScopes(scopes); err != nil {
		return err
	}

	commandOpts := cli.poppySetupCommandOptions()

	var (
		state    *setup.State
		unhinted int
	)

	if promptAsk.canPrompt(cmd) {
		state, unhinted, err = cli.askPoppySetup(cmd, screen, &in, file)
	} else {
		state, err = cli.checkPoppySetupFlags(cmd, screen, &in, file, commandOpts)
	}

	if err != nil {
		return err
	}

	if err := askPoppyWebUnscoped(screen, promptAsk.canPrompt(cmd), state, &in); err != nil {
		return err
	}

	plan, err := setup.Build(state, in)
	if err != nil {
		return poppyBuildError(err, in, commandOpts)
	}

	textOpts := display.PoppySetupOptions{
		Inputs:   in,
		DryRun:   opts.DryRun,
		Command:  commandOpts,
		NoHeader: screen.HeaderShown(),
		Unhinted: unhinted,
	}

	if opts.DryRun {
		if !cli.renderer.PoppyHuman() {
			cli.renderer.PoppyJSON(newPoppyPlanOutput(plan))

			return nil
		}

		cli.renderer.PoppyPlan(plan, textOpts)

		// The title is printed once. After the plan, a question goes below it.
		screen.Printed()

		return nil
	}

	// With --json, only a person at a terminal sees the plan, before the question.
	if cli.renderer.PoppyHuman() || promptAsk.canPrompt(cmd) {
		cli.renderer.PoppyPlan(plan, textOpts)

		// The title is printed once. After the plan, a question goes below it.
		screen.Printed()
	}

	applied := []string{}

	if !plan.Empty() {
		switch setup.Decide(cli.force, promptAsk.canPrompt(cmd)) {
		case setup.Refuse:
			details, _ := json.Marshal(newPoppyPlanOutput(plan))

			return validationError{
				err:     errors.New("confirmation required: re-run with --force to apply the plan"),
				details: details,
				reason:  "confirmation_required",
			}
		case setup.Ask:
			question := fmt.Sprintf("Apply %s?", display.PoppyChangeCount(plan.Count()))

			proceed, err := screen.confirmAsk("Apply the plan", confirmPoppyReason(plan, in), question)
			if err != nil {
				return err
			}

			if !proceed {
				cli.renderer.PoppyDeclined("Nothing was applied.")

				return nil
			}
		}

		if applied, err = cli.applyPoppyPlan(cmd.Context(), state, in, plan); err != nil {
			return cli.writePoppyFailure(plan, cli.poppyError(err), textOpts)
		}
	}

	result := poppySetupResult{Plan: newPoppyPlanOutput(plan), Applied: appliedPoppyChanges(plan, applied)}

	if cli.renderer.PoppyJSON(result) {
		return nil
	}

	cli.renderer.PoppySetupResult(plan, textOpts)

	return nil
}

// poppyPlanOutput is the plan as the JSON output prints it. The warnings are a
// list even when there are none, and the app is said to be first-party or not.
type poppyPlanOutput struct {
	setup.Plan
	Warnings      []string `json:"warnings"`
	AppFirstParty bool     `json:"app_first_party"`
}

func newPoppyPlanOutput(plan setup.Plan) poppyPlanOutput {
	return poppyPlanOutput{Plan: plan, Warnings: poppyNonNil(plan.Warnings), AppFirstParty: plan.AppFirstParty}
}

// poppySetupResult is the JSON output of a setup that is not a dry run.
type poppySetupResult struct {
	Plan poppyPlanOutput `json:"plan"`
	// Applied are the changes that were made, in the form they have in the plan.
	Applied []setup.Change `json:"applied"`
}

// appliedPoppyChanges returns the changes of the plan that applied describes. The
// changes are made in plan order, so the applied ones are the first of the plan.
func appliedPoppyChanges(plan setup.Plan, applied []string) []setup.Change {
	changes := make([]setup.Change, 0, len(applied))

	for i := range min(len(applied), len(plan.Changes)) {
		changes = append(changes, plan.Changes[i])
	}

	return changes
}

// confirmPoppyReason says what the confirmation of a plan is about.
func confirmPoppyReason(plan setup.Plan, in setup.Inputs) string {
	reason := "This changes your Auth0 tenant and writes " + in.Out + "."

	for _, change := range plan.Changes {
		if change.Impact == setup.ImpactTenantWide {
			return reason + " The tenant settings affect every app."
		}
	}

	return reason
}

// writePoppyFailure prints the steps of a setup that stopped partway: the ones
// made, the one that failed and the ones not run, and the command that continues.
// It returns the error to end the command with: the cause, classified as a partial
// failure, with the steps and the command to continue in its details. The steps
// already say what was applied and how to continue, so a person gets only the
// cause.
func (c *cli) writePoppyFailure(plan setup.Plan, err error, opts display.PoppySetupOptions) error {
	var partial *setup.PartialError
	if !errors.As(err, &partial) {
		return err
	}

	if c.renderer.PoppyHuman() {
		c.renderer.PoppySetupFailure(plan, setup.Outcome{Done: len(partial.Applied), Failure: partial.Err}, opts)
	}

	details, _ := json.Marshal(struct {
		Applied   []string `json:"applied"`
		Failed    string   `json:"failed"`
		Remaining []string `json:"remaining"`
		Next      string   `json:"next"`
	}{
		Applied:   poppyNonNil(partial.Applied),
		Failed:    partial.Failed,
		Remaining: poppyNonNil(partial.Remaining),
		Next:      setup.FormatCommand("", setup.Command(opts.Inputs, opts.Command), 0),
	})

	return validationError{err: partial.Err, details: details, reason: "partial_failure"}
}

// poppyNonNil returns the list itself, or an empty one for nil, so that the JSON
// output has a list.
func poppyNonNil(list []string) []string {
	if list == nil {
		return []string{}
	}

	return list
}

// applyPoppyPlan makes the changes of the plan and returns what it changed.
func (c *cli) applyPoppyPlan(ctx context.Context, state *setup.State, in setup.Inputs, plan setup.Plan) ([]string, error) {
	applier := &setup.Applier{
		Tenant:  c.apiv3.TenantSettings,
		Servers: c.apiv3.ResourceServerV3,
		Clients: c.apiv3.ClientV3,
	}

	var applied []string
	err := ansi.Waiting(func() (err error) {
		applied, err = applier.Apply(ctx, state, in, plan)

		return err
	})

	return applied, err
}

// poppyReader builds the reader of the tenant state over the Management API clients.
func (c *cli) poppyReader() *setup.Reader {
	return &setup.Reader{
		TenantDomain: c.tenant,
		Tenant:       c.apiv3.TenantSettings,
		Domains:      c.api.CustomDomain,
		Servers:      c.apiv3.ResourceServerV3,
		Clients:      c.apiv3.ClientV3,
	}
}

// readPoppyState reads the tenant and the requested APIs and web app, and adds
// the existing config file to them.
func (c *cli) readPoppyState(ctx context.Context, apis []string, app string, file *config.Config) (*setup.State, error) {
	reader := c.poppyReader()

	var state *setup.State
	err := ansi.Waiting(func() (err error) {
		if state, err = reader.Read(ctx, apis, app); err != nil {
			return err
		}

		state.File = file

		return nil
	})
	if err != nil {
		return nil, c.poppyError(poppySetupError(err))
	}

	return state, nil
}

// readPoppyConfigLenient reads the config file without validating it, so a
// stale file can still be rebuilt. It returns nil when there is no file.
func readPoppyConfigLenient(path string) (*config.Config, error) {
	cfg, err := config.ReadLenient(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	return cfg, err
}

// askPoppyQuestions asks each question in its own block, in a terminal. An answer
// the question refuses ends the command.
func askPoppyQuestions(screen *questionScreen, in *setup.Inputs, questions []setup.Question) error {
	for _, question := range questions {
		if err := screen.askText(question, in); err != nil {
			return err
		}
	}

	return nil
}

// poppyPlaceholderURL stands for an answer while the questions are counted.
const poppyPlaceholderURL = "https://placeholder.invalid"

// poppyMissingQuestions returns every question that the state makes necessary,
// not only the next batch. Each is answered with a placeholder on a copy of the
// inputs, to bring the ones that follow.
func poppyMissingQuestions(state *setup.State, in setup.Inputs) []setup.Question {
	probe := in
	probe.APIURLs = maps.Clone(in.APIURLs)

	var missing []setup.Question

	for questions := setup.Questions(state, probe); len(questions) > 0; questions = setup.Questions(state, probe) {
		for _, question := range questions {
			missing = append(missing, question)

			if err := question.Answer(&probe, poppyPlaceholderURL); err != nil {
				return missing
			}
		}
	}

	return missing
}

// poppyMissingValues returns every question that the state makes necessary plus a
// question for each scope that has no description, which is what a caller that
// cannot ask needs to be told in one round.
func poppyMissingValues(state *setup.State, in setup.Inputs) []setup.Question {
	return append(poppyMissingQuestions(state, in), setup.ScopeQuestions(state, in)...)
}

// checkPoppySetupFlags reads the tenant state of a setup that cannot ask, and
// stops with every flag that is missing, in one round. It returns the state when
// nothing is missing. The API and the web app are read as the flags and the config
// file name them.
func (c *cli) checkPoppySetupFlags(cmd *cobra.Command, screen *questionScreen, in *setup.Inputs, file *config.Config, commandOpts setup.CommandOptions) (*setup.State, error) {
	if err := c.requirePoppyFlags(cmd, in, file, commandOpts); err != nil {
		return nil, err
	}

	screen.ForDomain(in.CompanyDomain)

	state, err := c.readPoppyState(cmd.Context(), in.APIs, in.App, file)
	if err != nil {
		return nil, err
	}

	// A file of another issuer is refused before the missing values are listed.
	if err := setup.CheckIssuer(state, *in); err != nil {
		return nil, poppySetupError(err)
	}

	if err := setup.Check(state, *in); err != nil {
		if !isPoppyScopeDescriptionRequired(err) {
			return nil, poppySetupError(err)
		}

		// The descriptions are asked for with the other values that are missing, so
		// that one round gets them all.
		return nil, poppyMissingFlagsError(*in, commandOpts, poppyMissingValues(state, *in))
	}

	if len(setup.Questions(state, *in)) > 0 {
		return nil, poppyMissingFlagsError(*in, commandOpts, poppyMissingQuestions(state, *in))
	}

	return state, nil
}

// requirePoppyFlags stops, naming every flag that is missing, when the company
// domain, an API or the web app was not given. With the API and the web app
// given, the state is read to find what else is missing.
func (c *cli) requirePoppyFlags(cmd *cobra.Command, in *setup.Inputs, file *config.Config, commandOpts setup.CommandOptions) error {
	questions := setup.RequiredQuestions(*in)
	if len(questions) == 0 {
		return nil
	}

	if !slices.ContainsFunc(questions, func(q setup.Question) bool { return q.Part != "" }) {
		state, err := c.readPoppyState(cmd.Context(), in.APIs, in.App, file)
		if err != nil {
			return err
		}

		questions = append(questions, poppyMissingValues(state, *in)...)
	}

	return poppyMissingFlagsError(*in, commandOpts, questions)
}

// askPoppySetup asks the questions of setup in a terminal, one part after the
// other: the company domain and name, then the API (its pick, the OpenAPI URL, the
// description and the hints of its scopes), then the web app (its pick and the
// Application Login URI). The tenant is read again when an answer changes what is
// read, so a question is asked as soon as the state can say whether it is needed.
// It returns the state with the APIs and the web app, and the number of scopes left
// without hints.
func (c *cli) askPoppySetup(cmd *cobra.Command, screen *questionScreen, in *setup.Inputs, file *config.Config) (state *setup.State, unhinted int, err error) {
	var domain, api, web []setup.Question

	for _, question := range setup.RequiredQuestions(*in) {
		switch question.Part {
		case setup.PartAPI:
			api = append(api, question)
		case setup.PartWeb:
			web = append(web, question)
		default:
			domain = append(domain, question)
		}
	}

	var (
		readAPIs []string
		readApp  string
	)

	read := func() (err error) {
		if state != nil && slices.Equal(in.APIs, readAPIs) && in.App == readApp {
			return nil
		}

		if state, err = c.readPoppyState(cmd.Context(), in.APIs, in.App, file); err != nil {
			return err
		}

		readAPIs, readApp = slices.Clone(in.APIs), in.App

		return nil
	}

	if err = c.askPoppyRequiredQuestions(cmd, screen, in, domain); err != nil {
		return nil, 0, err
	}

	if err = read(); err != nil {
		return nil, 0, err
	}

	// A file of another issuer is refused before the first question that depends on it.
	if err = setup.CheckIssuer(state, *in); err != nil {
		return nil, 0, poppySetupError(err)
	}

	if err = askPoppyQuestions(screen, in, setup.NameQuestions(state, *in)); err != nil {
		return nil, 0, err
	}

	if err = c.askPoppyRequiredQuestions(cmd, screen, in, api); err != nil {
		return nil, 0, err
	}

	if err = read(); err != nil {
		return nil, 0, err
	}

	if unhinted, err = askPoppyAPIs(screen, state, in); err != nil {
		return nil, 0, err
	}

	if err = c.askPoppyRequiredQuestions(cmd, screen, in, web); err != nil {
		return nil, 0, err
	}

	if err = read(); err != nil {
		return nil, 0, err
	}

	if err = setup.Check(state, *in); err != nil {
		return nil, 0, poppySetupError(err)
	}

	if err = askPoppyQuestions(screen, in, setup.LoginURIQuestions(state, *in)); err != nil {
		return nil, 0, err
	}

	return state, unhinted, nil
}

// askPoppyAPIs asks what the APIs of the state need and the tenant cannot say: the
// OpenAPI URL of each API, the description of each scope that has none, and the
// hints of the scopes. It returns the number of scopes left without hints. The
// state has no web app yet.
func askPoppyAPIs(screen *questionScreen, state *setup.State, in *setup.Inputs) (unhinted int, err error) {
	checkErr := setup.CheckAPIs(state, *in)
	if checkErr != nil && !isPoppyScopeDescriptionRequired(checkErr) {
		return 0, poppySetupError(checkErr)
	}

	if err = askPoppyQuestions(screen, in, setup.APIURLQuestions(state, *in)); err != nil {
		return 0, err
	}

	// The descriptions come after the OpenAPI URLs, the order of the parts.
	if checkErr != nil {
		if err = askPoppyQuestions(screen, in, setup.ScopeQuestions(state, *in)); err != nil {
			return 0, err
		}

		if err = setup.CheckAPIs(state, *in); err != nil {
			return 0, poppySetupError(err)
		}
	}

	return askPoppyHints(screen, setup.HintScopes(state, *in, true), in)
}

// askPoppyRequiredQuestions asks, in a terminal, the questions that give the company
// domain, the APIs or the web app. Each of the API and the web app is preceded by
// the question whether that part is wanted at all, and a part answered no is left
// out. The API and the web app, which must exist, are picked from the ones of the
// tenant, and the company domain starts on the one that a custom domain of the
// tenant suggests.
func (c *cli) askPoppyRequiredQuestions(cmd *cobra.Command, screen *questionScreen, in *setup.Inputs, questions []setup.Question) error {
	for _, question := range questions {
		if question.Part != "" {
			if err := askPoppyPart(screen, true, question.Part, in); err != nil {
				return err
			}

			if (question.Part == setup.PartAPI && in.NoAPI) || (question.Part == setup.PartWeb && in.NoWeb) {
				continue
			}
		}

		var err error

		switch question.Part {
		case setup.PartAPI:
			in.APIs, err = c.pickAPIsOf(cmd.Context(), screen)
		case setup.PartWeb:
			in.App, err = c.pickAppOf(cmd.Context(), screen)
		default:
			question.Default = c.guessPoppyCompanyDomain(cmd.Context())
			err = screen.askText(question, in)
		}

		if err != nil {
			return err
		}

		screen.ForDomain(in.CompanyDomain)
	}

	return nil
}

// isPoppyScopeDescriptionRequired tells whether err says scopes lack a description.
func isPoppyScopeDescriptionRequired(err error) bool {
	var input *setup.InputError

	return errors.As(err, &input) && input.Reason == setup.ReasonScopeDescriptionRequired
}

// The wording of the question about web.unscoped. The text is the question
// itself; the note follows a Yes.
const (
	poppyWebUnscopedTitle = "Web Browsing permissions"
	poppyWebUnscopedText  = "Does your website treat poppy:browser-sign-in as the account's full access on the website? Answer no to limit access to the permissions the end user approves. If yes, web.unscoped is set to true."
	poppyWebUnscopedNote  = "Make sure the description of poppy:browser-sign-in says so, for example \"Browse the website, and see and do anything your account can do there\". auth0 poppy doctor reminds you."
)

// askPoppyWebUnscoped asks, in a terminal, whether the browser join gives an agent
// full access to the account of the user on the website. The answer goes into the
// inputs as if --web-unscoped had given it. Nothing is asked when the flag was
// passed or the config file already has a value.
func askPoppyWebUnscoped(screen *questionScreen, interactive bool, state *setup.State, in *setup.Inputs) error {
	if !interactive || !state.AsksWebUnscoped(*in) {
		return nil
	}

	unscoped, err := screen.confirmAsk(poppyWebUnscopedTitle, "", poppyWebUnscopedText)
	if err != nil {
		return err
	}

	in.WebUnscoped = &unscoped

	if unscoped {
		screen.Note("%s", poppyWebUnscopedNote)
	}

	return nil
}

// The wording of the questions about the two parts of Poppy. The text is the
// question itself.
const (
	poppyAPIPartTitle  = "API access"
	poppyAPIPartReason = "Agents call your API with the scopes you approve."
	poppyAPIPartText   = "Do you want agents to use an API with Poppy?"
	poppyWebPartTitle  = "Web Browsing"
	poppyWebPartReason = "Agents browse your website on behalf of the end user, either as guests or signed in. Signed-in access is approved by end users."
	poppyWebPartText   = "Do you want agents to browse your website signed in (browser sign-in)?"
)

// askPoppyPart asks, in a terminal, whether a part of Poppy (setup.PartAPI or
// setup.PartWeb) is wanted. An answer of no turns the part off in the inputs, as
// --no-api or --no-web would. Nothing is asked when a flag or the config file has
// answered already, or when the other part is off too: that is an error, because
// there would be nothing to set up.
func askPoppyPart(screen *questionScreen, interactive bool, part string, in *setup.Inputs) error {
	if !interactive {
		return nil
	}

	switch {
	case part == setup.PartAPI && in.AsksAPI():
		wanted, err := screen.confirmDefault(poppyAPIPartTitle, poppyAPIPartReason, poppyAPIPartText, true)
		if err != nil {
			return err
		}

		in.NoAPI = !wanted
	case part == setup.PartWeb && in.AsksWeb():
		wanted, err := screen.confirmDefault(poppyWebPartTitle, poppyWebPartReason, poppyWebPartText, true)
		if err != nil {
			return err
		}

		in.NoWeb = !wanted
	}

	if in.NoAPI && in.NoWeb {
		return poppySetupError(setup.NothingToSetUp())
	}

	return nil
}

// poppySetupError turns the errors of the setup package into CLI errors. Other
// errors pass through unchanged.
func poppySetupError(err error) error {
	var input *setup.InputError
	var notReady *setup.NotReadyError
	var missing *config.MissingFieldsError

	switch {
	case errors.As(err, &input):
		return poppyInputFailure(input)
	case errors.As(err, &notReady):
		return validationError{err: err, reason: "poppy_not_ready"}
	case errors.As(err, &missing):
		return poppyMissingValuesError(missing, setup.Inputs{}, setup.CommandOptions{})
	case errors.Is(err, config.ErrIssuerMismatch):
		return validationError{err: fmt.Errorf("%w. Pass --overwrite-config to overwrite it", err), reason: "issuer_mismatch"}
	}

	return err
}

// poppyBuildError turns the error of building or deriving the config file into a
// CLI error. The values it could not fill become the flags that give them, in the
// command the user ran.
func poppyBuildError(err error, in setup.Inputs, opts setup.CommandOptions) error {
	var missing *config.MissingFieldsError
	if errors.As(err, &missing) {
		return poppyMissingValuesError(missing, in, opts)
	}

	return poppySetupError(err)
}

// askText asks a question of the setup and stores the answer in the inputs. An
// answer the question refuses ends the command.
func (s *questionScreen) askText(question setup.Question, in *setup.Inputs) error {
	answer, err := s.askLine(question.Title, question.Reason, question.Prompt, question.Default)
	if err != nil {
		return err
	}

	if err := question.Answer(in, answer); err != nil {
		return poppySetupError(err)
	}

	return nil
}

// poppyChangeNotes explain each value of the changes hint. The last, the cautious
// one, is the one a question starts on.
var poppyChangeNotes = map[string]string{
	"none":         "It only reads, nothing is changed",
	"reversible":   "It changes something that can be undone",
	"irreversible": "It cannot be undone",
}

const (
	poppyChangesQuestion      = "Does it change anything?"
	poppyPersonalDataQuestion = "Does it show or change personal data?"
)

// askPoppyHints asks for the hints of the scopes that have none, in a terminal.
// The answers go into the inputs as if --hint had given them. It starts with one
// question that can leave them all for later, and returns how many scopes were
// left without hints. After the questions of the first scope it offers to give
// the same answers to the others. Every question starts on the cautious answer,
// so that pressing Enter is always safe.
func askPoppyHints(s *questionScreen, scopes []setup.HintScope, in *setup.Inputs) (unhinted int, err error) {
	if len(scopes) == 0 {
		return 0, nil
	}

	var set bool

	intro := fmt.Sprintf("%d scopes have no hints yet. Agents show them to the user before asking for approval.", len(scopes))
	question := "Set them now?"

	if len(scopes) == 1 {
		intro = "1 scope has no hints yet. Agents show them to the user before asking for approval."
		question = "Set it now?"
	}

	err = s.ask("Hints for your scopes", intro, func() (err error) {
		set, err = s.asker.confirm(question, true)

		return err
	})
	if err != nil || !set {
		return len(scopes), err
	}

	if in.Hints == nil {
		in.Hints = make(map[string]config.Hints, len(scopes))
	}

	for i := 0; i < len(scopes); i++ {
		var (
			hint config.Hints
			same bool
		)

		scope := scopes[i]
		remaining := len(scopes) - i - 1
		title := fmt.Sprintf("Hints for %s on %s  (%d of %d)", text.Line(scope.Name), text.Line(strings.Join(scope.APIs, ", ")), i+1, len(scopes))

		err = s.ask(title, scope.Description, func() (err error) {
			if hint, err = askPoppyScopeHint(s); err != nil {
				return err
			}

			if i > 0 || remaining == 0 {
				return nil
			}

			noun := "scopes"
			if remaining == 1 {
				noun = "scope"
			}

			same, err = s.asker.confirm(fmt.Sprintf("Use the same answers for the remaining %d %s?", remaining, noun), false)

			return err
		})
		if err != nil {
			return 0, err
		}

		in.Hints[scope.Name] = hint

		if same {
			for _, other := range scopes[i+1:] {
				in.Hints[other.Name] = hint
			}

			break
		}
	}

	return 0, nil
}

// askPoppyScopeHint asks the two questions about one scope, inside the block of
// the scope.
func askPoppyScopeHint(s *questionScreen) (config.Hints, error) {
	var hint config.Hints

	width := 0
	for _, changes := range config.ValidChanges {
		width = max(width, utf8.RuneCountInString(changes))
	}

	options := make([]string, len(config.ValidChanges))
	for i, changes := range config.ValidChanges {
		options[i] = changes + strings.Repeat(" ", width-utf8.RuneCountInString(changes)+2) + poppyChangeNotes[changes]
	}

	picked, err := s.asker.choose(poppyChangesQuestion, options, len(options)-1)
	if err != nil {
		return hint, err
	}

	hint.Changes = config.ValidChanges[picked]

	if hint.PersonalData, err = s.asker.confirm(poppyPersonalDataQuestion, true); err != nil {
		return hint, err
	}

	return hint, nil
}
