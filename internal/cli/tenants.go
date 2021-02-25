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
	}

	cmd.AddCommand(useTenantCmd(cli))
	return cmd
}

func useTenantCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "use",
		Aliases: []string{"select"},
		Short:   "Set the active tenant",
		Long:    `auth0 tenants use <tenant>`,
		Args:    cobra.MaximumNArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var selectedTenant string
			if len(args) == 0 {
				tens, err := cli.listTenants()
				if err != nil {
					return fmt.Errorf("unable to load tenants from config")
				}

				tenNames := make([]string, len(tens))
				for i, t := range tens {
					tenNames[i] = t.Name
				}

				input := prompt.SelectInput("tenant", "Tenant:", "Tenant to activate", tenNames, true)
				if err := prompt.AskOne(input, &selectedTenant); err != nil {
					return err
				}
			} else {
				requestedTenant := args[0]
				t, ok := cli.config.Tenants[requestedTenant]
				if !ok {
					return fmt.Errorf("Unable to find tenant in config: %s", requestedTenant)

				}
				selectedTenant = t.Name
			}

			cli.config.DefaultTenant = selectedTenant
			if err := cli.persistConfig(); err != nil {
				return fmt.Errorf("persisting config: %w", err)
			}
			return nil
		},
	}

	return cmd
}
