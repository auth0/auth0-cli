package cli

import (
	"fmt"

	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/spf13/cobra"
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
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
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
