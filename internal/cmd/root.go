package cmd

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
	// `travel0-prod`. some of its information can be source via:
	// 1. env var (e.g. AUTH0_API_KEY)
	// 2. global flag (e.g. --api-key)
	// 3. toml file (e.g. api_key = "..." in ~/.config/auth0/config.toml)
	cfg := &config.Config{}

	rootCmd := &cobra.Command{
		Use:           "auth0",
		SilenceUsage:  true,
		SilenceErrors: true,
		Short:         "Command-line tool to interact with Auth0.",
		Long:          "Command-line tool to interact with Auth0.\n" + getLogin(&fs, cfg),

		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// We want cfg.Init to run for any command since it provides the
			// underlying tenant information we'll need to fulfill the user's
			// request.
			cfg.Init()
		},
	}

	rootCmd.SetUsageTemplate(namespaceUsageTemplate())
	rootCmd.PersistentFlags().StringVar(&cfg.Profile.APIKey,
		"api-key", "", "Your API key to use for the command")
	rootCmd.PersistentFlags().StringVar(&cfg.Color,
		"color", "", "turn on/off color output (on, off, auto)")
	rootCmd.PersistentFlags().StringVar(&cfg.ProfilesFile,
		"config", "", "config file (default is $HOME/.config/auth0/config.toml)")
	rootCmd.PersistentFlags().StringVar(&cfg.Profile.DeviceName,
		"device-name", "", "device name")
	rootCmd.PersistentFlags().StringVar(&cfg.LogLevel,
		"log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVarP(&cfg.Profile.ProfileName,
		"tenant", "", "default", "the tenant info to read from for config")
	rootCmd.Flags().BoolP("version", "v", false, "Get the version of the Auth0 CLI")

	rootCmd.AddCommand(actionsCmd(cfg))
	rootCmd.AddCommand(triggersCmd(cfg))

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
