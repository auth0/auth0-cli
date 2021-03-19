package cli

import (
	"context"
	"os"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/buildinfo"
	"github.com/auth0/auth0-cli/internal/display"
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
	}

	rootCmd := &cobra.Command{
		Use:           "auth0",
		SilenceUsage:  true,
		SilenceErrors: true,
		Short:         "Supercharge your development workflow.",
		Long:          "Supercharge your development workflow.\n" + getLogin(cli),
		Version:       buildinfo.GetVersionWithCommit(),

		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
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

	// order of the comamnds here matters
	// so add new commands in a place that reflect its relevance or relation with other commands:
	rootCmd.AddCommand(loginCmd(cli))
	rootCmd.AddCommand(tenantsCmd(cli))
	rootCmd.AddCommand(appsCmd(cli))
	rootCmd.AddCommand(rulesCmd(cli))
	rootCmd.AddCommand(quickstartsCmd(cli))
	rootCmd.AddCommand(apisCmd(cli))
	rootCmd.AddCommand(testCmd(cli))
	rootCmd.AddCommand(logsCmd(cli))
	rootCmd.AddCommand(logoutCmd(cli))

	// keep completion at the bottom:
	rootCmd.AddCommand(completionCmd(cli))

	// TODO(cyx): backport this later on using latest auth0/v5.
	// rootCmd.AddCommand(actionsCmd(cli))
	// rootCmd.AddCommand(triggersCmd(cli))

	if err := rootCmd.ExecuteContext(context.TODO()); err != nil {
		header := []string{"error\n"}
		if cli.tenant != "" {
			header = append([]string{ansi.Bold(cli.tenant)}, header...)
		}
		cli.renderer.Heading(header...)
		cli.renderer.Errorf(err.Error())
		os.Exit(1)
	}
}
