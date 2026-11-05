package cli

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/poppy/discovery"
	"github.com/auth0/auth0-cli/internal/poppy/doctor"
	"github.com/auth0/auth0-cli/internal/poppy/setup"
	"github.com/auth0/auth0-cli/internal/poppy/text"
	"github.com/auth0/auth0-cli/internal/prompt"
)

// poppyExperimentalWarning closes the help text of every Poppy command.
const poppyExperimentalWarning = "**Warning:** This command is experimental and is subject to change in future versions."

// poppyAsker is how the Poppy commands ask questions. Whether a command may ask
// depends on canPrompt alone, whatever the output format. Tests replace the
// fields to run as if in a terminal.
type poppyAsker struct {
	canPrompt func(cmd *cobra.Command) bool
	// Erasable tells whether the lines of a question can be taken back off the
	// screen that the writer draws.
	erasable func(w io.Writer) bool
	// Interrupt ends the command after Ctrl-C.
	interrupt func(err error) error
	// The four kinds of prompt. A prompt that the user ends with Ctrl-C returns
	// terminal.InterruptErr.
	text       func(s *poppyScreen, label, def string) (string, error)
	confirm    func(s *poppyScreen, message string, def bool) (bool, error)
	choose     func(s *poppyScreen, message string, options []string, def int) (int, error)
	chooseMany func(s *poppyScreen, message string, options []string, defaults []int) ([]int, error)
	guest      func(s *poppyScreen) poppyGuestPrompts
}

var poppyAsk = poppyAsker{
	canPrompt:  canPrompt,
	erasable:   display.PoppyCanErase,
	interrupt:  handleInputError,
	text:       terminalPoppyText,
	confirm:    terminalPoppyConfirm,
	choose:     terminalPoppyChoose,
	chooseMany: terminalPoppyChooseMany,
	guest:      terminalPoppyGuestPrompts,
}

// errPoppyDoctorFailed is returned after the report is printed when a doctor check failed.
var errPoppyDoctorFailed = errors.New("some doctor checks failed")

// errNoPoppySession means there is no tenant login the doctor checks can use
// without asking the user for anything.
var errNoPoppySession = errors.New("no usable tenant login")

var (
	poppyCompanyDomain = Flag{
		Name:     "Company Domain",
		LongForm: "company-domain",
		Help:     "The Company's website host, such as shop.example. With several company domains, picks the one this app serves.",
	}
	poppyAPI = Flag{
		Name:     "API",
		LongForm: "api",
		Help:     "A Poppy API, by id or identifier. Repeat for several. When registering an agent, the Poppy APIs to grant (default: every Poppy API of the tenant).",
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
		Help:     "The web app's login route, used for the browser join. Asked if the app has none.",
	}
	poppyName = Flag{
		Name:     "Name",
		LongForm: "name",
		Help:     "The Company's name for the config file. When registering an agent, the agent's name.",
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
	poppyYes = Flag{
		Name:     "Yes",
		LongForm: "yes",
		Help:     "Skip confirmation.",
	}
	poppySkipDoctor = Flag{
		Name:     "Skip Doctor",
		LongForm: "skip-doctor",
		Help:     "Do not run the doctor checks after setup.",
	}
	poppyGuestAPIAccess = Flag{
		Name:     "Guest API Access",
		LongForm: "guest-api-access",
		Help:     "Also create the client grant that lets signed-out guests reach the API. Choose its scopes with --guest-scopes or --guest-all-scopes; in a terminal, setup asks.",
	}
	poppyGuestScopes = Flag{
		Name:     "Guest Scopes",
		LongForm: "guest-scopes",
		Help:     "Comma-separated scopes the guest grant carries, which every API must define. Only affects the guest grant; use --scope (singular) to describe scopes. Needs --guest-api-access. Without a terminal, pass this or --guest-all-scopes. Mutually exclusive with --guest-all-scopes.",
	}
	poppyGuestAllScopes = Flag{
		Name:     "Guest All Scopes",
		LongForm: "guest-all-scopes",
		Help:     "The guest grant carries every scope of the API. Only affects the guest grant; use --scope (singular) to describe scopes. Needs --guest-api-access. Mutually exclusive with --guest-scopes.",
	}
	poppyForceConfig = Flag{
		Name:     "Force Config",
		LongForm: "force-config",
		Help:     "Overwrite a config file that belongs to a different issuer.",
	}
	poppyGrantScopes = Flag{
		Name:     "Scopes",
		LongForm: "scopes",
		Help:     "Comma-separated list of scopes to grant. Without a terminal, pass this or --all-scopes. Mutually exclusive with --all-scopes.",
	}
	poppyAllScopes = Flag{
		Name:     "All Scopes",
		LongForm: "all-scopes",
		Help:     "Grant every offered scope. Mutually exclusive with --scopes.",
	}
	poppyRequireDPoP = Flag{
		Name:     "Require DPoP",
		LongForm: "require-dpop",
		Help:     "Require DPoP-bound tokens for the agent. Off by default.",
	}
	poppyAll = Flag{
		Name:     "All",
		LongForm: "all",
		Help:     "Include agents that have no grant to a Poppy API.",
	}
	poppyConfig = Flag{
		Name:     "Config",
		LongForm: "config",
		Help:     "Config file to check. Used only if it exists.",
	}
	poppyRemoveAllGrants = Flag{
		Name:     "Remove All Grants",
		LongForm: "remove-all-grants",
		Help:     "Also delete the app's other grants, then the agent record and the app.",
	}
)

func poppyCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "poppy",
		Short: "Set up your company for personal agents",
		Long: poppyLong("Prepares your company for personal agents, using the Personal Agent Protocol (Poppy). " +
			"Poppy settings are spread across the tenant, an API and a web app, and the discovery files on your " +
			"website must repeat Auth0's values exactly. These commands set that up, write one config file, a `poppy.json` document, for " +
			"your web and API SDKs, manage the agents you allow, and check the whole chain before an agent arrives."),
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(poppySetupCmd(cli))
	cmd.AddCommand(poppyConfigCmd(cli))
	cmd.AddCommand(poppyDoctorCmd(cli))
	cmd.AddCommand(poppyAgentsCmd(cli))

	return cmd
}

