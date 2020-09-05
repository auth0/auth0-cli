package cli

import (
	"fmt"
	"os"

	"github.com/auth0/auth0-cli/internal/config"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// Execute is the primary entrypoint of the CLI app.
func Execute() {
	// fs is a mock friendly os.File interface.
	fs := afero.NewOsFs()

	// cfg contains tenant related information, e.g. `travel0-dev`,
	// `travel0-prod`. some of its information can be sourced via:
	// 1. env var (e.g. AUTH0_API_KEY)
	// 2. global flag (e.g. --api-key)
	// 3. JSON file (e.g. api_key = "..." in ~/.config/auth0/config.json)
	cfg := &config.Config{}

	rootCmd := &cobra.Command{
		Use:           "auth0",
		SilenceUsage:  true,
		SilenceErrors: true,
		Short:         "Command-line tool to interact with Auth0.",
		Long:          "Command-line tool to interact with Auth0.\n" + getLogin(&fs, cfg),

		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Initialize everything once. Later callers can then
			// freely assume that config is fully primed and ready
			// to go.
			return cfg.Init()
		},
	}

	rootCmd.SetUsageTemplate(namespaceUsageTemplate())
	rootCmd.PersistentFlags().StringVar(&cfg.Tenant,
		"tenant", "", "Specific tenant to use")

	rootCmd.PersistentFlags().BoolVar(&cfg.Verbose,
		"verbose", false, "Enable verbose mode.")

	rootCmd.AddCommand(actionsCmd(cfg))
	rootCmd.AddCommand(triggersCmd(cfg))

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
