package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var tenantDomain = Argument{
	Name: "Tenant",
	Help: "Tenant to select",
}

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
		Example: `  auth0 tenants list
  auth0 tenants ls`,
		RunE: func(cmd *cobra.Command, args []string) error {
			tenants, err := cli.listTenants()
			if err != nil {
				return fmt.Errorf("failed to load tenants: %w", err)
			}

			tenantNames := make([]string, len(tenants))
			for i, t := range tenants {
				tenantNames[i] = t.Domain
			}

			cli.renderer.TenantList(tenantNames)
			return nil
		},
	}

	return cmd
}

func useTenantCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "use",
		Args:  cobra.MaximumNArgs(1),
		Short: "Set the active tenant",
		Long:  "Set the active tenant for the Auth0 CLI.",
		Example: `  auth0 tenants use
  auth0 tenants use <tenant>
  auth0 tenants use "example.us.auth0.com"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			selectedTenant, err := selectTenant(cli, cmd, args)
			if err != nil {
				return err
			}

			cli.config.DefaultTenant = selectedTenant
			if err := cli.persistConfig(); err != nil {
				return fmt.Errorf("failed to set the default tenant: %w", err)
			}

			cli.renderer.Infof("Default tenant switched to: %s", selectedTenant)
			return nil
		},
	}

	return cmd
}

func openTenantCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "open",
		Args:  cobra.MaximumNArgs(1),
		Short: "Open the settings page of the tenant",
		Long:  "Open the tenant's settings page in the Auth0 Dashboard.",
		Example: `  auth0 tenants open
  auth0 tenants open <tenant>
  auth0 tenants open "example.us.auth0.com"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			selectedTenant, err := selectTenant(cli, cmd, args)
			if err != nil {
				return err
			}

			openManageURL(cli, selectedTenant, "tenant/general")
			return nil
		},
	}

	return cmd
}

func selectTenant(cli *cli, cmd *cobra.Command, args []string) (string, error) {
	var selectedTenant string

	if len(args) == 0 {
		err := tenantDomain.Pick(cmd, &selectedTenant, cli.tenantPickerOptions)
		return selectedTenant, err
	}

	selectedTenant = args[0]
	if _, ok := cli.config.Tenants[selectedTenant]; !ok {
		return "", fmt.Errorf(
			"failed to find tenant %s.\n\nRun 'auth0 login' to configure a new tenant.",
			selectedTenant,
		)
	}

	return selectedTenant, nil
}

func (c *cli) tenantPickerOptions() (pickerOptions, error) {
	tenants, err := c.listTenants()
	if err != nil {
		return nil, fmt.Errorf("failed to load tenants: %w", err)
	}

	var priorityOpts, opts pickerOptions
	for _, tenant := range tenants {
		opt := pickerOption{value: tenant.Domain, label: tenant.Domain}

		// Check if this is currently the default tenant.
		if tenant.Domain == c.config.DefaultTenant {
			priorityOpts = append(priorityOpts, opt)
		} else {
			opts = append(opts, opt)
		}
	}

	if len(opts)+len(priorityOpts) == 0 {
		return nil, fmt.Errorf("there are currently no tenants to pick from")
	}

	return append(priorityOpts, opts...), nil
}