func poppyDoctorCmd(cli *cli) *cobra.Command {
	var configFile string

	cmd := &cobra.Command{
		Use:   "doctor [domain]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Check the Poppy discovery chain end to end",
		Long: poppyLong("Checks the whole discovery chain without changing anything: the files on your website, Auth0's " +
			"metadata, the tenant settings and each registered agent. Each failure comes with the spec section and a " +
			"FIX line telling you what to change. It understands sites on protocol 0.3 and 0.4, and runs the extra " +
			"checks of 0.4 (D15 to D17) only on sites that declare it. The checks that only need the company domain " +
			"(D1 to D7, and D15 to D17) work without a login, so they also work against other companies. The checks that read your tenant (D8 to " +
			"D13: tenant settings, the web app, the MFA policy, and each agent's metadata, keys and grants, and D14, " +
			"which compares the config file with the tenant) run when you are logged in to a tenant, or use the " +
			"environment credentials of `AUTH0_CLI_AUTH_MODE=env`, and show as skipped otherwise. This command " +
			"never starts a login by itself: an expired or missing session only skips those checks. They read the " +
			"tenant with `read:tenant_settings`, `read:clients`, `read:mfa_policies`, `read:client_grants`, " +
			"`read:agents`, `read:resource_servers` and `read:client_credentials`; a check whose scope is missing is " +
			"skipped with the login command that adds it. The command exits with 1 when a check fails."),
		Example: `  auth0 poppy doctor shop.example
  auth0 poppy doctor shop.example --config ./config/auth0-poppy.json
  auth0 poppy doctor shop.example --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			domain, err := poppyDoctorDomain(args, configFile)
			if err != nil {
				return err
			}

			cli.tenantExplicit = cmd.Flags().Changed("tenant")

			tenant, err := cli.optionalPoppyTenant(cmd.Context())
			if err != nil {
				return err
			}

			opts := cli.poppyDoctor
			opts.Tenant, opts.ConfigFile = tenant, configFile

			return runPoppyDoctor(cmd.Context(), cli, domain, opts)
		},
	}

	poppyConfig.RegisterString(cmd, &configFile, config.DefaultFileName)
	registerPoppyOutputFlags(cmd, cli)

	return cmd
}

// poppyDoctorTenant gives the doctor checks access to the tenant through the
// Management API clients of a login.
func (c *cli) poppyDoctorTenant() *doctor.Tenant {
	return &doctor.Tenant{State: c.poppyReader(), Agents: c.poppyAgents(), Policies: c.apiv3.GuardianPolicy}
}

// optionalPoppyTenant returns access to the tenant when a login can be used
// as it is, and nil when it cannot. It never starts a login, asks a question
// or writes the config: see setupPoppyAuthentication. An error means the
// environment credentials the caller asked for do not work.
func (c *cli) optionalPoppyTenant(ctx context.Context) (*doctor.Tenant, error) {
	switch err := c.setupPoppyAuthentication(ctx); {
	case errors.Is(err, errNoPoppySession):
		return nil, nil
	case err != nil:
		return nil, err
	}

	return c.poppyDoctorTenant(), nil
}

// setupPoppyAuthentication is setupWithAuthentication limited to what needs no
// interaction. With environment credentials selected it uses them. Otherwise it
// reads the saved login and uses it only if its token is still valid, and
// returns errNoPoppySession when it is not: it never refreshes a token,
// repairs the config, or runs a device-code login, as setupWithAuthentication
// does.
func (c *cli) setupPoppyAuthentication(ctx context.Context) error {
	if err := validateAuthMode(os.Getenv); err != nil {
		return err
	}

	if envAuthEnabled(os.Getenv) {
		return c.setupWithEnvAuthentication(ctx)
	}

	if err := c.Config.Initialize(); err != nil {
		return fmt.Errorf("%w: %w", errNoPoppySession, err)
	}

	tenant, err := c.Config.GetTenant(cmp.Or(c.tenant, c.Config.DefaultTenant))
	if err != nil {
		return fmt.Errorf("%w: %w", errNoPoppySession, err)
	}

	accessToken, err := tenant.CheckAuthenticationStatus()
	if err != nil {
		return fmt.Errorf("%w: %w", errNoPoppySession, err)
	}

	invokerMetadata := c.invokerMetadataHeaderValue()

	api, err := initializeManagementClient(tenant.Domain, accessToken, invokerMetadata)
	if err != nil {
		return authError{err: err, reason: "client_init_failed"}
	}

	apiv3, err := initializeManagementClientV3(tenant.Domain, accessToken, invokerMetadata)
	if err != nil {
		return authError{err: err, reason: "client_init_failed"}
	}

	c.tenant = tenant.Domain
	c.api = auth0.NewAPI(api)
	c.apiv3 = auth0.NewAPIV3(apiv3)

	return nil
}

// poppyDoctorDomain returns the normalized company domain to check: the
// argument, else organization.domain of the config file when it exists.
func poppyDoctorDomain(args []string, configFile string) (string, error) {
	if len(args) == 1 {
		domain, err := discovery.NormalizeDomain(args[0])
		if err != nil {
			return "", validationError{err: err}
		}

		return domain, nil
	}

	cfg, err := config.ReadLenient(configFile)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", validationError{err: fmt.Errorf("pass a domain, such as auth0 poppy doctor shop.example, or run the command where %s exists", configFile)}
	case err != nil:
		return "", err
	}

	domain, err := discovery.NormalizeDomain(cfg.Organization.Domain)
	if err != nil {
		return "", validationError{err: fmt.Errorf("organization.domain of %s: %w", configFile, err)}
	}

	return domain, nil
}

// runPoppyDoctor prints the doctor report and fails when a check failed.
func runPoppyDoctor(ctx context.Context, cli *cli, domain string, opts doctor.Options) error {
	report := doctor.Run(ctx, domain, opts)

	cli.renderer.PoppyDoctor(report)

	if report.ExitCode() != 0 {
		return errPoppyDoctorFailed
	}

	return nil
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

type poppySetupOptions struct {
	DryRun     bool
	Yes        bool
	SkipDoctor bool
}

func poppySetupCmd(cli *cli) *cobra.Command {
	var (
		flags setup.Flags
		opts  poppySetupOptions
	)

	cmd := &cobra.Command{
		Use:   "setup",
		Args:  cobra.NoArgs,
		Short: "Turn on Poppy for the tenant, an API and a web app",
		Long: poppyLong("Turns on the Poppy settings for the tenant, an existing API and an existing web app, and " +
			"writes the config file. The API and the web app must already exist, and their scopes, MFA policy and " +
			"Actions are left alone. The command shows a plan first and marks the tenant-wide changes, then applies " +
			"it once you confirm; use `--dry-run` to see the plan only, and `--yes` to skip the question, which " +
			"is required without a terminal. In a terminal it asks for any required " +
			"value you did not pass, such as the OpenAPI URL of an API or the login route of the web app. Running " +
			"it again changes nothing when the tenant already matches, or continues where a failed run stopped. It " +
			"finishes by running `auth0 poppy doctor`, including the checks that read the tenant, unless `--skip-doctor` is set; a failed check does not fail " +
			"setup, because your website may not serve the config file until you deploy it. With `--guest-api-access` the " +
			"guest grant carries only the scopes you choose, with `--guest-scopes` or `--guest-all-scopes`; in a terminal " +
			"setup asks, and it also asks whether to let signed-out guests use the API when no guest grant exists yet. " +
			"Setup adds the scope `poppy:browser-sign-in` to each API when it is missing, with a default description " +
			"unless you give one with `--scope`; every other scope offered to agents needs a description, which a terminal asks for " +
			"and otherwise comes from `--scope <name>=<description>`."),
		Example: `  auth0 poppy setup --company-domain shop.example --api https://api.shop.example --app "Shop Web" --scope orders:read="See your orders and their status. Read only."
  auth0 poppy setup --company-domain shop.example --api https://api.shop.example --app "Shop Web" --dry-run
  auth0 poppy setup --company-domain shop.example --api https://api.shop.example --app "Shop Web" --guest-api-access --guest-scopes orders:read --yes
  auth0 poppy setup --company-domain shop.example --api https://api.shop.example --app "Shop Web" --api-url https://api.shop.example=https://api.shop.example/openapi.json --login-uri https://shop.example/login --hint checkout=changes:irreversible,personal_data:true --yes`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPoppySetup(cmd, cli, flags, opts)
		},
	}

	poppyCompanyDomain.RegisterString(cmd, &flags.CompanyDomain, "")
	poppyAPI.RegisterStringArray(cmd, &flags.APIs, nil)
	poppyAPIURL.RegisterStringArray(cmd, &flags.APIURLs, nil)
	poppyApp.RegisterString(cmd, &flags.App, "")
	poppyScope.RegisterStringArray(cmd, &flags.Scopes, nil)
	poppyHint.RegisterStringArray(cmd, &flags.Hints, nil)
	poppyName.RegisterString(cmd, &flags.Name, "")
	poppyLoginURI.RegisterString(cmd, &flags.LoginURI, "")
	poppyOut.RegisterString(cmd, &flags.Out, config.DefaultFileName)
	poppyDryRun.RegisterBool(cmd, &opts.DryRun, false)
	poppyYes.RegisterBool(cmd, &opts.Yes, false)
	poppySkipDoctor.RegisterBool(cmd, &opts.SkipDoctor, false)
	poppyGuestAPIAccess.RegisterBool(cmd, &flags.GuestAPIAccess, false)
	poppyGuestScopes.RegisterStringSlice(cmd, &flags.GuestScopes, nil)
	poppyGuestAllScopes.RegisterBool(cmd, &flags.GuestAllScopes, false)
	poppyForceConfig.RegisterBool(cmd, &flags.ForceConfig, false)
	registerPoppyOutputFlags(cmd, cli)
	cmd.MarkFlagsMutuallyExclusive(poppyGuestScopes.LongForm, poppyGuestAllScopes.LongForm)

	return cmd
}

