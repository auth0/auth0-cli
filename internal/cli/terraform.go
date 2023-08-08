package cli

import (
	"context"
	"os"
	"path"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/auth0"
)

var tfFlags = terraformFlags{
	OutputDIR: Flag{
		Name:      "Output Dir",
		LongForm:  "output-dir",
		ShortForm: "o",
		Help: "Output directory for the generated Terraform config files. If not provided, the files will be " +
			"saved in the current working directory.",
	},
}

type (
	terraformFlags struct {
		OutputDIR Flag
	}

	terraformInputs struct {
		OutputDIR string
	}
)

func (i *terraformInputs) parseResourceFetchers(api *auth0.API) []resourceDataFetcher {
	// Hard coding this for now until we add support for the `--resources` flag.
	return []resourceDataFetcher{
		&clientResourceFetcher{
			api: api,
		},
	}
}

func terraformCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "terraform",
		Aliases: []string{"tf"},
		Short:   "Manage terraform configuration for your Auth0 Tenant",
		Long: "This command facilitates the integration of Auth0 with [Terraform](https://www.terraform.io/), an " +
			"Infrastructure as Code tool.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(generateTerraformCmd(cli))

	return cmd
}

func generateTerraformCmd(cli *cli) *cobra.Command {
	var inputs terraformInputs

	cmd := &cobra.Command{
		Use:     "generate",
		Aliases: []string{"gen", "export"}, // Reconsider aliases and command name before releasing.
		Short:   "Generate terraform configuration for your Auth0 Tenant",
		Long: "This command is designed to streamline the process of generating Terraform configuration files for " +
			"your Auth0 resources, serving as a bridge between the two.\n\nIt automatically scans your Auth0 Tenant " +
			"and compiles a set of Terraform configuration files based on the existing resources and configurations." +
			"\n\nThe generated Terraform files are written in HashiCorp Configuration Language (HCL).",
		RunE: generateTerraformCmdRun(cli, &inputs),
	}

	tfFlags.OutputDIR.RegisterString(cmd, &inputs.OutputDIR, "./")

	return cmd
}

func generateTerraformCmdRun(cli *cli, inputs *terraformInputs) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		data, err := fetchImportData(cmd.Context(), inputs.parseResourceFetchers(cli.api)...)
		if err != nil {
			return err
		}

		// Just temporarily. Remove this once import file generation is in place.
		cli.renderer.JSONResult(data)
		cli.renderer.Newline()

		if err := generateTerraformConfigFiles(inputs); err != nil {
			return err
		}

		cli.renderer.Infof("Terraform config files generated successfully.")
		cli.renderer.Infof(
			"Follow this " +
				"[quickstart](https://registry.terraform.io/providers/auth0/auth0/latest/docs/guides/quickstart) " +
				"to go through setting up an Auth0 application for the provider to authenticate against and manage " +
				"resources.",
		)

		return nil
	}
}

func fetchImportData(ctx context.Context, fetchers ...resourceDataFetcher) (importDataList, error) {
	var importData importDataList

	for _, fetcher := range fetchers {
		data, err := fetcher.FetchData(ctx)
		if err != nil {
			return nil, err
		}

		importData = append(importData, data...)
	}

	return importData, nil
}

func generateTerraformConfigFiles(inputs *terraformInputs) error {
	const readWritePermission = 0755
	if err := os.MkdirAll(inputs.OutputDIR, readWritePermission); err != nil {
		if !os.IsExist(err) {
			return err
		}
	}

	mainTerraformConfigFile, err := os.Create(path.Join(inputs.OutputDIR, "main.tf"))
	if err != nil {
		return err
	}
	defer mainTerraformConfigFile.Close()

	mainTerraformConfigFileContent := `terraform {
  required_version = "~> 1.5.0"
  required_providers {
    auth0 = {
      source  = "auth0/auth0"
      version = "1.0.0-beta.1"
    }
  }
}

provider "auth0" {
  debug         = true
}
`

	_, err = mainTerraformConfigFile.WriteString(mainTerraformConfigFileContent)
	return err
}
