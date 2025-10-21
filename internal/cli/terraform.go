package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/prompt"
)

var tfFlags = terraformFlags{
	OutputDIR: Flag{
		Name:      "Output Dir",
		LongForm:  "output-dir",
		ShortForm: "o",
		Help: "Output directory for the generated Terraform config files. If not provided, the files will be " +
			"saved in the current working directory.",
	},
	Resources: Flag{
		Name:      "Resource Types",
		LongForm:  "resources",
		ShortForm: "r",
		Help: "Resource types to generate Terraform config for. If not provided, config files for all " +
			"available resources will be generated.",
	},
	TerraformVersion: Flag{
		Name:      "Terraform Version",
		LongForm:  "tf-version",
		ShortForm: "v",
		Help: "Terraform version that ought to be used while generating the terraform files for resources. " +
			"If not provided, 1.5.0 is used by default",
	},
}

type (
	terraformFlags struct {
		OutputDIR        Flag
		Resources        Flag
		TerraformVersion Flag
	}

	terraformInputs struct {
		OutputDIR        string
		Resources        []string
		TerraformVersion string
	}
)

func (i *terraformInputs) parseResourceFetchers(api *auth0.API) ([]resourceDataFetcher, error) {
	fetchers := make([]resourceDataFetcher, 0)
	var err error

	for _, resource := range i.Resources {
		switch resource {
		case "auth0_action":
			fetchers = append(fetchers, &actionResourceFetcher{api})
		case "auth0_attack_protection":
			fetchers = append(fetchers, &attackProtectionResourceFetcher{})
		case "auth0_branding":
			fetchers = append(fetchers, &brandingResourceFetcher{})
		case "auth0_branding_theme":
			fetchers = append(fetchers, &brandingThemeResourceFetcher{api})
		case "auth0_phone_provider":
			fetchers = append(fetchers, &phoneProviderResourceFetcher{api})
		case "auth0_client", "auth0_client_credentials":
			fetchers = append(fetchers, &clientResourceFetcher{api})
		case "auth0_client_grant":
			fetchers = append(fetchers, &clientGrantResourceFetcher{api})
		case "auth0_connection", "auth0_connection_clients":
			fetchers = append(fetchers, &connectionResourceFetcher{api})
		case "auth0_custom_domain":
			fetchers = append(fetchers, &customDomainResourceFetcher{api})
		case "auth0_email_provider":
			fetchers = append(fetchers, &emailProviderResourceFetcher{api})
		case "auth0_email_template":
			fetchers = append(fetchers, &emailTemplateResourceFetcher{api})
		case "auth0_flow":
			fetchers = append(fetchers, &flowResourceFetcher{api})
		case "auth0_flow_vault_connection":
			fetchers = append(fetchers, &flowVaultConnectionResourceFetcher{api})
		case "auth0_form":
			fetchers = append(fetchers, &formResourceFetcher{api})
		case "auth0_guardian":
			fetchers = append(fetchers, &guardianResourceFetcher{})
		case "auth0_log_stream":
			fetchers = append(fetchers, &logStreamResourceFetcher{api})
		case "auth0_organization", "auth0_organization_connections":
			fetchers = append(fetchers, &organizationResourceFetcher{api})
		case "auth0_network_acl":
			fetchers = append(fetchers, &networkACLResourceFetcher{api})
		case "auth0_pages":
			fetchers = append(fetchers, &pagesResourceFetcher{})
		case "auth0_prompt":
			fetchers = append(fetchers, &promptResourceFetcher{})
		case "auth0_prompt_custom_text":
			fetchers = append(fetchers, &promptCustomTextResourceFetcherResourceFetcher{api})
		case "auth0_prompt_screen_renderer":
			fetchers = append(fetchers, &promptScreenRendererResourceFetcher{api})
		case "auth0_resource_server", "auth0_resource_server_scopes":
			fetchers = append(fetchers, &resourceServerResourceFetcher{api})
		case "auth0_role", "auth0_role_permissions":
			fetchers = append(fetchers, &roleResourceFetcher{api})
		case "auth0_self_service_profile":
			fetchers = append(fetchers, &selfServiceProfileFetcher{api})
		case "auth0_tenant":
			fetchers = append(fetchers, &tenantResourceFetcher{})
		case "auth0_trigger_actions":
			fetchers = append(fetchers, &triggerActionsResourceFetcher{api})
		case "auth0_user_attribute_profile":
			fetchers = append(fetchers, &userAttributeProfilesResourceFetcher{api})
		default:
			err = errors.Join(err, fmt.Errorf("unsupported resource type: %s", resource))
		}
	}

	return fetchers, err
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
		Aliases: []string{"gen"},
		Short:   "Generate terraform configuration for your Auth0 Tenant",
		Long: "(Experimental) This command is designed to streamline the process of generating Terraform configuration files for " +
			"your Auth0 resources, serving as a bridge between the two.\n\nIt automatically scans your Auth0 Tenant " +
			"and compiles a set of Terraform configuration files (HCL) based on the existing resources and configurations." +
			"\n\nRefer to the [instructional guide](https://registry.terraform.io/providers/auth0/auth0/latest/docs/guides/generate_terraform_config) for specific details on how to use this command." +
			"\n\n**Warning:** This command is experimental and is subject to change in future versions.",
		Example: `  auth0 tf generate
  auth0 tf generate -o tmp-auth0-tf
  auth0 tf generate -o tmp-auth0-tf -r auth0_client
  auth0 tf generate --output-dir tmp-auth0-tf --resources auth0_action,auth0_tenant,auth0_client `,
		RunE: generateTerraformCmdRun(cli, &inputs),
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")
	tfFlags.OutputDIR.RegisterString(cmd, &inputs.OutputDIR, "./")
	tfFlags.Resources.RegisterStringSlice(cmd, &inputs.Resources, defaultResources)
	tfFlags.TerraformVersion.RegisterString(cmd, &inputs.TerraformVersion, "1.5.0")

	return cmd
}

