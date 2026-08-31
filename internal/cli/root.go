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

	"github.com/auth0/go-auth0/management"
	"github.com/auth0/go-auth0/v3/management/core"
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

	addPersistentFlags(rootCmd, cli)
	addSubCommands(rootCmd, cli)

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
		renderErrorMessage(cli.renderer, err.Error())

		instrumentation.ReportException(err)
		os.Exit(1) // nolint:gocritic
	}
}

func buildRootCmd(cli *cli) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "auth0",
		SilenceUsage:  true,
		SilenceErrors: true,
		Short:         rootShort,
		Long:          rootLong + "\n\n" + getLogin(cli),
		Version:       buildinfo.GetVersionWithCommit(),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cli.executedCommandPath = cmd.CommandPath()

			applyAgentModeDefaults(cli, cmd)

			ansi.Initialize(cli.noColor)
			prepareInteractivity(cmd)
			cli.configureRenderer()

			// Emitted after ansi.Initialize so the notice respects the color setting.
			if cli.agentMode {
				cli.renderer.Infof("Agent mode on: JSON output, prompts and colors off. Disable with --agent-mode=false.")
			}

			if !commandRequiresAuthentication(cmd.CommandPath()) {
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

func commandRequiresAuthentication(invokedCommandName string) bool {
	commandsWithNoAuthRequired := []string{
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
	rootCmd.AddCommand(networkACLCmd(cli))
	rootCmd.AddCommand(tenantSettingsCmd(cli))
	rootCmd.AddCommand(tokenExchangeCmd(cli))
	rootCmd.AddCommand(sessionsCmd(cli))
	rootCmd.AddCommand(refreshTokensCmd(cli))

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
		os.Exit(0)
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
	properties := map[string]string{
		"success":     "false",
		"error_class": "unknown",
	}

	if errors.Is(err, config.ErrInvalidToken) || errors.Is(err, config.ErrMalformedToken) {
		properties["error_class"] = "auth"
		return properties
	}

	var missingScopesErr config.ErrTokenMissingRequiredScopes
	if errors.As(err, &missingScopesErr) {
		properties["error_class"] = "auth"
		return properties
	}

	if status, ok := managementHTTPStatus(err); ok {
		properties["error_class"] = errorClassForHTTPStatus(status)
	}

	return properties
}

// managementHTTPStatus extracts the HTTP status from a go-auth0 management API
// error anywhere in the error chain, supporting both the v1 (management.Error)
// and v3 (*core.APIError) SDK error types.
func managementHTTPStatus(err error) (int, bool) {
	var v1 management.Error
	if errors.As(err, &v1) {
		return v1.Status(), true
	}

	var v3 *core.APIError
	if errors.As(err, &v3) {
		return v3.StatusCode, true
	}

	return 0, false
}

func errorClassForHTTPStatus(status int) string {
	switch {
	case status == 401 || status == 403:
		return "auth"
	case status == 400 || status == 422:
		return "validation"
	case status == 404:
		return "not_found"
	case status == 429:
		return "rate_limit"
	case status >= 500:
		return "api"
	default:
		return "unknown"
	}
}
