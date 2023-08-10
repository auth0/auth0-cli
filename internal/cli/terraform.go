package cli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"text/template"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
	"github.com/hashicorp/terraform-exec/tfexec"
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

		if err := generateTerraformImportConfig(inputs.OutputDIR, data); err != nil {
			return err
		}

		if terraformProviderCredentialsAreAvailable() {
			if err := generateTerraformResourceConfig(cmd.Context(), inputs.OutputDIR); err == nil {
				cli.renderer.Infof("Terraform resource config files generated successfully in: %q", inputs.OutputDIR)
				cli.renderer.Infof(
					"Review the config and generate the terraform state by running: \n\n	cd %s && ./terraform apply",
					inputs.OutputDIR,
				)
				cli.renderer.Newline()
				cli.renderer.Infof(
					"After running the above command and generating the state, " +
						"the ./terraform binary and auth0_import.tf files can be safely removed.\n",
				)

				return nil
			}
		}

		cli.renderer.Infof("Terraform resource import files generated successfully in: %q", inputs.OutputDIR)
		cli.renderer.Infof(
			"Follow this " +
				"[quickstart](https://registry.terraform.io/providers/auth0/auth0/latest/docs/guides/quickstart) " +
				"to go through setting up an Auth0 application for the provider to authenticate against and manage " +
				"resources.",
		)
		cli.renderer.Infof(
			"After setting up the provider credentials, run: \n\n"+
				"	cd %s && terraform init && terraform plan -generate-config-out=generated.tf && terraform apply",
			inputs.OutputDIR,
		)
		cli.renderer.Newline()
		cli.renderer.Infof(
			"After running the above command and generating the state, " +
				"the auth0_import.tf file can be safely removed.\n",
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

func generateTerraformImportConfig(outputDIR string, data importDataList) error {
	if len(data) == 0 {
		return errors.New("no import data available")
	}

	if err := createOutputDirectory(outputDIR); err != nil {
		return err
	}

	if err := createMainFile(outputDIR); err != nil {
		return err
	}

	return createImportFile(outputDIR, data)
}

func createOutputDirectory(outputDIR string) error {
	const readWritePermission = 0755

	if err := os.MkdirAll(outputDIR, readWritePermission); err != nil && !os.IsExist(err) {
		return err
	}

	return nil
}

func createMainFile(outputDIR string) error {
	filePath := path.Join(outputDIR, "main.tf")

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	fileContent := `terraform {
  required_version = "~> 1.5.0"
  required_providers {
    auth0 = {
      source  = "auth0/auth0"
      version = "1.0.0-beta.1"
    }
  }
}

provider "auth0" {
  debug = true
}
`

	_, err = file.WriteString(fileContent)
	return err
}

func createImportFile(outputDIR string, data importDataList) error {
	filePath := path.Join(outputDIR, "auth0_import.tf")

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	fileContent := `# This file is automatically generated via the Auth0 CLI.
# It can be safely removed after the successful generation
# of Terraform resource definition files.
{{range .}}
import {
  id = "{{ .ImportID }}"
  to = {{ .ResourceName }}
}
{{end}}
`

	t, err := template.New("terraform").Parse(fileContent)
	if err != nil {
		return err
	}

	return t.Execute(file, data)
}

func generateTerraformResourceConfig(ctx context.Context, outputDIR string) error {
	absoluteOutputPath, err := filepath.Abs(outputDIR)
	if err != nil {
		return err
	}

	installer := &releases.ExactVersion{
		Product:    product.Terraform,
		Version:    version.Must(version.NewVersion("1.5.0")),
		InstallDir: absoluteOutputPath,
	}

	execPath, err := installer.Install(ctx)
	if err != nil {
		return err
	}

	tf, err := tfexec.NewTerraform(absoluteOutputPath, execPath)
	if err != nil {
		return err
	}

	if err = tf.Init(context.Background()); err != nil {
		return err
	}

	// -generate-config-out flag is not supported by terraform-exec, so we do this through exec.Command.
	cmd := exec.CommandContext(ctx, execPath, "plan", "-generate-config-out=generated.tf")
	cmd.Dir = absoluteOutputPath
	return cmd.Run()
}

func terraformProviderCredentialsAreAvailable() bool {
	domain := os.Getenv("AUTH0_DOMAIN")
	clientID := os.Getenv("AUTH0_CLIENT_ID")
	clientSecret := os.Getenv("AUTH0_CLIENT_SECRET")
	apiToken := os.Getenv("AUTH0_API_TOKEN")

	return (domain != "" && clientID != "" && clientSecret != "") || (domain != "" && apiToken != "")
}
