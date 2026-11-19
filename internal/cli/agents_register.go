package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/spf13/cobra"

	coreagents "github.com/auth0/auth0-cli/internal/agents"
	poppyagents "github.com/auth0/auth0-cli/internal/agents/poppy"
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/text"
)

// agentArgument is an argument of the agents commands that a terminal asks
// for when it is not given.
type agentArgument struct {
	// Title and reason are drawn above the question, and label is where the answer is
	// typed.
	title, reason, label string
}

var agentURLArgument = agentArgument{
	title:  "Agent URL",
	reason: "URL of the agent's metadata document.",
	label:  "URL",
}

// agentToUnregister is the argument of unregister: a terminal picks it from the
// agents that have an agent app.
var agentToUnregister = Argument{
	Name: "Agent",
	Help: "URL or ID of the agent to unregister.",
}

var (
	agentsAPIFlag = Flag{
		Name:     "API",
		LongForm: "api",
		Help:     "An API to grant, by ID or identifier. Repeat for several. In a terminal, without this or --poppy, you are asked which APIs to grant.",
	}
	agentsPoppyRegister = Flag{
		Name:     "Poppy",
		LongForm: "poppy",
		Help:     "Limit the APIs to Poppy APIs. On its own it grants every Poppy API of the tenant. With --api, every API named must be a Poppy API.",
	}
	agentsPoppyUnregister = Flag{
		Name:     "Poppy",
		LongForm: "poppy",
		Help:     "Remove only the grants on Poppy APIs. The agent record and the app are removed only when no other grant remains.",
	}
	agentsPoppyList = Flag{
		Name:     "Poppy",
		LongForm: "poppy",
		Help:     "Show only the grants on Poppy APIs.",
	}
	agentsPoppyShow = Flag{
		Name:     "Poppy",
		LongForm: "poppy",
		Help:     "Show only the grants on Poppy APIs.",
	}
	agentsNameFlag = Flag{
		Name:     "Name",
		LongForm: "name",
		Help:     "The agent's name.",
	}
	agentsConfigFlag = Flag{
		Name:     "Config",
		LongForm: "config",
		Help:     "Config file to read scope hints from. Used only if it exists.",
	}
	agentsGrantScopes = Flag{
		Name:     "Scopes",
		LongForm: "scopes",
		Help: "Scopes to grant. Repeat the flag or separate scopes with commas. With several APIs, each API gets the listed scopes it offers; " +
			"to give a scope to one API only, write <api>=<scope>. Without a terminal, pass this or --all-scopes. Mutually exclusive with --all-scopes.",
	}
	agentsAllScopes = Flag{
		Name:     "All Scopes",
		LongForm: "all-scopes",
		Help:     "Grant every offered scope. Mutually exclusive with --scopes.",
	}
	agentsRequireDPoP = Flag{
		Name:     "Require DPoP",
		LongForm: "require-dpop",
		Help:     "Require DPoP-bound tokens for the agent. Off by default.",
	}
)

// poppyBrowserSignInScope is the scope that lets an agent browse a website for a person.
const poppyBrowserSignInScope = "poppy:browser-sign-in"

// Management API scopes each agents command needs, checked before it changes anything.
var (
	agentRegisterScopes = []string{
		"read:clients", "create:clients", "update:clients", "read:agents", "create:agents",
		"read:client_grants", "create:client_grants", "read:resource_servers",
	}
	agentsUnregisterScopes = []string{
		"read:agents", "read:clients", "read:client_grants",
		"delete:client_grants", "delete:agents", "delete:clients",
	}
	// The scopes of the extra columns of list and show. Show also checks the keys
	// Auth0 stored. Narrowing to the Poppy APIs reads the resource servers.
	agentsListScopes = []string{"read:clients", "read:client_grants"}
	agentsShowScopes = []string{"read:clients", "read:client_grants", "read:client_credentials"}
)

// agentUnregisterScopes are the scopes of unregister. Telling the Poppy APIs from
// the others reads the resource servers, which only --poppy does.
func agentUnregisterScopes(poppy bool) []string {
	if !poppy {
		return agentsUnregisterScopes
	}

	return append(slices.Clone(agentsUnregisterScopes), "read:resource_servers")
}

type agentRegisterInputs struct {
	APIs        []string
	Poppy       bool
	Name        string
	Config      string
	Scopes      []string
	AllScopes   bool
	RequireDPoP bool
}

