package cli

import (
	"fmt"

	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/spf13/cobra"
)

var (
	tenantDomain = Argument{
		Name: "Tenant",
		Help: "Tenant to select",
	}
)

func tenantsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tenants",
		Short: "Manage configured tenants",
		Long:  "Manage configured tenants.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(useTenantCmd(cli))
	cmd.AddCommand(listTenantCmd(cli))
	cmd.AddCommand(openTenantCmd(cli))
	return cmd
}

func listTenantCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your tenants",
		Long:    "List your tenants.",
		Example: "auth0 tenants list",
		RunE: func(cmd *cobra.Command, args []string) error {
			tens, err := cli.listTenants()
			if err != nil {
				return fmt.Errorf("Unable to load tenants due to an unexpected error: %w", err)
			}

			tenNames := make([]string, len(tens))
			for i, t := range tens {
				tenNames[i] = t.Domain
			}

			cli.renderer.TenantList(tenNames)
			return nil
		},
	}
	return cmd
}

func useTenantCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "use",
		Args:    cobra.MaximumNArgs(1),
		Short:   "Set the active tenant",
		Long:    "Set the active tenant.",
		Example: "auth0 tenants use <tenant>",
		RunE: func(cmd *cobra.Command, args []string) error {
			var selectedTenant string
			if len(args) == 0 {
				tens, err := cli.listTenants()
				if err != nil {
					return fmt.Errorf("Unable to load tenants due to an unexpected error: %w", err)
				}

				tenNames := make([]string, len(tens))
				for i, t := range tens {
					tenNames[i] = t.Domain
				}

				input := prompt.SelectInput("tenant", "Tenant:", "Tenant to activate", tenNames, tenNames[0], true)
				if err := prompt.AskOne(input, &selectedTenant); err != nil {
					return handleInputError(err)
				}
			} else {
				requestedTenant := args[0]
				t, ok := cli.config.Tenants[requestedTenant]
				if !ok {
					return fmt.Errorf("Unable to find tenant %s; run 'auth0 tenants use' to see your configured tenants or run 'auth0 login' to configure a new tenant", requestedTenant)
				}
				selectedTenant = t.Domain
			}

			cli.config.DefaultTenant = selectedTenant
			if err := cli.persistConfig(); err != nil {
				return fmt.Errorf("An error occurred while setting the default tenant: %w", err)
			}
			cli.renderer.Infof("Default tenant switched to: %s", selectedTenant)
			return nil
		},
	}

	return cmd
}

func openTenantCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Domain string
	}

	cmd := &cobra.Command{
		Use:     "open",
		Args:    cobra.MaximumNArgs(1),
		Short:   "Open tenant settings page in the Auth0 Dashboard",
		Long:    "Open tenant settings page in the Auth0 Dashboard.",
		Example: "auth0 tenants open <tenant>",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := tenantDomain.Pick(cmd, &inputs.Domain, cli.tenantPickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.Domain = args[0]

				if _, ok := cli.config.Tenants[inputs.Domain]; !ok {
					return fmt.Errorf("Unable to find tenant %s; run 'auth0 login' to configure a new tenant", inputs.Domain)
				}
			}

			openManageURL(cli, inputs.Domain, "tenant/general")
			return nil
		},
	}

	return cmd
}

func (c *cli) tenantPickerOptions() (pickerOptions, error) {
	tens, err := c.listTenants()
	if err != nil {
		return nil, fmt.Errorf("Unable to load tenants due to an unexpected error: %w", err)
	}

	var priorityOpts, opts pickerOptions

	for _, t := range tens {
		opt := pickerOption{value: t.Domain, label: t.Domain}

		// check if this is currently the default tenant.
		if t.Domain == c.config.DefaultTenant {
			priorityOpts = append(priorityOpts, opt)
		} else {
			opts = append(opts, opt)
		}
	}

	if len(opts)+len(priorityOpts) == 0 {
		return nil, errNoApps
	}

	return append(priorityOpts, opts...), nil
}