func runPoppySetup(cmd *cobra.Command, cli *cli, flags setup.Flags, opts poppySetupOptions) error {
	in, err := flags.Parse()
	if err != nil {
		return poppySetupError(err)
	}

	screen := cli.newPoppyScreen("Poppy setup")
	screen.ForDomain(in.CompanyDomain)

	if err := askPoppyQuestions(cmd, screen, &in, setup.RequiredQuestions(in)); err != nil {
		return err
	}

	screen.ForDomain(in.CompanyDomain)

	if err := cli.checkPoppyScopes(setup.ReadScopes); err != nil {
		return err
	}

	file, err := readPoppyConfigLenient(in.Out)
	if err != nil {
		return err
	}

	state, err := cli.readPoppyState(cmd.Context(), in.APIs, in.App, file)
	if err != nil {
		return err
	}

	if err := setup.Check(state, in); err != nil {
		if !poppyAsk.canPrompt(cmd) || !isPoppyScopeDescriptionRequired(err) {
			return poppySetupError(err)
		}

		if err := describePoppyScopes(screen, setup.ScopeQuestions(state, in), &in); err != nil {
			return err
		}

		if err := setup.Check(state, in); err != nil {
			return poppySetupError(err)
		}
	}

	for questions := setup.Questions(state, in); len(questions) > 0; questions = setup.Questions(state, in) {
		if err := askPoppyQuestions(cmd, screen, &in, questions); err != nil {
			return err
		}
	}

	unhinted := 0
	if poppyAsk.canPrompt(cmd) {
		if unhinted, err = askPoppyHints(screen, setup.HintScopes(state, in, true), &in); err != nil {
			return err
		}
	}

	if err := choosePoppyGuestScopes(poppyAsk.canPrompt(cmd), poppyAsk.guest(screen), state, &in); err != nil {
		return err
	}

	plan, err := setup.Build(state, in)
	if err != nil {
		return poppySetupError(err)
	}

	if !opts.DryRun {
		if err := cli.checkPoppyScopes(plan.Scopes()); err != nil {
			return err
		}
	}

	textOpts := display.PoppySetupOptions{
		Inputs:   in,
		DryRun:   opts.DryRun,
		Command:  setup.CommandOptions{Yes: opts.Yes, SkipDoctor: opts.SkipDoctor},
		NoHeader: screen.HeaderShown(),
		Unhinted: unhinted,
	}

	if opts.DryRun {
		if !cli.renderer.PoppyHuman() {
			screen.DropHeader()
			cli.renderer.PoppyJSON(plan)

			return nil
		}

		cli.renderer.PoppyPlan(plan, textOpts)

		// The title is printed once. After the plan, a question goes below it.
		screen.Printed()

		return nil
	}

	// With --json, only a person at a terminal sees the plan, before the question.
	if cli.renderer.PoppyHuman() || poppyAsk.canPrompt(cmd) {
		cli.renderer.PoppyPlan(plan, textOpts)

		// The title is printed once. After the plan, a question goes below it.
		screen.Printed()
	}

	applied := []string{}

	if !plan.Empty() {
		switch setup.Decide(opts.Yes, poppyAsk.canPrompt(cmd)) {
		case setup.Refuse:
			details, _ := json.Marshal(plan)

			return validationError{
				err:     errors.New("confirmation required: re-run with --yes to apply the plan"),
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

	result := poppySetupResult{Plan: plan, Applied: applied}

	if !opts.SkipDoctor {
		// Setup has a login, so the doctor checks can read the tenant too.
		doctorOpts := cli.poppyDoctor
		doctorOpts.Tenant, doctorOpts.ConfigFile = cli.poppyDoctorTenant(), in.Out

		var report doctor.Report
		_ = ansi.Waiting(func() error {
			report = doctor.Run(cmd.Context(), in.CompanyDomain, doctorOpts)

			return nil
		})
		result.Doctor = &report
	}

	if cli.renderer.PoppyJSON(result) {
		return nil
	}

	cli.renderer.PoppySetupResult(plan, result.Doctor, textOpts)

	return nil
}

// poppySetupResult is the JSON output of a setup that is not a dry run.
type poppySetupResult struct {
	Plan    setup.Plan     `json:"plan"`
	Applied []string       `json:"applied"`
	Doctor  *doctor.Report `json:"doctor,omitempty"`
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
// It returns the error to end the command with. The steps already say what was
// applied and how to continue, so a person gets only the cause. The JSON error
// keeps the whole account.
func (c *cli) writePoppyFailure(plan setup.Plan, err error, opts display.PoppySetupOptions) error {
	var partial *setup.PartialError
	if !errors.As(err, &partial) || !c.renderer.PoppyHuman() {
		return err
	}

	c.renderer.PoppySetupFailure(plan, setup.Outcome{Done: len(partial.Applied), Failure: partial.Err}, opts)

	return partial.Err
}

// applyPoppyPlan makes the changes of the plan and returns what it changed.
func (c *cli) applyPoppyPlan(ctx context.Context, state *setup.State, in setup.Inputs, plan setup.Plan) ([]string, error) {
	applier := &setup.Applier{
		Tenant:  c.apiv3.TenantSettings,
		Servers: c.apiv3.ResourceServerV3,
		Clients: c.apiv3.ClientV3,
		Grants:  c.apiv3.ClientGrant,
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
		Grants:       c.apiv3.ClientGrant,
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

// askPoppyQuestions asks for each missing value in a terminal, each in its own
// block that goes away once answered. Without a terminal it stops at the first,
// naming the flag that gives it.
func askPoppyQuestions(cmd *cobra.Command, screen *poppyScreen, in *setup.Inputs, questions []setup.Question) error {
	for _, question := range questions {
		if !poppyAsk.canPrompt(cmd) {
			return validationError{err: fmt.Errorf("%s is required. %s", question.Flag, question.Reason), reason: "missing_flag"}
		}

		if err := screen.askText(question, in); err != nil {
			return err
		}
	}

	return nil
}

// isPoppyScopeDescriptionRequired tells whether err says scopes lack a description.
func isPoppyScopeDescriptionRequired(err error) bool {
	var input *setup.InputError

	return errors.As(err, &input) && input.Reason == setup.ReasonScopeDescriptionRequired
}

// describePoppyScopes asks for the description of each scope that has none and
// stores it as if --scope had been passed. An answer that is empty or only blanks
// is asked again.
func describePoppyScopes(screen *poppyScreen, questions []setup.Question, in *setup.Inputs) error {
	for _, question := range questions {
		if err := screen.askText(question, in); err != nil {
			return err
		}
	}

	return nil
}

// poppyGuestPrompts are the questions about the guest grant. Tests replace them.
type poppyGuestPrompts struct {
	// Allow asks whether signed-out guests may use the API.
	allow func() (bool, error)
	// All asks whether the guest grant carries every scope. False means the user
	// wants to select the scopes.
	all func() (bool, error)
	// Pick asks which of the scopes of an API the guest grant carries, with none
	// chosen to begin with. It returns the names of the ones chosen.
	pick func(audience string, scopes []setup.GuestScope) ([]string, error)
	// Note tells the user something that is not an error.
	note func(format string, args ...any)
}

const (
	poppyGuestAllChoice    = "All scopes"
	poppyGuestSelectChoice = "Select scopes"
)

func terminalPoppyGuestPrompts(s *poppyScreen) poppyGuestPrompts {
	return poppyGuestPrompts{
		note: s.Note,
		allow: func() (allow bool, err error) {
			err = s.ask("Guest access", "Signed-out guests can use the API with only the scopes you choose.", func() (err error) {
				allow, err = s.asker.confirm(s, "Let signed-out guests use the API?", false)

				return err
			})

			return allow, err
		},
		all: func() (all bool, err error) {
			err = s.ask("Guest scopes", "The guest grant carries every scope of the API, or only the ones you select.", func() error {
				picked, err := s.asker.choose(s, "All scopes, or select scopes?", []string{poppyGuestAllChoice, poppyGuestSelectChoice}, 1)
				all = picked == 0

				return err
			})

			return all, err
		},
		pick: func(audience string, scopes []setup.GuestScope) (names []string, err error) {
			choices := make([]display.PoppyScopeChoice, len(scopes))
			for i, scope := range scopes {
				choices[i] = display.PoppyScopeChoice{Name: scope.Name, Description: scope.Description, Hints: scope.Hints}
			}

			title := "Guest scopes on " + text.Line(audience)

			err = s.ask(title, "Signed-out guests get only the scopes you select.", func() error {
				picked, err := s.asker.chooseMany(s, title, s.ScopeRows(choices), nil)
				for _, i := range picked {
					names = append(names, scopes[i].Name)
				}

				return err
			})

			return names, err
		},
	}
}

// choosePoppyGuestScopes settles which scopes the guest grant carries. A flag
// answers its question. Without one, a terminal asks and anything else fails
// before a change is made.
func choosePoppyGuestScopes(interactive bool, prompts poppyGuestPrompts, state *setup.State, in *setup.Inputs) error {
	if interactive && !in.GuestAPIAccess {
		if state.AllGuestGrants() {
			return nil
		}

		allow, err := prompts.allow()
		if err != nil || !allow {
			return err
		}

		in.GuestAPIAccess = true
	}

	chosen, err := setup.ChooseGuestScopes(state, *in)

	switch {
	case errors.Is(err, setup.ErrGuestScopesRequired) && interactive:
		return askPoppyGuestScopes(prompts, state, in)
	case errors.Is(err, setup.ErrGuestScopesRequired):
		return poppyGuestScopesRequiredError(state, *in)
	case err != nil:
		return poppySetupError(err)
	}

	in.GuestScopes = chosen

	return nil
}

// askPoppyGuestScopes asks whether the guest grant carries every scope or a
// selection, and for the selection on each API that has no guest grant yet. An
// API with no scope chosen gets no guest grant.
func askPoppyGuestScopes(prompts poppyGuestPrompts, state *setup.State, in *setup.Inputs) error {
	all, err := prompts.all()
	if err != nil {
		return err
	}

	if all {
		in.GuestAllScopes = true

		return nil
	}

	offers, err := setup.GuestOffers(state, *in)
	if err != nil {
		return poppySetupError(err)
	}

	in.GuestScopes = make(map[string][]string, len(offers))

	var skipped []string

	for _, offer := range offers {
		picked, err := prompts.pick(offer.Audience, offer.Scopes)
		if err != nil {
			return err
		}

		if len(picked) == 0 {
			skipped = append(skipped, offer.Audience)

			continue
		}

		in.GuestScopes[offer.Audience] = picked
	}

	// One note is enough: when nothing was chosen it covers every API.
	if len(in.GuestScopes) == 0 {
		prompts.note("No scope was chosen, so the guest grant is left as it is.")

		return nil
	}

	for _, audience := range skipped {
		prompts.note("No scope chosen for %s, so it gets no guest grant.", audience)
	}

	return nil
}

// poppyGuestScopesRequiredError lists the scopes on offer, with their hints, for
// a caller that cannot be asked.
func poppyGuestScopesRequiredError(state *setup.State, in setup.Inputs) error {
	offers, err := setup.GuestOffers(state, in)
	if err != nil {
		return poppySetupError(err)
	}

	type offered struct {
		Name        string        `json:"name"`
		Description string        `json:"description"`
		Hints       *config.Hints `json:"hints,omitempty"`
	}
	type offer struct {
		Audience string    `json:"audience"`
		Scopes   []offered `json:"scopes"`
	}

	var message strings.Builder
	message.WriteString("pass --guest-scopes or --guest-all-scopes to choose the scopes of the guest grant. Offered scopes:")

	offerList := make([]offer, len(offers))
	for i, o := range offers {
		offerList[i] = offer{Audience: o.Audience, Scopes: make([]offered, len(o.Scopes))}

		names := make([]string, len(o.Scopes))
		for j, scope := range o.Scopes {
			names[j] = scope.Name
			offerList[i].Scopes[j] = offered{Name: scope.Name, Description: scope.Description, Hints: scope.Hints}
		}

		fmt.Fprintf(&message, "\n  %s: %s", o.Audience, strings.Join(names, ", "))
	}

	message.WriteString("\n  FIX  Pass --guest-scopes <scope>[,<scope>] to name them, or --guest-all-scopes to carry every scope.")

	details, _ := json.Marshal(map[string]any{"apis": offerList})

	return validationError{err: errors.New(message.String()), details: details, reason: "scopes_required"}
}

// poppySetupError turns the errors of the setup package into CLI errors. Other
// errors pass through unchanged.
func poppySetupError(err error) error {
	var input *setup.InputError
	var notReady *setup.NotReadyError
	var missing *config.MissingFieldsError

	switch {
	case errors.As(err, &input):
		return validationError{err: err, reason: input.Reason}
	case errors.As(err, &notReady):
		return validationError{err: err, reason: "poppy_not_ready"}
	case errors.As(err, &missing):
		return validationError{
			err:    fmt.Errorf("%w. Pass --api-url as <identifier>=<url>, or run auth0 poppy setup", err),
			reason: "missing_config_values",
		}
	case errors.Is(err, config.ErrIssuerMismatch):
		return validationError{err: fmt.Errorf("%w. Pass --force-config to overwrite it", err), reason: "issuer_mismatch"}
	}

	return err
}

func poppyConfigCmd(cli *cli) *cobra.Command {
	var flags setup.Flags

	cmd := &cobra.Command{
		Use:   "config",
		Args:  cobra.NoArgs,
		Short: "Rebuild the Poppy config file from the tenant",
		Long: poppyLong("Rebuilds the config file from the tenant without changing the tenant, so CI and deploys can " +
			"refresh it safely. It reads the tenant, adds the values Auth0 does not store, and writes the file. " +
			"If the tenant is not set up for Poppy, it exits with an error naming the missing setting. The config " +
			"file is for one website, so with several company domains pass `--company-domain` to pick the one this " +
			"app serves. The file is a protocol 0.4 `poppy.json` document and nothing else, so your web SDK can " +
			"serve it as `/.well-known/poppy.json`. A file written for 0.3 is upgraded, and members of the file that " +
			"the CLI does not use are kept. The file holds no API identifier and no web app, so without `--api` the " +
			"APIs are the Poppy APIs of the tenant that the `apis` entries of the existing file are at, and without " +
			"`--app` the web app is the only app of the tenant that has the browser join on. When that does not " +
			"settle on one, the command stops and asks for `--api` or `--app`."),
		Example: `  auth0 poppy config
  auth0 poppy config --company-domain shop.example --out ./config/auth0-poppy.json
  auth0 poppy config --company-domain shop.example --api https://api.shop.example --app "Shop Web" --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPoppyConfig(cmd, cli, flags)
		},
	}

	poppyCompanyDomain.RegisterString(cmd, &flags.CompanyDomain, "")
	poppyAPI.RegisterStringArray(cmd, &flags.APIs, nil)
	poppyAPIURL.RegisterStringArray(cmd, &flags.APIURLs, nil)
	poppyApp.RegisterString(cmd, &flags.App, "")
	poppyHint.RegisterStringArray(cmd, &flags.Hints, nil)
	poppyName.RegisterString(cmd, &flags.Name, "")
	poppyOut.RegisterString(cmd, &flags.Out, config.DefaultFileName)
	poppyForceConfig.RegisterBool(cmd, &flags.ForceConfig, false)
	registerPoppyOutputFlags(cmd, cli)

	return cmd
}

func runPoppyConfig(cmd *cobra.Command, cli *cli, flags setup.Flags) error {
	in, err := flags.Parse()
	if err != nil {
		return poppySetupError(err)
	}

	screen := cli.newPoppyScreen("Poppy config")
	screen.ForDomain(in.CompanyDomain)

	if err := cli.checkPoppyScopes(setup.ReadScopes); err != nil {
		return err
	}

	file, err := readPoppyConfigLenient(in.Out)
	if err != nil {
		return err
	}

	apis, app, err := cli.resolvePoppyAPIsAndApp(cmd.Context(), in, file)
	if err != nil {
		return err
	}

	state, err := cli.readPoppyState(cmd.Context(), apis, app, file)
	if err != nil {
		return err
	}

	domain, choices := state.CompanyDomain(in)
	if len(choices) > 1 {
		if domain, err = askPoppyCompanyDomain(cmd, screen, choices); err != nil {
			return err
		}
	}
	in.CompanyDomain = domain
	screen.ForDomain(domain)

	if err := setup.Ready(state, in); err != nil {
		return poppySetupError(err)
	}

	unhinted := 0
	if poppyAsk.canPrompt(cmd) {
		if unhinted, err = askPoppyHints(screen, setup.HintScopes(state, in, false), &in); err != nil {
			return err
		}
	}

	cfg, err := setup.Derive(state, in)
	if err != nil {
		return poppySetupError(err)
	}

	before, readErr := os.ReadFile(in.Out)

	if err := config.Write(in.Out, cfg); err != nil {
		return err
	}

	if !cli.renderer.PoppyHuman() {
		screen.DropHeader()
		cli.renderer.PoppyJSON(cfg)

		return nil
	}

	after, err := os.ReadFile(in.Out)
	unchanged := readErr == nil && err == nil && bytes.Equal(before, after)

	cli.renderer.PoppyConfigResult(in, unchanged, screen.HeaderShown(), unhinted)

	return nil
}

// askPoppyCompanyDomain picks one of the company domains the tenant lists.
func askPoppyCompanyDomain(cmd *cobra.Command, screen *poppyScreen, choices []string) (domain string, err error) {
	if !poppyAsk.canPrompt(cmd) {
		return "", validationError{
			err:    errors.New("--company-domain is required: the tenant lists several company domains (" + strings.Join(choices, ", ") + ")"),
			reason: "missing_flag",
		}
	}

	err = screen.ask("Company domain", "The tenant lists several company domains, and the config file is for the one website your app serves.", func() error {
		picked, err := screen.asker.choose(screen, "Company domain this app serves", choices, 0)
		if err == nil {
			domain = choices[picked]
		}

		return err
	})

	return domain, err
}

// resolvePoppyAPIsAndApp returns the APIs and the web app to put in the config
// file: the ones given by flags, else the ones the tenant and the existing file
// settle on. The file names neither, so the APIs are the Poppy APIs of the
// tenant that its apis entries are at, and the app is the only app that has the
// browser join on. It stops with a FIX line when that is not one.
func (c *cli) resolvePoppyAPIsAndApp(ctx context.Context, in setup.Inputs, file *config.Config) (apis []string, app string, err error) {
	apis, app = in.APIs, in.App
	if len(apis) > 0 && app != "" {
		return apis, app, nil
	}

	reader := c.poppyReader()

	err = ansi.Waiting(func() error {
		if len(apis) == 0 {
			poppy, err := reader.ListPoppyAPIs(ctx)
			if err != nil {
				return err
			}

			if apis, err = setup.ResolveAPIs(file, poppy); err != nil {
				return err
			}
		}

		if app == "" {
			joined, err := reader.BrowserJoinApps(ctx)
			if err != nil {
				return err
			}

			if app, err = setup.ResolveApp(joined); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, "", c.poppyError(poppySetupError(err))
	}

	return apis, app, nil
}

// poppyScreen is what a Poppy command shows while it asks questions in a
// terminal: the text and the mechanics of the screen are in display, and this
// holds the prompts that are asked on it.
type poppyScreen struct {
	*display.PoppyScreen
	// Asker holds the prompts. It is the one that was set when the screen started.
	asker poppyAsker
	// Erase tells whether the blocks can be taken back. Without it they stay.
	erase bool
}

// newPoppyScreen starts the screen of a command. The title of the run needs the
// company domain, see ForDomain.
func (c *cli) newPoppyScreen(title string) *poppyScreen {
	w := c.renderer.ResultWriter
	erase := poppyAsk.erasable(w)

	return &poppyScreen{
		PoppyScreen: c.renderer.NewPoppyScreen(title, c.tenant, display.PoppyLayoutOf(w), erase),
		asker:       poppyAsk,
		erase:       erase,
	}
}

// ask draws a block with a title and a reason, runs the questions in it, and takes
// the block back. Ctrl-C takes it back too, tells that nothing was changed, and
// ends the command the way the other prompts do.
func (s *poppyScreen) ask(title, reason string, questions func() error) error {
	s.Begin(title, reason)

	err := questions()
	eraseErr := s.End()

	if errors.Is(err, terminal.InterruptErr) {
		s.DropHeader()
		s.Cancelled()

		return s.asker.interrupt(err)
	}

	return errors.Join(err, eraseErr)
}

// askLine asks a question with a title, a reason and a place to type the answer,
// and asks again for an answer that is empty.
func (s *poppyScreen) askLine(title, reason, label, def string) (answer string, err error) {
	err = s.ask(title, reason, func() error {
		for {
			var err error
			if answer, err = s.asker.text(s, label, def); err != nil {
				return err
			}

			if strings.TrimSpace(answer) != "" {
				return nil
			}

			s.Invalid("A value is required.")
		}
	})

	return answer, err
}

// askText asks a question of the setup and stores the answer in the inputs. An
// answer the question refuses ends the command.
func (s *poppyScreen) askText(question setup.Question, in *setup.Inputs) error {
	answer, err := s.askLine(question.Title, question.Reason, question.Prompt, question.Default)
	if err != nil {
		return err
	}

	if err := question.Answer(in, answer); err != nil {
		return poppySetupError(err)
	}

	return nil
}

// confirmAsk asks a question answered with yes or no, which is no unless the user
// says yes.
func (s *poppyScreen) confirmAsk(title, reason, message string) (yes bool, err error) {
	err = s.ask(title, reason, func() (err error) {
		yes, err = s.asker.confirm(s, message, false)

		return err
	})

	return yes, err
}

// The templates of the prompts when the screen can take them back. They draw the
// question the same way as the lines around them, indented under the title, and
// draw nothing once the question is answered, so there is no echo of the answer
// to take back.
const (
	poppyInputTemplate = `
{{- if not .ShowAnswer}}  {{.Message}}  {{if .Default}}{{color "black+h"}}({{.Default}}){{color "reset"}} {{end}}{{end}}`

	poppySelectTemplate = `
{{- define "option"}}
    {{- if eq .SelectedIndex .CurrentIndex }}{{color .Config.Icons.SelectFocus.Format }}  {{ .Config.Icons.SelectFocus.Text }} {{else}}{{color "default"}}    {{end}}
    {{- .CurrentOpt.Value}}
    {{- color "reset"}}
{{end}}
{{- if not .ShowAnswer}}  {{.Message}}{{"\n"}}
  {{- range $ix, $option := .PageEntries}}
    {{- template "option" $.IterateOption $ix $option}}
  {{- end}}
{{- end}}`

	poppyMultiSelectTemplate = `
{{- define "option"}}
    {{- "  "}}
    {{- if eq .SelectedIndex .CurrentIndex }}{{color .Config.Icons.SelectFocus.Format }}{{ .Config.Icons.SelectFocus.Text }}{{color "reset"}}{{else}} {{end}}
    {{- " "}}
    {{- if index .Checked .CurrentOpt.Index }}{{color .Config.Icons.MarkedOption.Format }}{{ .Config.Icons.MarkedOption.Text }}{{else}}{{color .Config.Icons.UnmarkedOption.Format }}{{ .Config.Icons.UnmarkedOption.Text }}{{end}}
    {{- color "reset"}}
    {{- " "}}{{- .CurrentOpt.Value}}
{{end}}
{{- if not .ShowAnswer}}
  {{- range $ix, $option := .PageEntries}}
    {{- template "option" $.IterateOption $ix $option}}
  {{- end}}
  {{- "  "}}{{color "black+h"}}` + poppyMultiSelectKeys + `{{color "reset"}}{{"\n"}}
{{- end}}`

	// The keys of a list of choices that can be several.
	poppyMultiSelectKeys = "space to toggle, right for all, left for none, enter to confirm"
)

// quiet runs a prompt with the templates above when the screen can take its lines
// back. Otherwise the prompt keeps its own, which show the answer.
func (s *poppyScreen) quiet(run func() error) error {
	if !s.erase {
		return run()
	}

	input, choice, multi := survey.InputQuestionTemplate, survey.SelectQuestionTemplate, survey.MultiSelectQuestionTemplate
	defer func() {
		survey.InputQuestionTemplate, survey.SelectQuestionTemplate, survey.MultiSelectQuestionTemplate = input, choice, multi
	}()

	survey.InputQuestionTemplate = poppyInputTemplate
	survey.SelectQuestionTemplate = poppySelectTemplate
	survey.MultiSelectQuestionTemplate = poppyMultiSelectTemplate

	return run()
}

// askPrompt runs one prompt. Ctrl-C comes back as terminal.InterruptErr, and any
// other failure as an unexpected error.
func (s *poppyScreen) askPrompt(question survey.Prompt, answer any) error {
	err := s.quiet(func() error { return prompt.AskOne(&survey.Question{Prompt: question}, answer) })

	switch {
	case err == nil, errors.Is(err, terminal.InterruptErr):
		return err
	default:
		return unexpectedError(err)
	}
}

// leftOnScreen counts the rows of a list that Ctrl-C leaves on the screen. A list
// does not take its rows back then, and the cursor is below them.
func (s *poppyScreen) leftOnScreen(rows ...string) {
	for _, row := range rows {
		s.AddRows(s.Rows(row))
	}
}

// terminalPoppyText asks for a line of text, with a default that Enter accepts.
func terminalPoppyText(s *poppyScreen, label, def string) (string, error) {
	var answer string
	if err := s.askPrompt(&survey.Input{Message: label, Default: def}, &answer); err != nil {
		return "", err
	}

	// The prompt takes back the row it ends on. A typed answer that wrapped onto
	// more rows than the question took leaves the others behind.
	typed := answer
	if typed == def {
		typed = ""
	}

	question := "  " + label + "  "
	if def != "" {
		question += "(" + def + ") "
	}

	s.AddRows(s.Rows(question+typed) - s.Rows(question))

	return answer, nil
}

// terminalPoppyConfirm asks a question answered with yes or no. Enter takes the
// default.
func terminalPoppyConfirm(s *poppyScreen, message string, def bool) (bool, error) {
	options := ansi.Faint("(y/N)")
	if def {
		options = ansi.Faint("(Y/n)")
	}

	for {
		answer, err := terminalPoppyText(s, message+"  "+options, "")
		if err != nil {
			return false, err
		}

		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "":
			return def, nil
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		}

		s.Invalid("Answer y or n.")
	}
}

// terminalPoppyChoose asks to pick one of the options, which start on the one at
// index def. It returns the index picked.
func terminalPoppyChoose(s *poppyScreen, message string, options []string, def int) (int, error) {
	var picked int

	err := s.askPrompt(&survey.Select{
		Message:  message,
		Options:  options,
		Default:  def,
		PageSize: len(options),
		// Typing must not filter a short list that has no room to show it.
		Filter: func(string, string, int) bool { return true },
	}, &picked)

	if errors.Is(err, terminal.InterruptErr) {
		rows := []string{"  " + message}
		for _, option := range options {
			rows = append(rows, "    "+option)
		}

		s.leftOnScreen(rows...)
	}

	return picked, err
}

// terminalPoppyChooseMany asks to pick any of the options, which start with the
// ones at the indexes in defaults chosen. It returns the indexes picked.
func terminalPoppyChooseMany(s *poppyScreen, message string, options []string, defaults []int) ([]int, error) {
	var picked []int

	page := min(len(options), poppyMaxListRows)

	err := s.askPrompt(&survey.MultiSelect{
		Message:  message,
		Options:  options,
		Default:  defaults,
		PageSize: page,
		Filter:   func(string, string, int) bool { return true },
	}, &picked)

	if errors.Is(err, terminal.InterruptErr) {
		var rows []string
		for _, option := range options[:page] {
			rows = append(rows, strings.Repeat(" ", display.PoppyListMarks)+option)
		}

		s.leftOnScreen(append(rows, "  "+poppyMultiSelectKeys)...)
	}

	return picked, err
}

// poppyMaxListRows is the most options a list shows at once before it scrolls.
const poppyMaxListRows = 12

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
func askPoppyHints(s *poppyScreen, scopes []setup.HintScope, in *setup.Inputs) (unhinted int, err error) {
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
		set, err = s.asker.confirm(s, question, true)

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

			same, err = s.asker.confirm(s, fmt.Sprintf("Use the same answers for the remaining %d %s?", remaining, noun), false)

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
// the scope. The first answer stays in view while the second is asked.
func askPoppyScopeHint(s *poppyScreen) (config.Hints, error) {
	var hint config.Hints

	width := 0
	for _, changes := range config.ValidChanges {
		width = max(width, utf8.RuneCountInString(changes))
	}

	options := make([]string, len(config.ValidChanges))
	for i, changes := range config.ValidChanges {
		options[i] = changes + strings.Repeat(" ", width-utf8.RuneCountInString(changes)+2) + poppyChangeNotes[changes]
	}

	picked, err := s.asker.choose(s, poppyChangesQuestion, options, len(options)-1)
	if err != nil {
		return hint, err
	}

	hint.Changes = config.ValidChanges[picked]
	s.Answered(poppyChangesQuestion, hint.Changes)

	if hint.PersonalData, err = s.asker.confirm(s, poppyPersonalDataQuestion, true); err != nil {
		return hint, err
	}

	answer := "No"
	if hint.PersonalData {
		answer = "Yes"
	}
	s.Answered(poppyPersonalDataQuestion, answer)

	return hint, nil
}
