package cli

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

func customDomainsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "custom-domains",
		Short: "Manage resources for custom-domains",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(customDomainsListCmd(cli))
	cmd.AddCommand(customDomainsCreateCmd(cli))
	cmd.AddCommand(customDomainsDeleteCmd(cli))
	cmd.AddCommand(customDomainsGetCmd(cli))
	cmd.AddCommand(customDomainsVerifyCmd(cli))

	return cmd
}

func customDomainsListCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List custom domains configurations",
		Long: `
Retrieve details on custom domains.

  auth0 custom-domains list

`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var customDomains []*management.CustomDomain
			err := ansi.Spinner("Getting custom domains", func() error {
				var err error
				customDomains, err = cli.api.CustomDomain.List()
				return err
			})

			if err != nil {
				return err
			}

			cli.renderer.CustomDomainList(customDomains)
			return nil
		},
	}

	return cmd
}

func customDomainsCreateCmd(cli *cli) *cobra.Command {
	var flags struct {
		Domain             string
		Type               string
		VerificationMethod string
	}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Configure a new custom domain",
		Long: `
Create a new custom domain.

Note: The custom domain will need to be verified before it will accept requests.

  auth0 custom-domain create --domain example.org --type auth0_managed_certs --type txt

`,
		RunE: func(cmd *cobra.Command, args []string) error {

			if !cmd.Flags().Changed("domain") {
				qs := []*survey.Question{
					{
						Name: "Domain",
						Prompt: &survey.Input{
							Message: "Domain:",
							Help:    "Domain name.",
						},
					},
				}
				err := survey.Ask(qs, &flags)
				if err != nil {
					return err
				}
			}

			customDomain := &management.CustomDomain{
				Domain:             auth0.String(flags.Domain),
				Type:               auth0.String(flags.Type),
				VerificationMethod: auth0.String(flags.VerificationMethod),
			}

			err := ansi.Spinner("Creating custom domain", func() error {
				return cli.api.CustomDomain.Create(customDomain)
			})
			if err != nil {
				return err
			}

			cli.renderer.CustomDomainCreate(customDomain)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.Domain, "domain", "d", "", "Domain name.")
	cmd.Flags().StringVarP(&flags.Type, "type", "t", "auth0_managed_certs", "Custom domain provisioning type. Must be auth0_managed_certs or self_managed_certs. Defaults to auth0_managed_certs")
	cmd.Flags().StringVarP(&flags.VerificationMethod, "verification-method", "v", "txt", "Custom domain verification method. Must be txt.")

	return cmd
}

func customDomainsDeleteCmd(cli *cli) *cobra.Command {
	var flags struct {
		CustomDomainID string
	}
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a custom domain configuration",
		Long: `
Delete a custom domain and stop serving requests for it.

  auth0 custom-domains delete --custom-domain-id myCustomDomainID
`,
		RunE: func(cmd *cobra.Command, args []string) error {

			if !cmd.Flags().Changed("custom-domain-id") {
				qs := []*survey.Question{
					{
						Name: "CustomDomainID",
						Prompt: &survey.Input{
							Message: "CustomDomainID:",
							Help:    "ID of the custom domain to delete.",
						},
					},
				}
				err := survey.Ask(qs, &flags)
				if err != nil {
					return err
				}
			}

			return ansi.Spinner("Deleting custom domain", func() error {
				return cli.api.CustomDomain.Delete(flags.CustomDomainID)
			})
		},
	}

	cmd.Flags().StringVarP(&flags.CustomDomainID, "custom-domain-id", "i", "", "ID of the custom domain to delete.")

	return cmd
}

func customDomainsGetCmd(cli *cli) *cobra.Command {
	var flags struct {
		CustomDomainID string
	}
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get custom domain configuration",
		Long: `
Retrieve a custom domain configuration and status.

  auth0 custom-domain get --custom-domain-id myCustomDomainID
`,
		RunE: func(cmd *cobra.Command, args []string) error {

			if !cmd.Flags().Changed("custom-domain-id") {
				qs := []*survey.Question{
					{
						Name: "CustomDomainID",
						Prompt: &survey.Input{
							Message: "CustomDomainID:",
							Help:    "ID of the custom domain to retrieve.",
						},
					},
				}
				err := survey.Ask(qs, &flags)
				if err != nil {
					return err
				}
			}

			var customDomain *management.CustomDomain
			err := ansi.Spinner("Getting custom domain", func() error {
				var err error
				customDomain, err = cli.api.CustomDomain.Read(flags.CustomDomainID)
				return err
			})
			if err != nil {
				return err
			}

			cli.renderer.CustomDomainGet(customDomain)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.CustomDomainID, "custom-domain-id", "i", "", "ID of the custom domain to retrieve.")

	return cmd
}

func customDomainsVerifyCmd(cli *cli) *cobra.Command {
	var flags struct {
		CustomDomainID string
	}
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify a custom domain",
		Long: `
Run the verification process on a custom domain.

Note: Check the status field to see its verification status. Once verification is complete, it may take up to 10 minutes before the custom domain can start accepting requests.

For self_managed_certs, when the custom domain is verified for the first time, the response will also include the cname_api_key which you will need to configure your proxy. This key must be kept secret, and is used to validate the proxy requests.

Learn more about verifying custom domains that use Auth0 Managed certificates:
  - https://auth0.com/docs/custom-domains#step-2-verify-ownership

Learn more about verifying custom domains that use Self Managed certificates:
  - https://auth0.com/docs/custom-domains/self-managed-certificates#step-2-verify-ownership

  auth0 custom-domain verify --custom-domain-id myCustomDomainID
`,
		RunE: func(cmd *cobra.Command, args []string) error {

			if !cmd.Flags().Changed("custom-domain-id") {
				qs := []*survey.Question{
					{
						Name: "CustomDomainID",
						Prompt: &survey.Input{
							Message: "CustomDomainID:",
							Help:    "ID of the custom domain to verify.",
						},
					},
				}
				err := survey.Ask(qs, &flags)
				if err != nil {
					return err
				}
			}

			var customDomain *management.CustomDomain
			err := ansi.Spinner("Verifying custom domain", func() error {
				var err error
				customDomain, err = cli.api.CustomDomain.Verify(flags.CustomDomainID)
				return err
			})
			if err != nil {
				return err
			}

			cli.renderer.CustomDomainVerify(customDomain)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.CustomDomainID, "custom-domain-id", "i", "", "ID of the custom domain to retrieve.")

	return cmd
}