func generateTerraformCmdRun(cli *cli, inputs *terraformInputs) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		resources, err := inputs.parseResourceFetchers(cli.api)
		if err != nil {
			return err
		}

		var data importDataList
		err = ansi.Spinner("Fetching data from Auth0", func() error {
			data, err = fetchImportData(cmd.Context(), cli, resources...)
			return err
		})
		if err != nil {
			return err
		}

		if !checkOutputDirectoryIsEmpty(cli, cmd, inputs.OutputDIR) {
			return nil
		}

		if err := cleanOutputDirectory(inputs.OutputDIR); err != nil {
			return err
		}

		if err := generateTerraformImportConfig(inputs, data); err != nil {
			return err
		}

		cdInstructions := ""
		if inputs.OutputDIR != "./" {
			cdInstructions = fmt.Sprintf("cd %s && ", inputs.OutputDIR)
		}

		if terraformProviderCredentialsAreAvailable() {
			err := checkTerraformProviderAndCLIDomainsMatch(cli.Config.DefaultTenant)
			if err != nil {
				return err
			}

			err = ansi.Spinner("Generating Terraform configuration", func() error {
				return generateTerraformResourceConfig(cmd.Context(), inputs)
			})

			if err != nil {
				cli.renderer.Warnf("Terraform resource config generated successfully but there was an error with terraform plan.\n\n")
				cli.renderer.Warnf("Run " + ansi.Cyan(cdInstructions+"./terraform plan") + " to troubleshoot\n\n")
				cli.renderer.Warnf("Once the plan succeeds, run " + ansi.Cyan("./terraform apply") + " to complete the import.\n\n")
				cli.renderer.Infof("The terraform binary and auth0_import.tf files can be deleted afterwards.\n")
				return nil
			}

			cli.renderer.Infof("Terraform resource config files generated successfully in: %s", inputs.OutputDIR)
			cli.renderer.Infof(
				"Review the config and generate the terraform state by running: \n\n	" + ansi.Cyan(cdInstructions+"./terraform apply") + "\n",
			)
			cli.renderer.Infof(
				"Once Terraform files are auto-generated, the terraform binary and auth0_import.tf files can be deleted.\n",
			)

			return nil
		}

		cli.renderer.Errorf("Terraform provider credentials not detected\n")
		cli.renderer.Warnf(
			"Refer to following guide on how to create a dedicated Auth0 client and configure credentials: " +
				ansi.URL("https://registry.terraform.io/providers/auth0/auth0/latest/docs/guides/quickstart") + "\n\n" +
				"After provider credentials are set, run: \n\n" +
				ansi.Cyan(cdInstructions+"terraform init && terraform plan -generate-config-out=auth0_generated.tf && terraform apply") + "\n\n" +
				"Once the Terraform file is auto-generated, the auth0_import.tf file can be deleted.\n",
		)

		return nil
	}
}

func fetchImportData(ctx context.Context, cli *cli, fetchers ...resourceDataFetcher) (importDataList, error) {
	var importData importDataList

	for _, fetcher := range fetchers {
		data, err := fetcher.FetchData(ctx)
		if err != nil {
			// Checking for the forbidden scenario and skip.
			if strings.Contains(err.Error(), "403 Forbidden") {
				cli.renderer.Warnf("Skipping resource due to forbidden access: %s", err.Error())
				continue
			}

			if strings.Contains(err.Error(), "402 Payment Required") {
				cli.renderer.Warnf("Skipping resource due to payment required: %s", err.Error())
				continue
			}

			return nil, err
		}

		importData = append(importData, data...)
	}

	return deduplicateResourceNames(importData), nil
}

