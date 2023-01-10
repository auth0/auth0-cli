package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/analytics"
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/buildinfo"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/instrumentation"
)

const rootShort = "Build, manage and test your Auth0 integrations from the command line."

const panicMessage = `
!!     Uh oh. Something went wrong.
!!     If this problem keeps happening feel free to report an issue at
!!
!!     https://github.com/auth0/auth0-cli/issues/new/choose
`

// Execute is the primary entrypoint of the CLI app.
func Execute() {
	cli := &cli{
		renderer: display.NewRenderer(),
		tracker:  analytics.NewTracker(),
	}

	rootCmd := buildRootCmd(cli)
	rootCmd.SetUsageTemplate(namespaceUsageTemplate())

	addPersistentFlags(rootCmd, cli)
	addSubCommands(rootCmd, cli)

	overrideHelpAndVersionFlagText(rootCmd)

	defer func() {
		if v := recover(); v != nil {
			err := fmt.Errorf("panic: %v", v)

			// If we're in development mode, we should throw the
			// panic for so we have less surprises. For
			// non-developers, we'll swallow the panics.
			if instrumentation.ReportException(err) {
				fmt.Print(panicMessage)
			} else {
				panic(v)
			}
		}
	}()

	// platform specific terminal initialization:
	// this should run for all commands,
	// for most of the architectures there's no requirements:
	ansi.InitConsole()

	cancelCtx := contextWithCancel()
	if err := rootCmd.ExecuteContext(cancelCtx); err != nil {
		cli.renderer.Heading("error")
		cli.renderer.Errorf(err.Error())

		instrumentation.ReportException(err)
		os.Exit(1) // nolint:gocritic
	}

	timeoutCtx, cancel := context.WithTimeout(cancelCtx, 3*time.Second)
	// defers are executed in LIFO order
	defer cancel()
	defer cli.tracker.Wait(timeoutCtx) // No event should be tracked after this has run, or it will panic e.g. in earlier deferred functions
}

func buildRootCmd(cli *cli) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "auth0",
		SilenceUsage:  true,
		SilenceErrors: true,
		Short:         rootShort,
		Long:          rootShort + "\n" + getLogin(cli),
		Version:       buildinfo.GetVersionWithCommit(),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			ansi.Initialize(cli.noColor)
			prepareInteractivity(cmd)

			// If the user is trying to login, no need to go
			// through setup.
			if cmd.Use == "login" && cmd.Parent().Use == "auth0" {
				return nil
			}

			// We're tracking the login command in its Run method
			// so we'll only add this defer if the command is not login
			defer func() {
				if cli.tracker != nil && cli.isLoggedIn() {
					cli.tracker.TrackCommandRun(cmd, cli.config.InstallID)
				}
			}()

			// If the user is trying to logout, session information
			// isn't important as well.
			if cmd.Use == "logout" && cmd.Parent().Use == "auth0" {
				return nil
			}

			// Selecting tenants shouldn't really trigger a login.
			if cmd.Parent().Use == "tenants" && (cmd.Use == "use" || cmd.Use == "add") {
				return nil
			}

			// Getting the CLI completion script shouldn't trigger a login.
			if cmd.Use == "completion" && cmd.Parent().Use == "auth0" {
				return nil
			}

			// Getting help shouldn't trigger a login.
			if cmd.CalledAs() == "help" && cmd.Parent().Use == "auth0" {
				return nil
			}

			// config init shouldn't trigger a login.
			if cmd.CalledAs() == "init" && cmd.Parent().Use == "config" {
				return nil
			}

			// Initialize everything once. Later callers can then
			// freely assume that config is fully primed and ready
			// to go.
			return cli.setup(cmd.Context())
		},
	}

	return rootCmd
}

func addPersistentFlags(rootCmd *cobra.Command, cli *cli) {
	rootCmd.PersistentFlags().StringVar(&cli.tenant,
		"tenant", cli.config.DefaultTenant, "Specific tenant to use.")

	rootCmd.PersistentFlags().BoolVar(&cli.debug,
		"debug", false, "Enable debug mode.")

	rootCmd.PersistentFlags().BoolVar(&cli.noInput,
		"no-input", false, "Disable interactivity.")

	rootCmd.PersistentFlags().BoolVar(&cli.noColor,
		"no-color", false, "Disable colors.")
}

func addSubCommands(rootCmd *cobra.Command, cli *cli) {
	// The order of the commands here matters.
	// Add new commands in a place that reflect its
	// relevance or relation with other commands:
	rootCmd.AddCommand(loginCmd(cli))
	rootCmd.AddCommand(logoutCmd(cli))
	rootCmd.AddCommand(tenantsCmd(cli))
	rootCmd.AddCommand(appsCmd(cli))
	rootCmd.AddCommand(usersCmd(cli))
	rootCmd.AddCommand(rulesCmd(cli))
	rootCmd.AddCommand(actionsCmd(cli))
	rootCmd.AddCommand(apisCmd(cli))
	rootCmd.AddCommand(rolesCmd(cli))
	rootCmd.AddCommand(organizationsCmd(cli))
	rootCmd.AddCommand(universalLoginCmd(cli))
	rootCmd.AddCommand(emailCmd(cli))
	rootCmd.AddCommand(customDomainsCmd(cli))
	rootCmd.AddCommand(ipsCmd(cli))
	rootCmd.AddCommand(quickstartsCmd(cli))
	rootCmd.AddCommand(attackProtectionCmd(cli))
	rootCmd.AddCommand(testCmd(cli))
	rootCmd.AddCommand(logsCmd(cli))
	rootCmd.AddCommand(apiCmd(cli))

	// keep completion at the bottom:
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
