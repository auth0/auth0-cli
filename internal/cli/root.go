package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/analytics"
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/buildinfo"
	"github.com/auth0/auth0-cli/internal/config"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/instrumentation"
	"github.com/auth0/auth0-cli/internal/iostream"
)

const rootShort = "Build, manage and test your Auth0 integrations from the command line."

const rootLong = `Build, manage and test your Auth0 integrations from the command line.

## For Agents and Automation

The Auth0 CLI now includes features for AI agents and automation:

  • Schema Discovery: Use the '--schema' flag on a create/update command to print
    its request payload schema. Add '--json' for machine-readable output.
    Example: auth0 actions create --schema --json

  • JSON Input: Use '--data' flag for programmatic resource creation/updates
    Example: auth0 actions create --data @action.json

  • Schema Validation: JSON inputs are validated locally before API calls
    Example: auth0 actions create --data '{"name":"my-action",...}'

See 'auth0 <resource> --help' for details on specific resources.
For agent integration guide, visit: https://github.com/auth0/auth0-cli`

// agentModeHelp describes agent mode in one place. It is shown in the root help,
// both the human text and the JSON help an agent reads, so the CLI never has to
// re-announce the mode on every command it runs.
const agentModeHelp = `## Agent Mode

Agent mode makes every command's output machine-readable. The CLI enables it
automatically when it detects an AI agent. Force it with '--agent-mode' (or
AUTH0_AGENT_MODE=true) and turn it off with '--agent-mode=false' (or
AUTH0_AGENT_MODE=false).

In agent mode the CLI:

  • Prints results to stdout as JSON, and streams (for example 'auth0 logs tail')
    as newline-delimited JSON, one object per line.
  • Prints diagnostics to stderr as JSON lines ({"level","message"}) and errors as
    a JSON envelope ({"error":{"code","message","status","details"}}).
  • Disables interactive prompts and colors.
  • Exits with a code per failure class: 0 success, 1 generic, 2 usage, 3 auth,
    4 validation, 5 not-found, 6 rate-limit, 7 api, 130 interrupted.`

const panicMessage = `
!!     Uh oh. Something went wrong.
!!     If this problem keeps happening feel free to report an issue at
!!
!!     https://github.com/auth0/auth0-cli/issues/new/choose
`

var ciEnvironmentVariables = []string{
	"CI",
	"GITHUB_ACTIONS",
	"GITLAB_CI",
	"BUILDKITE",
	"CIRCLECI",
	"BUILD_ID",
	"JENKINS_URL",
	"TEAMCITY_VERSION",
	"TRAVIS",
	"TF_BUILD",
	"BITBUCKET_BUILD_NUMBER",
	"APPVEYOR",
	"DRONE",
	"CODEBUILD_BUILD_ID",
}

// Execute is the primary entrypoint of the CLI app.
func Execute() {
	cli := &cli{
		renderer: display.NewRenderer(),
		tracker:  analytics.NewTracker(),
	}

	// Prevent sorting of commands.
	cobra.EnableCommandSorting = false

	rootCmd := buildRootCmd(cli)
	rootCmd.SetUsageTemplate(namespaceUsageTemplate())

	// Wrap flag-parse errors so they map to the usage exit code (2).
	rootCmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return usageError{err}
	})

	addPersistentFlags(rootCmd, cli)
	addSubCommands(rootCmd, cli)

	enforceUnknownSubcommand(rootCmd)

	overrideHelpAndVersionFlagText(rootCmd)

	defer func() {
		if v := recover(); v != nil {
			err := fmt.Errorf("panic: %v", v)

			if instrumentation.ReportException(err) {
				fmt.Print(panicMessage) // If we're in development mode, we should throw the panic for so we have less surprises.
			} else {
				panic(v) // For non-developers, we'll swallow the panics.
			}
		}
	}()

	// Resolve agent mode for the pre-parse `--help` path; real commands re-apply the parsed flag in applyAgentModeDefaults.
	cli.agentMode = resolveAgentMode(cli.agentClientName(), os.Args[1:])

	if renderJSONHelpIfRequested(cli, rootCmd, os.Args[1:]) {
		return
	}

	// Platform specific terminal initialization:
	// this should run for all commands,
	// for most of the architectures there's no requirements.
	ansi.InitConsole()

	cancelCtx := contextWithCancel()
	err := rootCmd.ExecuteContext(cancelCtx)
	trackCommandOutcome(cli, err)

	timeoutCtx, cancel := context.WithTimeout(cancelCtx, 3*time.Second)
	defer cancel()
	cli.tracker.Wait(timeoutCtx) // No event should be tracked after this has run.

	if err != nil {
		if cli.wantsJSONError() {
			cli.renderer.ErrorJSON(buildErrorEnvelope(err))
		} else {
			renderErrorMessage(cli.renderer, err.Error())
		}

		instrumentation.ReportException(err)
		os.Exit(exitCodeForError(err)) // nolint:gocritic
	}
}