func generateTerraformImportConfig(inputs *terraformInputs, data importDataList) error {
	if len(data) == 0 {
		return errors.New("no import data available")
	}

	if err := createOutputDirectory(inputs.OutputDIR); err != nil {
		return err
	}

	if err := createMainFile(inputs); err != nil {
		return err
	}

	return createImportFile(inputs.OutputDIR, data)
}

func createOutputDirectory(outputDIR string) error {
	const readWritePermission = 0755

	if err := os.MkdirAll(outputDIR, readWritePermission); err != nil && !os.IsExist(err) {
		return err
	}

	return nil
}

func createMainFile(input *terraformInputs) error {
	filePath := path.Join(input.OutputDIR, "auth0_main.tf")

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	fileContent := `terraform {
  required_version = ">= ` + input.TerraformVersion + `"
  required_providers {
    auth0 = {
      source  = "auth0/auth0"
      version = ">= 1.0.0"
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
	defer func() {
		_ = file.Close()
	}()

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

func generateTerraformResourceConfig(ctx context.Context, input *terraformInputs) error {
	absoluteOutputPath, err := filepath.Abs(input.OutputDIR)
	if err != nil {
		return err
	}

	installer := &releases.ExactVersion{
		Product:    product.Terraform,
		Version:    version.Must(version.NewVersion(input.TerraformVersion)),
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
	cmd := exec.CommandContext(ctx, execPath, "plan", "-generate-config-out=auth0_generated.tf")
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

func checkTerraformProviderAndCLIDomainsMatch(currentCLIDomain string) error {
	providerDomain := os.Getenv("AUTH0_DOMAIN")
	if providerDomain == currentCLIDomain {
		return nil
	}
	return fmt.Errorf("terraform provider tenant domain %q does not match current CLI tenant %q", providerDomain, currentCLIDomain)
}

func deduplicateResourceNames(data importDataList) importDataList {
	nameMap := map[string]int{}
	deduplicatedList := importDataList{}

	for _, resource := range data {
		nameMap[resource.ResourceName]++
		if nameMap[resource.ResourceName] > 1 {
			resource.ResourceName = fmt.Sprintf("%s_%d", resource.ResourceName, nameMap[resource.ResourceName])
		}

		deduplicatedList = append(deduplicatedList, resource)
	}

	return deduplicatedList
}

func checkOutputDirectoryIsEmpty(cli *cli, cmd *cobra.Command, outputDIR string) bool {
	_, err := os.Stat(outputDIR)
	if os.IsNotExist(err) {
		return true
	}

	_, mainFileErr := os.Stat(path.Join(outputDIR, "auth0_main.tf"))
	_, importFileErr := os.Stat(path.Join(outputDIR, "auth0_import.tf"))
	_, generatedFileErr := os.Stat(path.Join(outputDIR, "auth0_generated.tf"))
	if os.IsNotExist(mainFileErr) && os.IsNotExist(importFileErr) && os.IsNotExist(generatedFileErr) {
		return true
	}

	cli.renderer.Warnf(
		"Output directory %q is not empty. "+
			"Proceeding will overwrite the auth0_main.tf, auth0_import.tf and auth0_generated.tf files.",
		outputDIR,
	)

	if !cli.force && canPrompt(cmd) {
		if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
			return false
		}
	}

	return true
}

func cleanOutputDirectory(outputDIR string) error {
	var joinedErrors error

	if err := os.Remove(path.Join(outputDIR, "auth0_main.tf")); err != nil && !os.IsNotExist(err) {
		joinedErrors = errors.Join(err)
	}

	if err := os.Remove(path.Join(outputDIR, "auth0_import.tf")); err != nil && !os.IsNotExist(err) {
		joinedErrors = errors.Join(err)
	}

	if err := os.Remove(path.Join(outputDIR, "auth0_generated.tf")); err != nil && !os.IsNotExist(err) {
		joinedErrors = errors.Join(err)
	}

	return joinedErrors
}

// sanitizeResourceName will return a valid terraform resource name.
//
// A name must start with a letter or underscore and may
// contain only letters, digits, underscores, and dashes.
func sanitizeResourceName(name string) string {
	// Regular expression pattern to remove invalid characters.
	namePattern := "[^a-zA-Z0-9_]+"
	re := regexp.MustCompile(namePattern)

	sanitizedName := re.ReplaceAllString(name, "_")

	// Regular expression pattern to remove leading digits or dashes.
	namePattern = "^[0-9-]+"
	re = regexp.MustCompile(namePattern)

	sanitizedName = re.ReplaceAllString(sanitizedName, "")
	sanitizedName = strings.Trim(sanitizedName, "_")
	sanitizedName = strings.ToLower(sanitizedName)

	return sanitizedName
}
