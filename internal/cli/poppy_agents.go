package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/poppy/agents"
	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/poppy/setup"
	"github.com/auth0/auth0-cli/internal/poppy/text"
)

// poppyAgentArgument is an argument of the agents commands that a terminal asks
// for when it is not given.
type poppyAgentArgument struct {
	// Title and reason are drawn above the question, and label is where the answer is
	// typed.
	title, reason, label string
}

var (
	poppyAgentURL = poppyAgentArgument{
		title:  "Agent URL",
		reason: "URL of the agent's metadata document.",
		label:  "URL",
	}
	poppyAgentRef = poppyAgentArgument{
		title:  "Agent URL or ID",
		reason: "URL or agent ID of the agent.",
		label:  "URL or ID",
	}
)

// Management API scopes each agents command needs, checked before it changes anything.
var (
	poppyListScopes     = []string{"read:agents", "read:clients", "read:client_grants", "read:resource_servers"}
	poppyShowScopes     = append(slices.Clone(poppyListScopes), "read:client_credentials")
	poppyRegisterScopes = []string{
		"read:clients", "create:clients", "update:clients", "read:agents", "create:agents",
		"read:client_grants", "create:client_grants", "read:resource_servers",
	}
	poppyRemoveScopes = []string{
		"read:agents", "read:clients", "read:client_grants", "read:resource_servers",
		"delete:client_grants", "delete:agents", "delete:clients",
	}
)

func poppyAgentsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agents",
		Short: "Manage the agents your company allows",
		Long: poppyLong("Manages the personal agents your company allows. In the beta, Auth0 accepts only agents " +
			"your company registers, so the registered list is the allow list. A Poppy agent is an agent app " +
			"with a grant to a Poppy API."),
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(poppyAgentsRegisterCmd(cli))
	cmd.AddCommand(poppyAgentsListCmd(cli))
	cmd.AddCommand(poppyAgentsShowCmd(cli))
	cmd.AddCommand(poppyAgentsRemoveCmd(cli))

	return cmd
}

type poppyRegisterInputs struct {
	APIs        []string
	Name        string
	Out         string
	Yes         bool
	Scopes      []string
	AllScopes   bool
	RequireDPoP bool
}

func poppyAgentsRegisterCmd(cli *cli) *cobra.Command {
	var inputs poppyRegisterInputs

	cmd := &cobra.Command{
		Use:   "register [agent-url]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Register an agent so it can sign in to your company",
		Long: poppyLong("Registers an agent from the URL of its metadata document. The command previews the document, " +
			"creates or updates the agent app, creates the agent record and grants the agent your Poppy APIs. " +
			"It changes the tenant only: the config file is not written, and is read only to show the hints of the " +
			"scopes on offer. By default it grants every Poppy API of the tenant. In a terminal it asks " +
			"which scopes to grant; otherwise pass `--scopes` or `--all-scopes`. DPoP is required only with " +
			"`--require-dpop`. Running it again refreshes the agent app from the current document and changes " +
			"nothing else, which is also how you pick up keys the agent has rotated."),
		Example: `  auth0 poppy agents register https://muse.example/agent.json
  auth0 poppy agents register https://muse.example/agent.json --api https://api.shop.example --scopes orders:read --yes
  auth0 poppy agents register https://muse.example/agent.json --all-scopes --require-dpop`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPoppyAgentsRegister(cmd, cli, inputs, args)
		},
	}

	poppyAPI.RegisterStringArray(cmd, &inputs.APIs, nil)
	poppyName.RegisterString(cmd, &inputs.Name, "")
	poppyOut.RegisterString(cmd, &inputs.Out, config.DefaultFileName)
	poppyYes.RegisterBool(cmd, &inputs.Yes, false)
	poppyGrantScopes.RegisterStringSlice(cmd, &inputs.Scopes, nil)
	poppyAllScopes.RegisterBool(cmd, &inputs.AllScopes, false)
	poppyRequireDPoP.RegisterBool(cmd, &inputs.RequireDPoP, false)
	registerPoppyOutputFlags(cmd, cli)
	cmd.MarkFlagsMutuallyExclusive(poppyGrantScopes.LongForm, poppyAllScopes.LongForm)

	return cmd
}