// wantsJSONError reports whether a failing command should emit the machine-readable
// JSON error envelope instead of a human message. It honors agent mode and the
// JSON output flags, and also covers usage/parse errors that fail before the
// renderer's format is configured.
func (c *cli) wantsJSONError() bool {
	return c.agentMode || c.json || c.jsonCompact ||
		c.renderer.Format == display.OutputFormatJSON ||
		c.renderer.Format == display.OutputFormatJSONCompact
}

func buildRootCmd(cli *cli) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "auth0",
		SilenceUsage:  true,
		SilenceErrors: true,
		Short:         rootShort,
		Long:          rootLong + "\n\n" + agentModeHelp + "\n\n" + getLogin(cli),
		Version:       buildinfo.GetVersionWithCommit(),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cli.executedCommandPath = cmd.CommandPath()

			applyAgentModeDefaults(cli, cmd)

			ansi.Initialize(cli.noColor)
			prepareInteractivity(cmd)
			cli.configureRenderer()

			// Namespace commands (e.g. `auth0 actions`) never call the API
			// themselves; they only print help or reject an unknown
			// subcommand, so they must not force authentication.
			if cmd.HasSubCommands() || !commandRequiresAuthentication(cmd.CommandPath()) {
				return nil
			}

			if err := cli.setupWithAuthentication(cmd.Context()); err != nil {
				return err
			}

			return nil
		},
	}

	return rootCmd
}

// enforceUnknownSubcommand makes namespace (parent) commands reject an unknown
// subcommand with a usage error (exit code 2) instead of silently printing help
// and exiting 0. Cobra treats a non-runnable parent as a help request before it
// ever validates positional args, so a plain `Args`/`cobra.NoArgs` on the parent
// never fires. To close that gap we make each namespace runnable — its RunE just
// prints help, preserving the bare `auth0 <group>` behavior — and give it an Args
// validator that rejects any leftover token as an unknown command. The Args check
// runs before PersistentPreRunE, so a typo like `auth0 actions lst` fails fast
// without attempting authentication.
func enforceUnknownSubcommand(cmd *cobra.Command) {
	for _, sub := range cmd.Commands() {
		enforceUnknownSubcommand(sub)
	}

	// Leaf commands and namespaces that already define their own run behavior
	// are left untouched.
	if !cmd.HasSubCommands() || cmd.Runnable() {
		return
	}

	// A namespace defines no flags of its own, so an unknown flag on it almost
	// always accompanies a mistyped subcommand such as `auth0 users lst --json`.
	// Tolerating unknown flags here lets parsing reach the Args validator below,
	// which reports the far more useful "unknown command" instead of "unknown
	// flag". Known persistent flags such as --help and --debug still parse normally.
	cmd.FParseErrWhitelist.UnknownFlags = true

	cmd.Args = func(c *cobra.Command, args []string) error {
		if len(args) == 0 {
			return nil
		}
		return usageError{fmt.Errorf("unknown command %q for %q", args[0], c.CommandPath())}
	}
	cmd.RunE = func(c *cobra.Command, _ []string) error {
		return c.Help()
	}
}

func commandRequiresAuthentication(invokedCommandName string) bool {
	commandsWithNoAuthRequired := []string{
		"auth0 " + cobra.ShellCompRequestCmd,
		"auth0 commands",
		"auth0 completion",
		"auth0 help",
		"auth0 login",
		"auth0 logout",
		"auth0 tenants use",
		"auth0 tenants list",
		"auth0 agent skills install",
	}

	for _, cmd := range commandsWithNoAuthRequired {
		if cmd == invokedCommandName {
			return false
		}
	}

	return true
}

// agentClientName caches detectAgent so mode resolution and telemetry share one lookup.
func (c *cli) agentClientName() string {
	if c.detectedAgent == "" {
		c.detectedAgent = detectAgent(iostream.IsInputTerminal() && iostream.IsOutputTerminal())
	}
	return c.detectedAgent
}

