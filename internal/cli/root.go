package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/auth0/auth0-cli/internal/analytics"
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth"
	"github.com/auth0/auth0-cli/internal/buildinfo"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/instrumentation"
	"github.com/joeshaw/envdecode"
	"github.com/spf13/cobra"
)

const rootShort = "Supercharge your development workflow."

// authCfg defines the configurable auth context the cli will run in.
var authCfg struct {
	Audience           string `env:"AUTH0_AUDIENCE,default=https://*.auth0.com/api/v2/"`
	ClientID           string `env:"AUTH0_CLIENT_ID,default=2iZo3Uczt5LFHacKdM0zzgUO2eG2uDjT"`
	DeviceCodeEndpoint string `env:"AUTH0_DEVICE_CODE_ENDPOINT,default=https://auth0.auth0.com/oauth/device/code"`
	OauthTokenEndpoint string `env:"AUTH0_OAUTH_TOKEN_ENDPOINT,default=https://auth0.auth0.com/oauth/token"`
}

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
	}

	rootCmd := buildRootCmd(cli)

	rootCmd.SetUsageTemplate(namespaceUsageTemplate())
	addPersistentFlags(rootCmd, cli)
	addSubcommands(rootCmd, cli)

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

	cancelCtx := contextWithCancel()
	if err := rootCmd.ExecuteContext(cancelCtx); err != nil {
		cli.renderer.Heading("error")
		cli.renderer.Errorf(err.Error())

		instrumentation.ReportException(err)
		os.Exit(1)
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
			if err := envdecode.StrictDecode(&authCfg); err != nil {
				return fmt.Errorf("could not decode env: %w", err)
			}

			cli.authenticator = &auth.Authenticator{
				Audience:           authCfg.Audience,
				ClientID:           authCfg.ClientID,
				DeviceCodeEndpoint: authCfg.DeviceCodeEndpoint,
				OauthTokenEndpoint: authCfg.OauthTokenEndpoint,
			}
			ansi.DisableColors = cli.noColor
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

	rootCmd.PersistentFlags().StringVar(&cli.format,
		"format", "", "Command output format. Options: json.")

	rootCmd.PersistentFlags().BoolVar(&cli.force,
		"force", false, "Skip confirmation.")

	rootCmd.PersistentFlags().BoolVar(&cli.noInput,
		"no-input", false, "Disable interactivity.")

	rootCmd.PersistentFlags().BoolVar(&cli.noColor,
		"no-color", false, "Disable colors.")

}

func addSubcommands(rootCmd *cobra.Command, cli *cli) {
	// order of the comamnds here matters
	// so add new commands in a place that reflect its relevance or relation with other commands:
	rootCmd.AddCommand(loginCmd(cli))
	rootCmd.AddCommand(logoutCmd(cli))
	rootCmd.AddCommand(configCmd(cli))
	rootCmd.AddCommand(tenantsCmd(cli))
	rootCmd.AddCommand(appsCmd(cli))
	rootCmd.AddCommand(usersCmd(cli))
	rootCmd.AddCommand(rulesCmd(cli))
	rootCmd.AddCommand(actionsCmd(cli))
	rootCmd.AddCommand(apisCmd(cli))
	rootCmd.AddCommand(rolesCmd(cli))
	rootCmd.AddCommand(organizationsCmd(cli))
	rootCmd.AddCommand(brandingCmd(cli))
	rootCmd.AddCommand(ipsCmd(cli))
	rootCmd.AddCommand(quickstartsCmd(cli))
	rootCmd.AddCommand(testCmd(cli))
	rootCmd.AddCommand(logsCmd(cli))

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

const panicMessage = `
!!     Uh oh. Something went wrong.
!!     If this problem keeps happening feel free to report an issue at
!!
!!     https://github.com/auth0/auth0-cli/issues/new/choose
`