func poppyAgentsListCmd(cli *cli) *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "list",
		Args:  cobra.NoArgs,
		Short: "List the registered agents",
		Long: poppyLong("Lists the Poppy agents registered with your company, with their metadata status and the APIs " +
			"each one can use. By default it shows only agents that have a grant to a Poppy API; use `--all` to " +
			"include the others."),
		Example: `  auth0 poppy agents list
  auth0 poppy agents list --all
  auth0 poppy agents list --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPoppyAgentsList(cmd, cli, all)
		},
	}

	poppyAll.RegisterBool(cmd, &all, false)
	registerPoppyOutputFlags(cmd, cli)

	return cmd
}

func poppyAgentsShowCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show [agent-url|agent-id]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show the details of an agent",
		Long: poppyLong("Shows one agent's details, given its URL or agent ID. It also checks the agent's metadata " +
			"document live and tells you whether the keys Auth0 stored are still current, so you know when the " +
			"agent needs to be registered again."),
		Example: `  auth0 poppy agents show https://muse.example/agent.json
  auth0 poppy agents show <agent-id>
  auth0 poppy agents show https://muse.example/agent.json --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPoppyAgentsShow(cmd, cli, args)
		},
	}

	registerPoppyOutputFlags(cmd, cli)

	return cmd
}

type poppyRemoveInputs struct {
	DryRun          bool
	RemoveAllGrants bool
}