// applyAgentModeDefaults, when agent mode is on, defaults to JSON output with prompts and colors off unless those flags were explicitly set.
func applyAgentModeDefaults(cli *cli, cmd *cobra.Command) {
	if !cli.agentMode {
		return
	}

	if !anyFlagChanged(cmd, "json", "json-compact", "csv") {
		cli.json = true
	}
	if !flagChanged(cmd, "no-input") {
		cli.noInput = true
	}
	if !flagChanged(cmd, "no-color") {
		cli.noColor = true
	}
}

// agentModeEnvVar explicitly enables (true) or disables (false) agent mode, overriding auto-detection.
const agentModeEnvVar = "AUTH0_AGENT_MODE"

// resolveAgentMode reports agent mode: an explicit --agent-mode flag wins, then AUTH0_AGENT_MODE (true/false), then a detected agent client.
func resolveAgentMode(agentClient string, args []string) bool {
	for _, arg := range args {
		if arg == "--agent-mode" {
			return true
		}
		if value, found := strings.CutPrefix(arg, "--agent-mode="); found {
			if enabled, err := strconv.ParseBool(value); err == nil {
				return enabled
			}
		}
	}

	if raw := strings.TrimSpace(os.Getenv(agentModeEnvVar)); raw != "" {
		if enabled, err := strconv.ParseBool(raw); err == nil {
			return enabled
		}
	}

	switch agentClient {
	case "human", "unknown":
		return false
	default:
		return true
	}
}

func flagChanged(cmd *cobra.Command, name string) bool {
	f := cmd.Flags().Lookup(name)
	return f != nil && f.Changed
}

func anyFlagChanged(cmd *cobra.Command, names ...string) bool {
	for _, name := range names {
		if flagChanged(cmd, name) {
			return true
		}
	}
	return false
}

func addPersistentFlags(rootCmd *cobra.Command, cli *cli) {
	rootCmd.PersistentFlags().StringVar(&cli.tenant,
		"tenant", cli.Config.DefaultTenant, "Specific tenant to use.")

	rootCmd.PersistentFlags().BoolVar(&cli.debug,
		"debug", false, "Enable debug mode.")

	rootCmd.PersistentFlags().BoolVar(&cli.noInput,
		"no-input", false, "Disable interactivity.")

	rootCmd.PersistentFlags().BoolVar(&cli.noColor,
		"no-color", false, "Disable colors.")

	rootCmd.PersistentFlags().BoolVar(&cli.agentMode,
		"agent-mode", false,
		"Output JSON, disable prompts and colors. Auto-enabled for AI agents; set AUTH0_AGENT_MODE=false to disable.")
}

func addSubCommands(rootCmd *cobra.Command, cli *cli) {
	// The order of the commands here matters.
	// Add new commands in a place that reflect its
	// relevance or relation with other commands.
	rootCmd.AddCommand(loginCmd(cli))
	rootCmd.AddCommand(logoutCmd(cli))
	rootCmd.AddCommand(tenantsCmd(cli))
	rootCmd.AddCommand(appsCmd(cli))
	rootCmd.AddCommand(aculCmd(cli))
	rootCmd.AddCommand(usersCmd(cli))
	rootCmd.AddCommand(rulesCmd(cli))
	rootCmd.AddCommand(actionsCmd(cli))
	rootCmd.AddCommand(apisCmd(cli))
	rootCmd.AddCommand(clientGrantsCmd(cli))
	rootCmd.AddCommand(rolesCmd(cli))
	rootCmd.AddCommand(organizationsCmd(cli))
	rootCmd.AddCommand(universalLoginCmd(cli))
	rootCmd.AddCommand(phoneCmd(cli))
	rootCmd.AddCommand(emailCmd(cli))
	rootCmd.AddCommand(customDomainsCmd(cli))
	rootCmd.AddCommand(quickstartsCmd(cli))
	rootCmd.AddCommand(attackProtectionCmd(cli))
	rootCmd.AddCommand(testCmd(cli))
	rootCmd.AddCommand(logsCmd(cli))
	rootCmd.AddCommand(apiCmd(cli))
	rootCmd.AddCommand(terraformCmd(cli))
	rootCmd.AddCommand(eventStreamsCmd(cli))
	rootCmd.AddCommand(formsCmd(cli))
	rootCmd.AddCommand(flowsCmd(cli))
	rootCmd.AddCommand(networkACLCmd(cli))
	rootCmd.AddCommand(tenantSettingsCmd(cli))
	rootCmd.AddCommand(tokenExchangeCmd(cli))
	rootCmd.AddCommand(sessionsCmd(cli))
	rootCmd.AddCommand(refreshTokensCmd(cli))
	rootCmd.AddCommand(guardianCmd(cli))

	rootCmd.AddCommand(commandsCmd(cli))
	rootCmd.AddCommand(agentCmd(cli))

	// Keep completion at the bottom.
	rootCmd.AddCommand(completionCmd(cli))
}

