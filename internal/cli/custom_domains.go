package cli

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/prompt"
)

const (
	customDomainProvisioningTypeAuth0 = "auth0_managed_certs"
	customDomainProvisioningTypeSelf  = "self_managed_certs"
	customDomainVerificationMethodTxt = "txt"
	customDomainTLSPolicyRecommended  = "recommended"
	customDomainTLSPolicyCompatible   = "compatible"
)

var (
	customDomainID = Argument{
		Name: "Id",
		Help: "Id of the custom domain.",
	}

	customDomainDomain = Flag{
		Name:       "Domain",
		LongForm:   "domain",
		ShortForm:  "d",
		Help:       "Domain name.",
		IsRequired: true,
	}

	customDomainType = Flag{
		Name:      "Provisioning Type",
		LongForm:  "type",
		ShortForm: "t",
		Help:      "Custom domain provisioning type. Must be 'auth0' for Auth0-managed certs or 'self' for self-managed certs.",
	}

	customDomainVerification = Flag{
		Name:      "Verification Method",
		LongForm:  "verification",
		ShortForm: "v",
		Help:      "Custom domain verification method. Must be 'txt'.",
	}

	customDomainPolicy = Flag{
		Name:         "TLS Policy",
		LongForm:     "policy",
		ShortForm:    "p",
		Help:         "The TLS version policy. Can be either 'compatible' or 'recommended'.",
		AlwaysPrompt: true,
	}

	customDomainIPHeader = Flag{
		Name:         "Custom Client IP Header",
		LongForm:     "ip-header",
		ShortForm:    "i",
		Help:         "The HTTP header to fetch the client's IP address.",
		AlwaysPrompt: true,
	}

	customDomainPolicyOptions = []string{
		customDomainTLSPolicyRecommended,
		customDomainTLSPolicyCompatible,
	}
)

func customDomainsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "domains",
		Short: "Manage custom domains",
		Long: "With a custom domain, your users feel confident that they are providing their credentials to the " +
			"right party. Authentication happens within the context of your brand which helps you build brand " +
			"loyalty. Users are not redirected to a third-party site that breaks the branding context. " +
			"This prevents users from becoming confused about whether they are still making a transaction " +
			"or operation with you.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listCustomDomainsCmd(cli))
	cmd.AddCommand(showCustomDomainCmd(cli))
	cmd.AddCommand(createCustomDomainCmd(cli))
	cmd.AddCommand(updateCustomDomainCmd(cli))
	cmd.AddCommand(deleteCustomDomainCmd(cli))
	cmd.AddCommand(verifyCustomDomainCmd(cli))

	return cmd
}

func listCustomDomainsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your custom domains",
		Long:    "List your existing custom domains. To create one, run: `auth0 domains create`.",
		Example: `  auth0 domains list
  auth0 domains ls
  auth0 domains ls --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var list []*management.CustomDomain

			if err := ansi.Waiting(func() error {
				var err error
				list, err = cli.api.CustomDomain.List(cmd.Context(), management.PerPage(defaultPageSize))
				return err
			}); err != nil {
				return fmt.Errorf("An unexpected error occurred: %w", err)
			}

			cli.renderer.CustomDomainList(list)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

	return cmd
}

func showCustomDomainCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show a custom domain",
		Long:  "Display information about a custom domain.",
		Example: `  auth0 domains show 
  auth0 domains show <domain-id>
  auth0 domains show <domain-id> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := customDomainID.Pick(cmd, &inputs.ID, cli.customDomainsPickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var customDomain *management.CustomDomain

			if err := ansi.Waiting(func() error {
				var err error
				customDomain, err = cli.api.CustomDomain.Read(cmd.Context(), url.PathEscape(inputs.ID))
				return err
			}); err != nil {
				return fmt.Errorf("Unable to get a custom domain with Id '%s': %w", inputs.ID, err)
			}

			cli.renderer.CustomDomainShow(customDomain)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

	return cmd
}

func createCustomDomainCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Domain               string
		Type                 string
		VerificationMethod   string
		TLSPolicy            string
		CustomClientIPHeader string
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a custom domain",
		Long: "Create a custom domain.\n\n" +
			"To create interactively, use `auth0 domains create` with no arguments.\n\n" +
			"To create non-interactively, supply the domain name, type, policy and " +
			"other information through the flags.",
		Example: `  auth0 domains create
  auth0 domains create --domain <domain-name>
  auth0 domains create --domain <domain-name> --policy recommended
  auth0 domains create --domain <domain-name> --policy recommended --type auth0
  auth0 domains create --domain <domain-name> --policy recommended --type auth0 --ip-header "cf-connecting-ip"
  auth0 domains create -d <domain-name> -p recommended -t auth0 -i "cf-connecting-ip" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := customDomainDomain.Ask(cmd, &inputs.Domain, nil); err != nil {
				return err
			}

			customDomain := &management.CustomDomain{
				Domain: &inputs.Domain,
			}

			if inputs.Type != "" {
				customDomain.Type = apiProvisioningTypeFor(inputs.Type)
			} else {
				customDomain.Type = auth0.String(customDomainProvisioningTypeAuth0)
			}

			if inputs.VerificationMethod != "" {
				customDomain.VerificationMethod = apiVerificationMethodFor(inputs.VerificationMethod)
			}

			if inputs.TLSPolicy != "" {
				customDomain.TLSPolicy = apiTLSPolicyFor(inputs.TLSPolicy)
			}

			if inputs.CustomClientIPHeader != "" {
				customDomain.CustomClientIPHeader = &inputs.CustomClientIPHeader
			}

			if err := ansi.Waiting(func() error {
				return cli.api.CustomDomain.Create(cmd.Context(), customDomain)
			}); err != nil {
				return fmt.Errorf("An unexpected error occurred while attempting to create the custom domain '%s': %w", inputs.Domain, err)
			}

			cli.renderer.CustomDomainCreate(customDomain)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	customDomainDomain.RegisterString(cmd, &inputs.Domain, "")
	customDomainType.RegisterString(cmd, &inputs.Type, "")
	customDomainVerification.RegisterString(cmd, &inputs.VerificationMethod, "")
	customDomainPolicy.RegisterString(cmd, &inputs.TLSPolicy, "")
	customDomainIPHeader.RegisterString(cmd, &inputs.CustomClientIPHeader, "")

	return cmd
}

func updateCustomDomainCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID                   string
		TLSPolicy            string
		CustomClientIPHeader string
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update a custom domain",
		Long: "Update a custom domain.\n\n" +
			"To update interactively, use `auth0 domains update` with no arguments.\n\n" +
			"To update non-interactively, supply the domain name, type, policy and " +
			"other information through the flags.",
		Example: `  auth0 domains update
  auth0 domains update <domain-id> --policy compatible
  auth0 domains update <domain-id> --policy compatible --ip-header "cf-connecting-ip"
  auth0 domains update <domain-id> -p compatible -i "cf-connecting-ip" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var current *management.CustomDomain

			if len(args) == 0 {
				err := customDomainID.Pick(cmd, &inputs.ID, cli.customDomainsPickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			// Load custom domain by id
			if err := ansi.Waiting(func() error {
				var err error
				current, err = cli.api.CustomDomain.Read(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("Unable to load custom domain: %w", err)
			}

			// Prompt for TLS policy
			if err := customDomainPolicy.SelectU(cmd, &inputs.TLSPolicy, customDomainPolicyOptions, current.TLSPolicy); err != nil {
				return err
			}

			// Prompt for custom domain custom client IP header
			if err := customDomainIPHeader.AskU(cmd, &inputs.CustomClientIPHeader, current.CustomClientIPHeader); err != nil {
				return err
			}

			// Start with an empty custom domain object. We'll conditionally
			// hydrate it based on the provided parameters since
			// we'll do PATCH semantics.
			c := &management.CustomDomain{}

			if inputs.TLSPolicy != "" {
				c.TLSPolicy = apiTLSPolicyFor(inputs.TLSPolicy)
			}

			if inputs.CustomClientIPHeader != "" {
				c.CustomClientIPHeader = &inputs.CustomClientIPHeader
			}

			// Update custom domain
			if err := ansi.Waiting(func() error {
				return cli.api.CustomDomain.Update(cmd.Context(), inputs.ID, c)
			}); err != nil {
				return fmt.Errorf("Unable to update custom domain: %v", err)
			}

			// Render custom domain update specific view
			cli.renderer.CustomDomainUpdate(c)
			return nil
		},
	}

	customDomainPolicy.RegisterStringU(cmd, &inputs.TLSPolicy, "")
	customDomainIPHeader.RegisterStringU(cmd, &inputs.CustomClientIPHeader, "")

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

	return cmd
}

func deleteCustomDomainCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Short:   "Delete a custom domain",
		Long: "Delete a custom domain.\n\n" +
			"To delete interactively, use `auth0 domains delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the custom domain id and the `--force` flag to " +
			"skip confirmation.",
		Example: `  auth0 domains delete
  auth0 domains rm
  auth0 domains delete <domain-id>
  auth0 domains delete <domain-id> --force
  auth0 domains delete <domain-id> <domain-id2>
  auth0 domains delete <domain-id> <domain-id2> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ids := make([]string, len(args))
			if len(args) == 0 {
				err := customDomainID.PickMany(cmd, &ids, cli.customDomainsPickerOptions)
				if err != nil {
					return err
				}
			} else {
				ids = append(ids, args...)
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			return ansi.Spinner("Deleting custom domain", func() error {
				var errs []error
				for _, id := range ids {
					if id != "" {
						if _, err := cli.api.CustomDomain.Read(cmd.Context(), url.PathEscape(id)); err != nil {
							return fmt.Errorf("Unable to delete custom domain (%s): %w", id, err)
						}

						if err := cli.api.CustomDomain.Delete(cmd.Context(), url.PathEscape(id)); err != nil {
							return fmt.Errorf("Unable to delete custom domain (%s): %w", id, err)
						}
					}
				}

				return errors.Join(errs...)
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

func verifyCustomDomainCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "verify",
		Args:  cobra.MaximumNArgs(1),
		Short: "Verify a custom domain",
		Long: "Verify a custom domain.\n\n" +
			"To verify interactively, use `auth0 domains verify` with no arguments.\n\n" +
			"To verify non-interactively, supply the custom domain id.",
		Example: `  auth0 domains verify 
  auth0 domains verify <domain-id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := customDomainID.Pick(cmd, &inputs.ID, cli.customDomainsPickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var customDomain *management.CustomDomain

			if err := ansi.Waiting(func() error {
				var err error
				customDomain, err = cli.api.CustomDomain.Verify(cmd.Context(), url.PathEscape(inputs.ID))
				return err
			}); err != nil {
				return fmt.Errorf("Unable to verify a custom domain with Id '%s': %w", inputs.ID, err)
			}

			cli.renderer.CustomDomainShow(customDomain)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

	return cmd
}

func apiProvisioningTypeFor(v string) *string {
	switch v {
	case "auth0":
		return auth0.String(customDomainProvisioningTypeAuth0)
	case "self":
		return auth0.String(customDomainProvisioningTypeSelf)
	default:
		return auth0.String(v)
	}
}

func apiVerificationMethodFor(v string) *string {
	switch v {
	case "txt":
		return auth0.String(customDomainVerificationMethodTxt)
	default:
		return auth0.String(v)
	}
}

func apiTLSPolicyFor(v string) *string {
	switch v {
	case "recommended":
		return auth0.String(customDomainTLSPolicyRecommended)
	case "compatible":
		return auth0.String(customDomainTLSPolicyCompatible)
	default:
		return auth0.String(v)
	}
}

func (c *cli) customDomainsPickerOptions(ctx context.Context) (pickerOptions, error) {
	var opts pickerOptions

	domains, err := c.api.CustomDomain.List(ctx)
	if err != nil {
		errStatus := err.(management.Error)
		// 403 is a valid response for free tenants that don't have
		// custom domains enabled
		if errStatus != nil && errStatus.Status() == 403 {
			return nil, errNoCustomDomains
		}

		return nil, err
	}

	for _, d := range domains {
		value := d.GetID()
		label := fmt.Sprintf("%s %s", d.GetDomain(), ansi.Faint("("+value+")"))
		opts = append(opts, pickerOption{value: value, label: label})
	}

	if len(opts) == 0 {
		return nil, errNoCustomDomains
	}

	return opts, nil
}
