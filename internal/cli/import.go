package cli

import (
	"fmt"

	"github.com/auth0/auth0-cli/internal/cli/importcmd"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
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

			// The command logic goes here

			config, err := importcmd.GetConfig(inputs.Config)
			if err != nil {
				return err
			}
			// config, error := getConfig(inputs.Config)
			// yaml, error := getYaml(inputs.Input, config)
			// appChanges, error := processApps(cli, yaml, config)
			// {additions, changes, deletions}
			// apiChanges, error := processApis(cli, yaml, config)
			// roleChanges, error := processRoles(cli, yaml, config)
			// additions, changes, deletions := calculateChanges(appChanges, apiChanges, roleChanges)
			// display.Import(additions, changes, deletions)
			// return nil

			// YAML file getYAML()
			// Take: YAML file path, config value
			// Do: parse the YAML into a struct instance and perform the replacements, according to the config
			// Return: YAML with replacements

			yamlData, err := importcmd.ParseYAML(inputs.Input, config)
			if err != nil {
				return err
			}

			j, _ := yaml.Marshal(&yamlData)
			fmt.Printf("yamlData is: \n%+v", string(j))

			// Config file getConfig()
			// Take: config file path
			// Do: parse the JSON into a struct instance
			// Return: config value

			fmt.Printf("Config file: %s\n", inputs.Config)
			fmt.Printf("Input file: %s\n", inputs.Input)

			return nil
		},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	importConfig.RegisterString(cmd, &inputs.Config, "")
	importInput.RegisterString(cmd, &inputs.Input, "")

	return cmd
}