func registerAgentCmd(cli *cli) *cobra.Command {
	var inputs agentRegisterInputs

	cmd := &cobra.Command{
		Use:   "register [agent-url]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Register an agent so it can sign in to your company",
		Long: poppyLong("Registers an agent from the URL of its metadata document. The command previews the document, " +
			"creates or updates the agent app, creates the agent record and grants the agent the APIs you choose. " +
			"It changes the tenant only, and does not write the config file.\n\n" +
			"Pass `--api` for each API to grant, by ID or identifier, or `--poppy` to grant every Poppy API of the tenant. " +
			"In a terminal, without either, it asks which APIs to grant. Choose the scopes with `--scopes` or `--all-scopes`; " +
			"in a terminal it asks otherwise. To let the agent browse your website on behalf of a person, " +
			"include the scope `poppy:browser-sign-in`. DPoP is required only with `--require-dpop`.\n\n" +
			"In a terminal the command shows what it will do and asks first; `--force` skips the question. " +
			"Without a terminal it applies the change. With `--poppy`, or `--config`, it reads the config file, if there is one, " +
			"to show the hints of the scopes on offer.\n\n" +
			"Running it again refreshes the agent app from the current document and changes nothing else, " +
			"which is also how you pick up keys the agent has rotated."),
		Example: `  auth0 agents register https://muse.example/agent.json
  auth0 agents register https://muse.example/agent.json --api https://api.shop.example --scopes orders:read,poppy:browser-sign-in
  auth0 agents register https://muse.example/agent.json --api https://api.shop.example --api https://api.crm.example --scopes https://api.shop.example=orders:read,https://api.crm.example=contacts:read
  auth0 agents register https://muse.example/agent.json --poppy --all-scopes
  auth0 agents register https://muse.example/agent.json --api https://api.shop.example --all-scopes --require-dpop`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgentRegister(cmd, cli, inputs, args)
		},
	}

	agentsAPIFlag.RegisterStringArray(cmd, &inputs.APIs, nil)
	agentsPoppyRegister.RegisterBool(cmd, &inputs.Poppy, false)
	agentsNameFlag.RegisterString(cmd, &inputs.Name, "")
	agentsConfigFlag.RegisterString(cmd, &inputs.Config, config.DefaultFileName)
	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")
	agentsGrantScopes.RegisterStringSlice(cmd, &inputs.Scopes, nil)
	agentsAllScopes.RegisterBool(cmd, &inputs.AllScopes, false)
	agentsRequireDPoP.RegisterBool(cmd, &inputs.RequireDPoP, false)
	registerPoppyOutputFlags(cmd, cli)
	cmd.MarkFlagsMutuallyExclusive(agentsGrantScopes.LongForm, agentsAllScopes.LongForm)

	return cmd
}

type agentUnregisterInputs struct {
	DryRun bool
	Poppy  bool
}

