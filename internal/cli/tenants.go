package cli

import (
	"context"
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
  auth0 tenants ls
  auth0 tenants ls --json
  auth0 tenants ls --json-compact
  auth0 tenants ls --csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			tenants, err := cli.Config.ListAllTenants()
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

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

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
			selectedTenant, err := selectValidTenantFromConfig(cli, cmd, args)
			if err != nil {
				return err
			}

			if err := cli.Config.SetDefaultTenant(selectedTenant); err != nil {
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
			selectedTenant, err := selectValidTenantFromConfig(cli, cmd, args)
			if err != nil {
				return err
			}

			openManageURL(cli, selectedTenant, "tenant/general")
			return nil
		},
	}

	return cmd
}

func selectValidTenantFromConfig(cli *cli, cmd *cobra.Command, args []string) (string, error) {
	if len(args) > 0 {
		tenant, err := cli.Config.GetTenant(args[0])
		if err != nil {
			return "", err
		}

		return tenant.Domain, nil
	}

	var selectedTenant string
	err := tenantDomain.Pick(cmd, &selectedTenant, cli.tenantPickerOptions)
	return selectedTenant, err
}

func (c *cli) tenantPickerOptions(_ context.Context) (pickerOptions, error) {
	tenants, err := c.Config.ListAllTenants()
	if err != nil {
		return nil, fmt.Errorf("failed to load tenants: %w", err)
	}

	var priorityOpts, opts pickerOptions
	for _, tenant := range tenants {
		opt := pickerOption{value: tenant.Domain, label: tenant.Domain}

		if tenant.Domain == c.Config.DefaultTenant {
			priorityOpts = append(priorityOpts, opt)
		} else {
			opts = append(opts, opt)
		}
	}

	if len(opts)+len(priorityOpts) == 0 {
		return nil, fmt.Errorf("there are no tenants to pick from. Add tenants by running `auth0 login`")
	}

	return append(priorityOpts, opts...), nil
}
