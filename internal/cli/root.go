package cli

import (
	"context"
	"os"

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

		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// If the user is trying to login, no need to go
			// through setup.
			if cmd.Use == "login" {
				return nil
			}

			// Initialize everything once. Later callers can then
			// freely assume that config is fully primed and ready
			// to go.
			return cli.setup()
		},
	}

	rootCmd.SetUsageTemplate(namespaceUsageTemplate())
	rootCmd.PersistentFlags().StringVar(&cli.tenant,
		"tenant", cli.config.DefaultTenant, "Specific tenant to use.")

	rootCmd.PersistentFlags().BoolVar(&cli.verbose,
		"verbose", false, "Enable verbose mode.")

	rootCmd.PersistentFlags().StringVar(&cli.format,
		"format", "", "Command output format. Options: json.")

	rootCmd.PersistentFlags().BoolVar(&cli.force,
		"force", false, "Skip confirmation.")

	rootCmd.PersistentFlags().BoolVar(&cli.noInput,
		"no-input", false, "Disable interactivity.")

	// order of the comamnds here matters
	// so add new commands in a place that reflect its relevance or relation with other commands:
	rootCmd.AddCommand(loginCmd(cli))
	rootCmd.AddCommand(quickstartCmd(cli))
	rootCmd.AddCommand(clientsCmd(cli))
	rootCmd.AddCommand(apisCmd(cli))
	rootCmd.AddCommand(logsCmd(cli))
	rootCmd.AddCommand(rulesCmd(cli))
	rootCmd.AddCommand(actionsCmd(cli))
	rootCmd.AddCommand(connectionsCmd(cli))
	rootCmd.AddCommand(rolesCmd(cli))
	rootCmd.AddCommand(customDomainsCmd(cli))
	rootCmd.AddCommand(usersCmd(cli))
	rootCmd.AddCommand(testCmd(cli))

	// keep completion at the bottom:
	rootCmd.AddCommand(completionCmd(cli))

	// TODO(cyx): backport this later on using latest auth0/v5.
	// rootCmd.AddCommand(actionsCmd(cli))
	// rootCmd.AddCommand(triggersCmd(cli))

	if err := rootCmd.ExecuteContext(context.TODO()); err != nil {
		cli.renderer.Errorf(err.Error())
		os.Exit(1)
	}
}