func contextWithCancel() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	go func() {
		<-ch
		defer cancel()
		os.Exit(exitInterrupted)
	}()

	return ctx
}

func overrideHelpAndVersionFlagText(cmd *cobra.Command) {
	cmd.Flags().BoolP("version", "v", false, "Version for auth0.")

	setHelpFlagTextFunc := func(c *cobra.Command) {
		c.Flags().BoolP("help", "h", false, fmt.Sprintf("Help for %s.", c.Name()))
	}

	setHelpFlagTextFunc(cmd)
	for _, c := range cmd.Commands() {
		setHelpFlagTextFunc(c)
		for _, c := range c.Commands() {
			setHelpFlagTextFunc(c)
		}
	}
}

func renderErrorMessage(display *display.Renderer, errorMessage string) {
	display.Heading(ansi.Red("error"))

	rawErrorMessage := []rune(errorMessage)
	if len(rawErrorMessage) == 0 {
		display.Errorf("An unknown error occurred.")
		display.Newline()
		return
	}

	humanReadableErrorMessage := string(
		append(
			[]rune{unicode.ToUpper(rawErrorMessage[0])},
			rawErrorMessage[1:]...,
		),
	) + "."

	display.Errorf(humanReadableErrorMessage)
	display.Newline()
}

func trackCommandOutcome(cli *cli, executionErr error) {
	if cli.tracker == nil {
		return
	}

	installID := resolveInstallIDForTracking(cli)
	if installID == "" {
		return
	}

	if cli.executedCommandPath == "" {
		cli.executedCommandPath = "auth0"
	}

	properties := commandTrackingProperties(cli)

	if executionErr != nil {
		failureProperties := mergeProperties(properties, classifyCommandFailure(executionErr))
		cli.tracker.TrackCommandRun(cli.executedCommandPath, installID, failureProperties)
		return
	}

	successProperties := mergeProperties(properties, map[string]string{
		"success":     "true",
		"error_class": "none",
	})
	cli.tracker.TrackCommandRun(cli.executedCommandPath, installID, successProperties)
}

func commandTrackingProperties(cli *cli) map[string]string {
	interactive := iostream.IsInputTerminal() && iostream.IsOutputTerminal()

	return map[string]string{
		"interactive":   boolString(interactive),
		"ci":            boolString(isCIEnvironment(os.Getenv)),
		"no_input":      boolString(cli.noInput),
		"output_format": outputFormatForTracking(cli.renderer),
		"forced":        boolString(cli.force),
		"agent_client":  cli.agentClientName(),
		"is_api":        boolString(isAPICommand(cli.executedCommandPath)),
		"tenant":        cli.tenant,
	}
}

// isAPICommand reports whether the executed command is the raw
// `auth0 api` Management API passthrough command.
func isAPICommand(commandPath string) bool {
	return commandPath == "auth0 api"
}

func outputFormatForTracking(renderer *display.Renderer) string {
	if renderer == nil || renderer.Format == "" {
		return "table"
	}

	return string(renderer.Format)
}

func isCIEnvironment(getEnv func(string) string) bool {
	for _, envVar := range ciEnvironmentVariables {
		rawValue := strings.TrimSpace(getEnv(envVar))
		if rawValue == "" {
			continue
		}

		lowerValue := strings.ToLower(rawValue)
		if lowerValue != "false" && lowerValue != "0" {
			return true
		}
	}

	return false
}

func boolString(value bool) string {
	if value {
		return "true"
	}

	return "false"
}

func mergeProperties(base map[string]string, override map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(override))

	for k, v := range base {
		merged[k] = v
	}

	for k, v := range override {
		merged[k] = v
	}

	return merged
}

func resolveInstallIDForTracking(cli *cli) string {
	if cli.Config.InstallID != "" {
		return cli.Config.InstallID
	}

	if err := cli.Config.Initialize(); err != nil {
		if errors.Is(err, config.ErrConfigFileMissing) {
			return ""
		}
		return ""
	}

	return cli.Config.InstallID
}

func classifyCommandFailure(err error) map[string]string {
	return map[string]string{
		"success":     "false",
		"error_class": errorClass(err),
	}
}
