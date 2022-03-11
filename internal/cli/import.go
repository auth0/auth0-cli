package cli

import (
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/spf13/cobra"
)

var (
	importConfig = Flag{
		Name:       "Config file path",
		LongForm:   "config",
		ShortForm:  "c",
		Help:       "Path to the JSON config file.",
		IsRequired: true,
	}
	importInput = Flag{
		Name:       "Input file path",
		LongForm:   "input",
		ShortForm:  "i",
		Help:       "Path to the input YAML file.",
		IsRequired: true,
	}
)

func importCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Config string
		Input  string
	}

	cmd := &cobra.Command{
		Use:     "import",
		Short:   "Import tenant resources and settings from a YAML file.",
		Long:    "Import tenant resources and settings from a YAML file. YAML files produced by the Auth0 Deploy CLI are supported.",
		Example: "auth0 import --config config.json --input tenant.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Ask for the config file path if the flag was not set
			if err := importConfig.Ask(cmd, &inputs.Config, nil); err != nil {
				return err
			}

			// Ask for the input file path if the flag was not set
			if err := importInput.Ask(cmd, &inputs.Input, nil); err != nil {
				return err
			}

			config, err := GetConfig(inputs.Config)
			if err != nil {
				return err
			}

			yamlData, err := ParseYAML(inputs.Input, config)
			if err != nil {
				return err
			}

			appsResult, err := ImportApps(cmd.Context(), cli, config, yamlData)
			if err != nil {
				return err
			}

			apisResult, err := ImportAPIs(cmd.Context(), cli, config, yamlData)
			if err != nil {
				return err
			}

			rolesResult, err := ImportRoles(cmd.Context(), cli, config, yamlData)
			if err != nil {
				return err
			}

			cli.renderer.Import([]*auth0.ImportChanges{appsResult, apisResult, rolesResult})

			return nil
		},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	importConfig.RegisterString(cmd, &inputs.Config, "")
	importInput.RegisterString(cmd, &inputs.Input, "")

	return cmd
}
