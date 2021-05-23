package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/auth0/auth0-cli/internal/analytics"
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/buildinfo"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/instrumentation"
	"github.com/spf13/cobra"
)

// Execute is the primary entrypoint of the CLI app.
func Execute() {
	// cfg contains tenant related information, e.g. `travel0-dev`,
	// `travel0-prod`. some of its information can be sourced via:
	// 1. env var (e.g. AUTH0_API_KEY)
	// 2. global flag (e.g. --api-key)
	// 3. JSON file (e.g. api_key = "..." in ~/.config/auth0/config.json)
	cli := &cli{
		renderer: display.NewRenderer(),
		tracker:  analytics.NewTracker(),
		context:  cliContext(),
	}

	rootCmd := &cobra.Command{
		Use:           "auth0",
		SilenceUsage:  true,
		SilenceErrors: true,
		Short:         "Supercharge your development workflow.",
		Long:          "Supercharge your development workflow.\n" + getLogin(cli),
		Version:       buildinfo.GetVersionWithCommit(),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			ansi.DisableColors = cli.noColor
			prepareInteractivity(cmd)

			// If the user is trying to login, no need to go
			// through setup.
			if cmd.Use == "login" && cmd.Parent().Use == "auth0" {
				return nil
			}

			// If the user is trying to logout, session information
			// isn't important as well.
			if cmd.Use == "logout" && cmd.Parent().Use == "auth0" {
				return nil
			}

			// Selecting tenants shouldn't really trigger a login.
			if cmd.Use == "use" && cmd.Parent().Use == "tenants" {
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

			defer cli.tracker.TrackCommandRun(cmd, cli.config.InstallID)

			// Initialize everything once. Later callers can then
			// freely assume that config is fully primed and ready
			// to go.
			return cli.setup(cmd.Context())
		},
	}

	rootCmd.SetUsageTemplate(namespaceUsageTemplate())
	rootCmd.PersistentFlags().StringVar(&cli.tenant,
		"tenant", cli.config.DefaultTenant, "Specific tenant to use.")

	rootCmd.PersistentFlags().BoolVar(&cli.debug,
		"debug", false, "Enable debug mode.")

	rootCmd.PersistentFlags().StringVar(&cli.format,
		"format", "", "Command output format. Options: json.")

	rootCmd.PersistentFlags().BoolVar(&cli.force,
		"force", false, "Skip confirmation.")

	rootCmd.PersistentFlags().BoolVar(&cli.noInput,
		"no-input", false, "Disable interactivity.")

	rootCmd.PersistentFlags().BoolVar(&cli.noColor,
		"no-color", false, "Disable colors.")
	// order of the comamnds here matters
	// so add new commands in a place that reflect its relevance or relation with other commands:
	rootCmd.AddCommand(loginCmd(cli))
	rootCmd.AddCommand(tenantsCmd(cli))
	rootCmd.AddCommand(usersCmd(cli))
	rootCmd.AddCommand(appsCmd(cli))
	rootCmd.AddCommand(rulesCmd(cli))
	rootCmd.AddCommand(apisCmd(cli))
	rootCmd.AddCommand(quickstartsCmd(cli))
	rootCmd.AddCommand(testCmd(cli))
	rootCmd.AddCommand(logsCmd(cli))
	rootCmd.AddCommand(logoutCmd(cli))
	rootCmd.AddCommand(brandingCmd(cli))
	rootCmd.AddCommand(rolesCmd(cli))
	rootCmd.AddCommand(ipsCmd(cli))

	// keep completion at the bottom:
	rootCmd.AddCommand(completionCmd(cli))

	// TODO(cyx): backport this later on using latest auth0/v5.
	// rootCmd.AddCommand(actionsCmd(cli))
	// rootCmd.AddCommand(triggersCmd(cli))

	defer func() {
		if v := recover(); v != nil {
			err := fmt.Errorf("panic: %v", v)

			// If we're in development mode, we should throw the
			// panic for so we have less surprises. For
			// non-developers, we'll swallow the panics.
			if instrumentation.ReportException(err) {
				fmt.Println(panicMessage)
			} else {
				panic(v)
			}
		}
	}()

	// platform specific terminal initialization:
	// this should run for all commands,
	// for most of the architectures there's no requirements:
	ansi.InitConsole()

	if err := rootCmd.ExecuteContext(cli.context); err != nil {
		cli.renderer.Heading("error")
		cli.renderer.Errorf(err.Error())

		instrumentation.ReportException(err)
		os.Exit(1)
	}

	ctx, _ := context.WithTimeout(cli.context, 3 * time.Second)
	defer cli.tracker.Wait(ctx)
}

func cliContext() context.Context {
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

const panicMessage = `
!!     Uh oh. Something went wrong.
!!     If this problem keeps happening feel free to report an issue at
!!
!!     https://github.com/auth0/auth0-cli/issues/new/choose
`