func poppyAgentsRemoveCmd(cli *cli) *cobra.Command {
	var inputs poppyRemoveInputs

	cmd := &cobra.Command{
		Use:     "remove [agent-url|agent-id]",
		Aliases: []string{"unregister", "rm"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "Remove an agent's Poppy access",
		Long: poppyLong("Removes the agent's Poppy access: its grants to your Poppy APIs. It changes the tenant only, " +
			"and does not touch the config file. When the agent has no other grants it also removes the agent record and the app, so registering " +
			"the agent again creates a new app and its users must sign in again. When the app has other grants, " +
			"only the Poppy access is removed and the other grants are listed, unless you pass `--remove-all-grants`; " +
			"in a terminal you are asked which to do. Deleting the app revokes its refresh tokens; when the app is kept, " +
			"its refresh tokens are not revoked. Access tokens already issued stay valid until they expire. " +
			"Use `--dry-run` to print what would be removed, and `--force` to skip the confirmation."),
		Example: `  auth0 poppy agents remove https://muse.example/agent.json
  auth0 poppy agents remove https://muse.example/agent.json --dry-run
  auth0 poppy agents remove <agent-id> --remove-all-grants --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPoppyAgentsRemove(cmd, cli, inputs, args)
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")
	poppyDryRun.RegisterBool(cmd, &inputs.DryRun, false)
	poppyRemoveAllGrants.RegisterBool(cmd, &inputs.RemoveAllGrants, false)
	registerPoppyOutputFlags(cmd, cli)

	return cmd
}

// poppyAgents builds the agents service over the Management API clients.
func (c *cli) poppyAgents() *agents.Service {
	return &agents.Service{
		Clients:         c.apiv3.ClientV3,
		Agents:          c.apiv3.Agent,
		Grants:          c.apiv3.ClientGrant,
		ResourceServers: c.apiv3.ResourceServerV3,
		Credentials:     c.apiv3.ClientCredential,
	}
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

// poppyError maps a 403 from the Management API to the login command for a
// missing scope, or to the tenant lacking a Poppy feature.
func (c *cli) poppyError(err error) error {
	var partial *agents.PartialError
	if errors.As(err, &partial) {
		partial.Err = c.poppyError(partial.Err)

		return err
	}

	var setupPartial *setup.PartialError
	if errors.As(err, &setupPartial) {
		setupPartial.Err = c.poppyError(setupPartial.Err)

		return err
	}

	var apiErr *core.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusForbidden {
		return err
	}

	var body []byte
	if inner := apiErr.Unwrap(); inner != nil {
		body = []byte(inner.Error())
	}

	if scopeErr := isInsufficientScopeError(apiErr.StatusCode, body); scopeErr != nil {
		return scopeErr
	}

	return poppyUnavailableError{err: err, debug: c.debug}
}

// poppyRef returns the argument given, or asks for it in a terminal.
func poppyRef(cmd *cobra.Command, screen *poppyScreen, argument poppyAgentArgument, args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}

	if !poppyAsk.canPrompt(cmd) {
		return "", usageError{err: fmt.Errorf("missing a required argument: %s", argument.title), reason: "missing_required_args"}
	}

	return screen.askLine(argument.title, argument.reason, argument.label, "")
}

// readPoppyConfig reads the config file, or returns nil when there is none. A
// file that exists but is not valid stops the command before it changes anything.
func readPoppyConfig(path string) (*config.Config, error) {
	cfg, err := config.Read(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	return cfg, err
}

func poppyScopeHints(cfg *config.Config, scope string) *config.Hints {
	if cfg == nil {
		return nil
	}

	return cfg.Scopes[scope].Hints
}

func runPoppyAgentsRegister(cmd *cobra.Command, cli *cli, inputs poppyRegisterInputs, args []string) error {
	screen := cli.newPoppyScreen("")

	agentURL, err := poppyRef(cmd, screen, poppyAgentURL, args)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(agentURL, "https://") {
		return validationError{err: fmt.Errorf("the agent URL must start with https://: %s", agentURL), reason: "invalid_agent_url"}
	}

	if err := cli.checkPoppyScopes(poppyRegisterScopes); err != nil {
		return err
	}

	cfg, err := readPoppyConfig(inputs.Out)
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	svc := cli.poppyAgents()

	preview, err := svc.Preview(ctx, agentURL)
	if err != nil {
		var invalid *agents.InvalidDocumentError
		if errors.As(err, &invalid) {
			details, _ := json.Marshal(map[string]any{"violations": invalid.Violations})

			return validationError{err: err, details: details, reason: "agent_invalid"}
		}

		return cli.poppyError(err)
	}

	apis, err := svc.PoppyAPIs(ctx, inputs.APIs)
	if err != nil {
		return cli.poppyError(err)
	}

	grants, err := agents.ChooseScopes(apis, inputs.Scopes, inputs.AllScopes)

	// With --json, only a person at a terminal who is asked something sees the
	// metadata of the agent.
	asking := poppyAsk.canPrompt(cmd) && (!inputs.Yes || errors.Is(err, agents.ErrScopesRequired))
	if cli.renderer.PoppyHuman() || asking {
		cli.renderer.PoppyAgentPreview(preview)
	}

	switch {
	case errors.Is(err, agents.ErrScopesRequired) && poppyAsk.canPrompt(cmd):
		grants, err = promptPoppyGrants(screen, apis, cfg)
	case errors.Is(err, agents.ErrScopesRequired):
		return scopesRequiredError(apis, cfg)
	case err != nil:
		return validationError{err: err, reason: "unknown_scope"}
	}
	if err != nil {
		return err
	}

	if poppyAsk.canPrompt(cmd) && !inputs.Yes {
		cli.renderer.PoppyRegisterPlan(preview, inputs.Name, inputs.RequireDPoP, grants)

		proceed, err := screen.confirmAsk("Register the agent", "This changes your Auth0 tenant: it creates or updates the agent app and grants it your APIs.", "Register this agent?")
		if err != nil {
			return err
		}

		if !proceed {
			cli.renderer.PoppyDeclined("Nothing was registered.")

			return nil
		}
	}

	reg, err := svc.Register(ctx, agents.RegisterRequest{
		Preview:     preview,
		Name:        inputs.Name,
		RequireDPoP: inputs.RequireDPoP,
		Grants:      grants,
	})
	if err != nil {
		return cli.poppyError(err)
	}

	if cli.renderer.PoppyJSON(reg) {
		return nil
	}

	cli.renderer.PoppyRegistration(reg)

	return nil
}

// scopesRequiredError lists the scopes on offer, with their hints, for a caller
// that cannot be asked.
func scopesRequiredError(apis []agents.API, cfg *config.Config) error {
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
	message.WriteString("pass --scopes or --all-scopes to choose the scopes to grant. Offered scopes:")

	offers := make([]offer, len(apis))
	for i, api := range apis {
		offers[i] = offer{Audience: api.Identifier, Scopes: make([]offered, len(api.Scopes))}

		names := make([]string, len(api.Scopes))
		for j, scope := range api.Scopes {
			names[j] = scope.Name
			offers[i].Scopes[j] = offered{Name: scope.Name, Description: scope.Description, Hints: poppyScopeHints(cfg, scope.Name)}
		}

		fmt.Fprintf(&message, "\n  %s: %s", api.Identifier, strings.Join(names, ", "))
	}

	details, _ := json.Marshal(map[string]any{"apis": offers})

	return validationError{err: errors.New(message.String()), details: details, reason: "scopes_required"}
}

// promptPoppyGrants asks which scopes to grant on each API. Scopes that only
// read are selected to begin with.
func promptPoppyGrants(screen *poppyScreen, apis []agents.API, cfg *config.Config) ([]agents.Grant, error) {
	grants := make([]agents.Grant, len(apis))

	for i, api := range apis {
		choices := make([]display.PoppyScopeChoice, len(api.Scopes))

		var defaults []int
		for j, scope := range api.Scopes {
			hints := poppyScopeHints(cfg, scope.Name)
			choices[j] = display.PoppyScopeChoice{Name: scope.Name, Description: scope.Description, Hints: hints}

			if hints != nil && hints.Changes == "none" {
				defaults = append(defaults, j)
			}
		}

		reason := "Choose what the agent may do on " + text.Line(api.Identifier) + "."
		if len(defaults) > 0 {
			reason += " The scopes your hints say change nothing are selected to begin with."
		}

		title := "Scopes to grant on " + text.Line(api.Identifier)

		var picked []int

		err := screen.ask(title, reason, func() (err error) {
			picked, err = screen.asker.chooseMany(screen, title, screen.ScopeRows(choices), defaults)

			return err
		})
		if err != nil {
			return nil, err
		}

		if len(picked) == 0 {
			return nil, validationError{err: fmt.Errorf("choose at least one scope to grant on %s", api.Identifier), reason: "scopes_required"}
		}

		grants[i] = agents.Grant{Audience: api.Identifier, Scopes: make([]string, len(picked))}
		for j, index := range picked {
			grants[i].Scopes[j] = api.Scopes[index].Name
		}
	}

	return grants, nil
}

func runPoppyAgentsList(cmd *cobra.Command, cli *cli, all bool) error {
	if err := cli.checkPoppyScopes(poppyListScopes); err != nil {
		return err
	}

	var list []agents.Agent
	err := ansi.Waiting(func() (err error) {
		list, err = cli.poppyAgents().List(cmd.Context(), all)

		return err
	})
	if err != nil {
		return cli.poppyError(err)
	}

	cli.renderer.PoppyAgentList(list)

	return nil
}

func runPoppyAgentsShow(cmd *cobra.Command, cli *cli, args []string) error {
	ref, err := poppyRef(cmd, cli.newPoppyScreen(""), poppyAgentRef, args)
	if err != nil {
		return err
	}

	if err := cli.checkPoppyScopes(poppyShowScopes); err != nil {
		return err
	}

	var detail *agents.Detail
	err = ansi.Waiting(func() (err error) {
		detail, err = cli.poppyAgents().Show(cmd.Context(), ref)

		return err
	})
	if err != nil {
		return cli.poppyError(err)
	}

	cli.renderer.PoppyAgentShow(detail)

	return nil
}

func runPoppyAgentsRemove(cmd *cobra.Command, cli *cli, inputs poppyRemoveInputs, args []string) error {
	screen := cli.newPoppyScreen("")

	ref, err := poppyRef(cmd, screen, poppyAgentRef, args)
	if err != nil {
		return err
	}

	if err := cli.checkPoppyScopes(poppyRemoveScopes); err != nil {
		return err
	}

	ctx := cmd.Context()
	svc := cli.poppyAgents()

	var removal *agents.Removal
	err = ansi.Waiting(func() (err error) {
		removal, err = svc.PlanRemoval(ctx, ref)

		return err
	})
	if err != nil {
		var notPoppy *agents.NotPoppyAgentError
		if errors.As(err, &notPoppy) {
			return validationError{err: err, reason: "not_poppy_agent"}
		}

		return cli.poppyError(err)
	}

	decision := agents.Decide(removal, inputs.RemoveAllGrants, cli.force, poppyAsk.canPrompt(cmd))
	opts := agents.RemoveOptions{Everything: decision.Everything}
	plan := svc.Plan(removal, opts)

	if removal.Gone && cli.renderer.PoppyHuman() {
		cli.renderer.PoppyRemovalGone(ref)
	}

	if inputs.DryRun {
		if !cli.renderer.PoppyJSON(plan) {
			cli.renderer.PoppyRemoval(display.PoppyRemovalWould, plan)
		}

		return nil
	}

	switch decision.Approval {
	case agents.Refuse:
		if cli.renderer.PoppyHuman() {
			cli.renderer.PoppyRemoval(display.PoppyRemovalWill, plan)
		}

		details, _ := json.Marshal(plan)

		return validationError{
			err:     errors.New("confirmation required: re-run with --force to remove the agent"),
			details: details,
			reason:  "confirmation_required",
		}
	case agents.Ask, agents.Choose:
		cli.renderer.PoppyRemoval(display.PoppyRemovalWill, plan)

		proceed, everything, err := askPoppyRemoval(screen, decision.Approval == agents.Choose)
		if err != nil {
			return err
		}

		if !proceed {
			cli.renderer.PoppyDeclined("Nothing was removed.")

			return nil
		}
		opts.Everything = everything
	}

	result, err := svc.Remove(ctx, removal, opts)
	if err != nil {
		return cli.poppyError(err)
	}

	if cli.renderer.PoppyJSON(result) {
		return nil
	}

	cli.renderer.PoppyRemoved(result, removal.Gone)

	return nil
}

// askPoppyRemoval asks the user to confirm a removal. With choose set, the app
// has other grants, so it offers to remove only the Poppy access.
func askPoppyRemoval(screen *poppyScreen, choose bool) (proceed, everything bool, err error) {
	if !choose {
		proceed, err = screen.confirmAsk("Remove the agent",
			"This removes the grants of the agent, and its record and app. Deleting the app revokes its refresh tokens. Access tokens already issued stay valid until they expire.", "Remove this agent?")

		return proceed, true, err
	}

	const (
		onlyPoppy  = "Remove only the Poppy access"
		all        = "Remove everything"
		cancelling = "Cancel"
	)

	options := []string{onlyPoppy, all, cancelling}

	var picked int

	err = screen.ask("Other grants of the agent app",
		"The agent app has grants to APIs other than Poppy ones. Removing only the Poppy access leaves the app and those grants, and does not revoke the refresh tokens of the app.", func() (err error) {
			picked, err = screen.asker.choose(screen, "What should be removed?", options, 0)

			return err
		})
	if err != nil {
		return false, false, err
	}

	return options[picked] != cancelling, options[picked] == all, nil
}