func unregisterAgentCmd(cli *cli) *cobra.Command {
	var inputs agentUnregisterInputs

	cmd := &cobra.Command{
		Use:   "unregister [agent-url|agent-id]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Unregister an agent and remove its API access",
		Long: poppyLong("Unregisters the agent and removes its API access: all of its grants, then its agent record and its app. " +
			"It changes the tenant only, and does not touch the config file. Registering the agent again creates a new app " +
			"and its users must sign in again.\n\n" +
			"With `--poppy` only the grants on your Poppy APIs are removed, and the " +
			"agent record and the app are removed only when the app has no other grants; otherwise the other grants are listed. " +
			"Deleting the app revokes its refresh tokens; when the app is kept, " +
			"its refresh tokens are not revoked. Access tokens already issued stay valid until they expire.\n\n" +
			"Without an argument, a terminal asks you to pick the agent. " +
			"Use `--dry-run` to print what would be removed, and `--force` to skip the confirmation."),
		Example: `  auth0 agents unregister
  auth0 agents unregister https://muse.example/agent.json
  auth0 agents unregister https://muse.example/agent.json --dry-run
  auth0 agents unregister https://muse.example/agent.json --poppy
  auth0 agents unregister <agent-id> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgentUnregister(cmd, cli, inputs, args)
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")
	poppyDryRun.RegisterBool(cmd, &inputs.DryRun, false)
	agentsPoppyUnregister.RegisterBool(cmd, &inputs.Poppy, false)
	registerPoppyOutputFlags(cmd, cli)

	return cmd
}

// agentRef returns the argument given, or asks for it in a terminal.
func agentRef(cmd *cobra.Command, screen *questionScreen, argument agentArgument, args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}

	if !promptAsk.canPrompt(cmd) {
		return "", usageError{err: fmt.Errorf("missing a required argument: %s", argument.title), reason: "missing_required_args"}
	}

	return screen.askLine(argument.title, argument.reason, argument.label, "")
}

// readPoppyConfig reads the config file, or returns nil when there is none.
func readPoppyConfig(path string) (*config.Config, error) {
	cfg, err := config.Read(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	return cfg, err
}

// readScopeHints reads the config file for the hints of the scopes. It is read
// with --poppy, or when --config is given. A file that cannot be read is
// reported and left out, as the hints are only a help.
func (c *cli) readScopeHints(cmd *cobra.Command, inputs agentRegisterInputs) *config.Config {
	if !inputs.Poppy && !cmd.Flags().Changed(agentsConfigFlag.LongForm) {
		return nil
	}

	cfg, err := readPoppyConfig(inputs.Config)
	if err != nil {
		c.renderer.Warnf("The scope hints in %s are not used, because the file could not be read: %v", text.Line(inputs.Config), err)

		return nil
	}

	return cfg
}

func poppyScopeHints(cfg *config.Config, scope string) *config.Hints {
	if cfg == nil {
		return nil
	}

	return cfg.Scopes[scope].Hints
}

func runAgentRegister(cmd *cobra.Command, cli *cli, inputs agentRegisterInputs, args []string) error {
	screen := cli.newPromptScreen("")

	agentURL, err := agentRef(cmd, screen, agentURLArgument, args)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(agentURL, "https://") {
		return validationError{err: fmt.Errorf("the agent URL must start with https://: %s", agentURL), reason: "invalid_agent_url"}
	}

	if err := cli.checkPoppyScopes(agentRegisterScopes); err != nil {
		return err
	}

	ctx := cmd.Context()
	svc := cli.agentsService()

	// Only a terminal can be asked which APIs to grant.
	pickAPIs := len(inputs.APIs) == 0 && !inputs.Poppy
	if pickAPIs && !promptAsk.canPrompt(cmd) {
		return cli.apisRequiredError(ctx, svc)
	}

	cfg := cli.readScopeHints(cmd, inputs)
	mapError := cli.agentsErrorFor(inputs.Poppy)

	var preview *coreagents.Preview

	err = ansi.Waiting(func() (err error) {
		preview, err = svc.Preview(ctx, agentURL)

		return err
	})
	if err != nil {
		var invalid *coreagents.InvalidDocumentError
		if errors.As(err, &invalid) {
			return invalidDocumentFailure(invalid)
		}

		return mapError(err)
	}

	var (
		apis     []coreagents.API
		grants   []coreagents.Grant
		scopeErr error
	)

	if pickAPIs {
		// A person is asked, so the metadata of the agent comes first.
		cli.renderer.AgentPreview(preview)

		var confirmed bool
		if apis, confirmed, err = cli.askAgentAPIs(ctx, screen, svc, cli.force); err != nil {
			return err
		}

		if !confirmed {
			cli.renderer.PoppyDeclined("Nothing was registered.")

			return nil
		}
	} else if apis, err = cli.pickAgentAPIs(ctx, svc, inputs); err != nil {
		return mapError(err)
	}

	// An agent with no API has no scopes to choose.
	if len(apis) > 0 {
		grants, scopeErr = coreagents.ChooseScopes(apis, inputs.Scopes, inputs.AllScopes)
	} else {
		grants = []coreagents.Grant{}
	}

	// With --json, only a person at a terminal who is asked something sees the
	// metadata of the agent.
	asking := promptAsk.canPrompt(cmd) && (!cli.force || errors.Is(scopeErr, coreagents.ErrScopesRequired))
	if !pickAPIs && (cli.renderer.PoppyHuman() || asking) {
		cli.renderer.AgentPreview(preview)
	}

	var unknownScope *coreagents.UnknownScopeError
	var noScopes *coreagents.NoScopesError

	switch {
	case errors.Is(scopeErr, coreagents.ErrScopesRequired) && promptAsk.canPrompt(cmd):
		grants, err = promptAgentGrants(screen, apis, cfg)
	case errors.Is(scopeErr, coreagents.ErrScopesRequired):
		return scopesRequiredError("pass --scopes or --all-scopes to choose the scopes to grant.", apis, cfg)
	case errors.As(scopeErr, &unknownScope):
		return validationError{err: scopeErr, reason: "unknown_scope"}
	case errors.As(scopeErr, &noScopes):
		return scopesRequiredError(scopeErr.Error()+"; pass --scopes with one of its scopes, or --all-scopes.", []coreagents.API{noScopes.API}, cfg)
	}
	if err != nil {
		return err
	}

	if promptAsk.canPrompt(cmd) && !cli.force {
		cli.renderer.AgentRegisterPlan(preview, inputs.Name, inputs.RequireDPoP, grants)

		proceed, err := screen.confirmAsk("Register the agent", "This changes your Auth0 tenant: it creates or updates the agent app and grants it your APIs.", "Register this agent?")
		if err != nil {
			return err
		}

		if !proceed {
			cli.renderer.PoppyDeclined("Nothing was registered.")

			return nil
		}
	}

	var reg *coreagents.Registered

	err = ansi.Waiting(func() (err error) {
		reg, err = svc.Register(ctx, coreagents.RegisterRequest{
			Preview:     preview,
			Name:        inputs.Name,
			RequireDPoP: inputs.RequireDPoP,
			Grants:      grants,
		})

		return err
	})
	if err != nil {
		return cli.registerFailure(err, cli.registerCommand(agentURL, inputs, grants), inputs.Poppy)
	}

	if cli.renderer.PoppyJSON(reg) {
		return nil
	}

	cli.renderer.AgentRegistration(reg)

	return nil
}

// registerFailure maps the failure of a registration. An agent app that is
// linked to another agent record than the one that exists for its URL is told
// apart; the command that continues a partial failure is rerun.
func (c *cli) registerFailure(err error, rerun string, poppy bool) error {
	var mismatch *coreagents.LinkMismatchError
	if !errors.As(err, &mismatch) {
		return c.agentsFailure(err, rerun, poppy)
	}

	fields := map[string]any{"next": c.command("apps", "update", mismatch.ClientID, "--agent-id", mismatch.RecordID)}

	var partial *coreagents.PartialError
	if errors.As(err, &partial) {
		fields["applied"], fields["failed"], fields["remaining"] = partial.Applied, partial.Failed, nonNil(partial.Remaining)
	}

	details, _ := json.Marshal(fields)

	return validationError{err: err, details: details, reason: "agent_link_mismatch"}
}

// registerCommand is the command that registers the agent again with the
// choices made, which continues a registration that stopped partway.
func (c *cli) registerCommand(agentURL string, inputs agentRegisterInputs, grants []coreagents.Grant) string {
	args := []string{"agents", "register", agentURL}

	var scopes []string

	for _, grant := range grants {
		args = append(args, "--api", grant.Audience)

		for _, scope := range grant.Scopes {
			// With several APIs, a scope is pinned to its API.
			if len(grants) > 1 {
				scope = grant.Audience + "=" + scope
			}

			scopes = append(scopes, scope)
		}
	}

	if len(scopes) > 0 {
		args = append(args, "--scopes", strings.Join(scopes, ","))
	}

	if inputs.Name != "" {
		args = append(args, "--name", inputs.Name)
	}

	if inputs.RequireDPoP {
		args = append(args, "--require-dpop")
	}

	return c.command(args...)
}

// command writes a command of the CLI for a terminal, for the tenant in use.
func (c *cli) command(args ...string) string {
	words := []string{"auth0"}
	for _, arg := range args {
		words = append(words, text.Quote(arg))
	}

	if c.tenant != "" {
		words = append(words, "--tenant", text.Quote(c.tenant))
	}

	return strings.Join(words, " ")
}

// nonNil turns a missing list into an empty one, so it is [] in the JSON.
func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}

	return values
}

// apiOffer is an API of the tenant, with the scopes it offers, in the details of
// an error.
type apiOffer struct {
	Audience string       `json:"audience"`
	Name     string       `json:"name"`
	Poppy    bool         `json:"poppy"`
	Scopes   []scopeOffer `json:"scopes"`
}

type scopeOffer struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// apisRequiredError tells a caller that cannot be asked to name the APIs to
// grant. Its details list every API with its scopes, so the caller can choose in
// one step. When the APIs cannot be listed, there are no details.
func (c *cli) apisRequiredError(ctx context.Context, svc *coreagents.Service) error {
	err := validationError{
		err:    errors.New("pass --api to choose the APIs to grant, or --poppy to grant every Poppy API of the tenant"),
		reason: "apis_required",
	}

	apis, poppy, listErr := c.listAgentAPIs(ctx, svc)
	if listErr != nil {
		return err
	}

	offers := make([]apiOffer, len(apis))
	for i, api := range apis {
		offers[i] = apiOffer{Audience: api.Identifier, Name: api.Name, Poppy: poppy[api.Identifier], Scopes: make([]scopeOffer, len(api.Scopes))}

		for j, scope := range api.Scopes {
			offers[i].Scopes[j] = scopeOffer{Name: scope.Name, Description: scope.Description}
		}
	}

	err.details, _ = json.Marshal(map[string]any{"apis": offers})

	return err
}

// listAgentAPIs lists the APIs of the tenant once, without the Management API,
// with the identifiers of the ones that are Poppy APIs.
func (c *cli) listAgentAPIs(ctx context.Context, svc *coreagents.Service) (apis []coreagents.API, poppy map[string]bool, err error) {
	var servers []*managementv3.ResourceServer

	err = ansi.Waiting(func() (err error) {
		servers, err = svc.ListServers(ctx)

		return err
	})
	if err != nil {
		return nil, nil, err
	}

	poppy = make(map[string]bool)
	for _, server := range servers {
		api := coreagents.NewAPI(server)

		// The Management API is the tenant's own, not one an agent is granted.
		if isPoppyManagementAPI(api.Identifier) {
			continue
		}

		apis = append(apis, api)
		poppy[api.Identifier] = server.GetUseForPersonalAgentProtocol()
	}

	return apis, poppy, nil
}

// pickAgentAPIs returns the APIs named with --api, or every Poppy API with
// --poppy and no --api. With --poppy, every API named must be a Poppy API.
func (c *cli) pickAgentAPIs(ctx context.Context, svc *coreagents.Service, inputs agentRegisterInputs) (apis []coreagents.API, err error) {
	err = ansi.Waiting(func() (err error) {
		apis, err = c.pickAPIs(ctx, svc, inputs)

		return err
	})

	return apis, err
}

func (c *cli) pickAPIs(ctx context.Context, svc *coreagents.Service, inputs agentRegisterInputs) ([]coreagents.API, error) {
	if inputs.Poppy {
		return c.poppyAgents().PoppyAPIs(ctx, inputs.APIs)
	}

	all, err := svc.APIs(ctx)
	if err != nil {
		return nil, err
	}

	return coreagents.PickAPIs(all, inputs.APIs)
}

// askAgentAPIs asks which of the tenant's APIs to grant, with the Poppy ones
// marked. Choosing none is allowed once confirmed, or with force; confirmed is
// false when the user declines that.
func (c *cli) askAgentAPIs(ctx context.Context, screen *questionScreen, svc *coreagents.Service, force bool) (apis []coreagents.API, confirmed bool, err error) {
	all, poppy, err := c.listAgentAPIs(ctx, svc)
	if err != nil {
		return nil, false, c.agentsError(err)
	}

	if len(all) > 0 {
		if apis, err = promptAgentAPIs(screen, all, poppy); err != nil {
			return nil, false, err
		}
	}

	if len(apis) > 0 || force {
		return apis, true, nil
	}

	confirmed, err = screen.confirmAsk("No API chosen",
		"The agent app and the agent record are created without access to any API.", "Register the agent without access to any API?")

	return nil, confirmed, err
}

// promptAgentAPIs asks which APIs to grant, none selected to begin with. The
// APIs in poppy are marked.
func promptAgentAPIs(screen *questionScreen, all []coreagents.API, poppy map[string]bool) ([]coreagents.API, error) {
	rows := make([]string, len(all))
	for i, api := range all {
		rows[i] = text.Line(api.Identifier)
		if poppy[api.Identifier] {
			rows[i] += "  (Poppy)"
		}
	}

	const title = "APIs to grant"

	var picked []int

	err := screen.ask(title, "Choose the APIs the agent may use.", func() (err error) {
		picked, err = screen.asker.chooseMany(title, rows, nil)

		return err
	})
	if err != nil {
		return nil, err
	}

	apis := make([]coreagents.API, len(picked))
	for i, index := range picked {
		apis[i] = all[index]
	}

	return apis, nil
}

// scopesRequiredError lists the scopes on offer, with their hints, for a caller
// that cannot be asked. The details list the scopes of each API.
func scopesRequiredError(reason string, apis []coreagents.API, cfg *config.Config) error {
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
	message.WriteString(reason)
	message.WriteString(" Offered scopes:")

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

// promptAgentGrants asks which scopes to grant on each API. Scopes that only
// read are selected to begin with.
func promptAgentGrants(screen *questionScreen, apis []coreagents.API, cfg *config.Config) ([]coreagents.Grant, error) {
	grants := make([]coreagents.Grant, len(apis))

	for i, api := range apis {
		choices := make([]display.ScopeChoice, len(api.Scopes))

		var defaults []int
		for j, scope := range api.Scopes {
			hints := poppyScopeHints(cfg, scope.Name)
			choices[j] = display.ScopeChoice{Name: scope.Name, Description: scope.Description, Hints: hints}

			if hints != nil && hints.Changes == "none" {
				defaults = append(defaults, j)
			}
		}

		reason := "Choose what the agent may do on " + text.Line(api.Identifier) + "."
		if len(defaults) > 0 {
			reason += " The scopes your hints say change nothing are selected to begin with."
		}

		if slices.ContainsFunc(api.Scopes, func(scope coreagents.Scope) bool { return scope.Name == poppyBrowserSignInScope }) {
			reason += " Include " + poppyBrowserSignInScope + " if the agent should browse your website on behalf of a person."
		}

		title := "Scopes to grant on " + text.Line(api.Identifier)

		var picked []int

		err := screen.ask(title, reason, func() (err error) {
			picked, err = screen.asker.chooseMany(title, screen.ScopeRows(choices), defaults)

			return err
		})
		if err != nil {
			return nil, err
		}

		if len(picked) == 0 {
			return nil, validationError{err: fmt.Errorf("choose at least one scope to grant on %s", api.Identifier), reason: "scopes_required"}
		}

		grants[i] = coreagents.Grant{Audience: api.Identifier, Scopes: make([]string, len(picked))}
		for j, index := range picked {
			grants[i].Scopes[j] = api.Scopes[index].Name
		}
	}

	return grants, nil
}

func runAgentUnregister(cmd *cobra.Command, cli *cli, inputs agentUnregisterInputs, args []string) error {
	screen := cli.newPromptScreen("")

	var ref string
	if len(args) == 1 {
		ref = args[0]
	} else if err := agentToUnregister.Pick(cmd, &ref, cli.unregisterPickerOptions(inputs.Poppy)); err != nil {
		return err
	}

	if err := cli.checkPoppyScopes(agentUnregisterScopes(inputs.Poppy)); err != nil {
		return err
	}

	ctx := cmd.Context()
	svc := cli.agentsService()
	mapError := cli.agentsErrorFor(inputs.Poppy)

	var removal *coreagents.Removal
	err := ansi.Waiting(func() (err error) {
		removal, err = svc.PlanRemoval(ctx, ref, cli.agentAudiences(inputs.Poppy))

		return err
	})
	if err != nil {
		var notAgent *coreagents.NotAnAgentError
		if errors.As(err, &notAgent) {
			return notAnAgentError(cli, notAgent, ref)
		}

		return mapError(err)
	}

	decision := coreagents.Decide(removal, cli.force, promptAsk.canPrompt(cmd))
	opts := coreagents.RemoveOptions{DeleteApp: decision.DeleteApp}
	plan := svc.Plan(removal, opts)

	if removal.Gone && cli.renderer.PoppyHuman() {
		cli.renderer.AgentRemovalGone(ref)
	}

	if inputs.DryRun {
		if !cli.renderer.PoppyJSON(plan) {
			cli.renderer.AgentRemoval(display.AgentRemovalWould, plan)
		}

		return nil
	}

	switch decision.Approval {
	case coreagents.Refuse:
		if cli.renderer.PoppyHuman() {
			cli.renderer.AgentRemoval(display.AgentRemovalWill, plan)
		}

		details, _ := json.Marshal(plan)

		return validationError{
			err:     errors.New("confirmation required: re-run with --force to unregister the agent"),
			details: details,
			reason:  "confirmation_required",
		}
	case coreagents.Ask:
		cli.renderer.AgentRemoval(display.AgentRemovalWill, plan)

		proceed, err := askUnregister(screen, decision.DeleteApp)
		if err != nil {
			return err
		}

		if !proceed {
			cli.renderer.PoppyDeclined("Nothing was unregistered.")

			return nil
		}
	case coreagents.Proceed:
	}

	result, err := svc.Remove(ctx, removal, opts)
	if err != nil {
		return cli.agentsFailure(err, cli.unregisterCommand(ref, inputs.Poppy), inputs.Poppy)
	}

	if cli.renderer.PoppyJSON(result) {
		return nil
	}

	cli.renderer.AgentRemoved(result, removal.Gone)

	return nil
}

// unregisterCommand is the command that unregisters the agent again without
// asking, which continues a removal that stopped partway.
func (c *cli) unregisterCommand(ref string, poppy bool) string {
	args := []string{"agents", "unregister", ref}
	if poppy {
		args = append(args, "--poppy")
	}

	return c.command(append(args, "--force")...)
}

// notAnAgentError tells why nothing was unregistered, and what to do instead.
func notAnAgentError(c *cli, notAgent *coreagents.NotAnAgentError, ref string) error {
	if notAgent.NoGrants {
		details, _ := json.Marshal(map[string]any{"next": c.command("agents", "unregister", ref)})

		return validationError{
			err:     fmt.Errorf("agent %s has no grant on a Poppy API, so nothing was removed. To unregister it completely, run it again without --poppy", ref),
			details: details,
			reason:  "no_poppy_grant",
		}
	}

	var details json.RawMessage
	if notAgent.AgentID != "" {
		details, _ = json.Marshal(map[string]any{"next": c.command("agents", "delete", notAgent.AgentID)})
	}

	return validationError{err: notAgent, details: details, reason: "not_an_agent"}
}

// unregisterPickerOptions lists the agents that unregister can remove, those
// with an agent app. With poppy, those that have grants but none on a Poppy API
// are left out.
func (c *cli) unregisterPickerOptions(poppy bool) pickerOptionsFunc {
	return func(ctx context.Context) (pickerOptions, error) {
		entries, err := c.agentsService().Candidates(ctx, c.agentAudiences(poppy))
		if err != nil {
			return nil, c.agentsErrorFor(poppy)(err)
		}

		var opts pickerOptions

		for _, e := range entries {
			url := e.Client.GetExternalClientID()

			name := e.Client.GetName()
			if e.Record != nil {
				name = e.Record.GetName()
			}

			opts = append(opts, pickerOption{value: url, label: fmt.Sprintf("%s %s", text.Line(name), ansi.Faint("("+text.Line(url)+")"))})
		}

		if len(opts) == 0 {
			return nil, errors.New("no agents to unregister. Register one by running: `auth0 agents register`")
		}

		return opts, nil
	}
}

// askUnregister asks the user to confirm unregistering an agent. Without
// deleteApp the app has other grants, so only the grants go and the app stays.
func askUnregister(screen *questionScreen, deleteApp bool) (bool, error) {
	if deleteApp {
		return screen.confirmAsk("Unregister the agent",
			"This removes the grants of the agent, and its record and app. Deleting the app revokes its refresh tokens. Access tokens already issued stay valid until they expire.", "Unregister this agent?")
	}

	return screen.confirmAsk("Unregister the agent",
		"This removes the grants of the agent on Poppy APIs. The agent record and app stay, because the app has other grants, and the refresh tokens of the app are not revoked.", "Unregister this agent?")
}

// agentsError maps a 403 from the Management API that names a missing scope to
// the login command that adds it. Any other error passes through as it is.
func (c *cli) agentsError(err error) error {
	if _, scopeErr := forbiddenScopeError(err); scopeErr != nil {
		return scopeErr
	}

	return err
}

// agentsErrorFor is the error mapping of an agents command. Only a command
// limited to Poppy APIs explains a 403 as the tenant lacking a Poppy feature.
func (c *cli) agentsErrorFor(poppy bool) func(error) error {
	if poppy {
		return c.poppyError
	}

	return c.agentsError
}

// agentsFailure maps the failure of a command that changes the tenant. A failure
// after the first change is a partial failure: its details list what was done,
// what failed and what is left, and next is the command that continues it.
func (c *cli) agentsFailure(err error, next string, poppy bool) error {
	mapError := c.agentsErrorFor(poppy)

	var partial *coreagents.PartialError
	if !errors.As(err, &partial) {
		return mapError(err)
	}

	partial.Err = mapError(partial.Err)

	details, _ := json.Marshal(map[string]any{
		"applied":   nonNil(partial.Applied),
		"failed":    partial.Failed,
		"remaining": nonNil(partial.Remaining),
		"next":      next,
	})

	// The cause is part of the message but not of the error chain, so that the
	// class of the cause does not hide that the command stopped partway.
	return validationError{err: errors.New(partial.Error()), details: details, reason: "partial_failure"}
}

// grantedManagementScopes returns the scopes of the saved login, or nil when
// they cannot be told.
func (c *cli) grantedManagementScopes() []string {
	if envAuthEnabled(os.Getenv) {
		return nil
	}

	tenant, err := c.Config.GetTenant(c.tenant)
	if err != nil {
		return nil
	}

	return scopesFromToken(tenant.GetAccessToken())
}

// warnUnreadable tells, once, that some values show as unknown because a read
// failed. A refused read points to the login command that adds the scopes
// missing from the saved login, among required. Other failures are told only
// when always is set, because the caller has no other way to report them.
func (c *cli) warnUnreadable(err error, required []string, always bool) {
	if err == nil {
		return
	}

	forbidden := false

	for _, e := range unjoin(err) {
		if ok, _ := forbiddenScopeError(e); ok {
			forbidden = true
		}
	}

	switch {
	case forbidden:
		missing := missingScopes(c.grantedManagementScopes(), required)
		if len(missing) == 0 {
			missing = required
		}

		c.renderer.AgentUnreadable(fmt.Sprintf("Some values show as unknown because the Management API refused to read them. Check that your login has the scopes %s. Run: auth0 login --scopes %s",
			strings.Join(missing, ", "), strings.Join(missing, ",")))
	case always:
		c.renderer.AgentUnreadable(fmt.Sprintf("Some values show as unknown because they could not be read: %v", err))
	}
}

// unjoin lists the errors an error joins, or the error itself.
func unjoin(err error) []error {
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		return joined.Unwrap()
	}

	return []error{err}
}

// agentsService builds the agents service over the Management API clients.
func (c *cli) agentsService() *coreagents.Service {
	return &coreagents.Service{
		Clients:         c.apiv3.ClientV3,
		Agents:          c.apiv3.Agents,
		Credentials:     c.apiv3.ClientCredential,
		Grants:          c.apiv3.ClientGrant,
		ResourceServers: c.apiv3.ResourceServerV3,
		Tenant:          c.tenant,
	}
}

// poppyAgents builds the Poppy layer over the agents service.
func (c *cli) poppyAgents() *poppyagents.Service {
	return &poppyagents.Service{Service: c.agentsService()}
}

// agentAudiences narrows what the agents commands read to the Poppy APIs when
// poppy is set. Without it nothing narrows, so the resource servers are not read.
func (c *cli) agentAudiences(poppy bool) coreagents.AudienceFilter {
	if !poppy {
		return nil
	}

	return c.poppyAgents().Audiences
}

// invalidDocumentFailure turns a metadata document that Auth0 rejects into a
// validation error. The message is one line and the violations are in the
// details, and the human output lists them after the message. The reason is
// agent_unreachable when the document could not be retrieved.
func invalidDocumentFailure(invalid *coreagents.InvalidDocumentError) error {
	details, _ := json.Marshal(map[string]any{"violations": invalid.Violations})

	reason := "agent_invalid"
	if invalid.Unreachable() {
		reason = "agent_unreachable"
	}

	err := apiResponseHintError{err: invalid, hint: poppyViolationsHint(invalid.Advice())}

	return validationError{err: err, details: details, reason: reason}
}

// poppyViolationsHint is the hint that lists what is wrong, one item per line,
// under the message of the error.
func poppyViolationsHint(advice string) string {
	lines := strings.Split(advice, "\n")
	for i := range lines {
		lines[i] = "  " + strings.TrimSpace(lines[i])
	}

	return strings.Join(lines, "\n")
}
