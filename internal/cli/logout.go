package cli

import (
	"fmt"

	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/spf13/cobra"
)

func logoutCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Logout of a tenant's session",
		Long:  `auth0 logout <tenant>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// NOTE(cyx): This was mostly copy/pasted from tenants
			// use command. Consider refactoring.
			var selectedTenant string
			if len(args) == 0 {
				tens, err := cli.listTenants()
				if err != nil {
					return fmt.Errorf("Unable to load tenants due to an unexpected error: %w", err)
				}

				tenNames := make([]string, len(tens))
				for i, t := range tens {
					tenNames[i] = t.Name
				}

				input := prompt.SelectInput("tenant", "Tenant:", "Tenant to activate", tenNames, true)
				if err := prompt.AskOne(input, &selectedTenant); err != nil {
					return fmt.Errorf("An unexpected error occurred: %w", err)
				}
			} else {
				requestedTenant := args[0]
				t, ok := cli.config.Tenants[requestedTenant]
				if !ok {
					return fmt.Errorf("Unable to find tenant %s; run `auth0 tenants use` to see your configured tenants or run `auth0 login` to configure a new tenant", requestedTenant)
				}
				selectedTenant = t.Name
			}

			if err := cli.removeTenant(selectedTenant); err != nil {
				return fmt.Errorf("Unexpected error logging out tenant: %s: %v", selectedTenant, err)
			}

			cli.renderer.Infof("Successfully logged out tenant: %s", selectedTenant)
			return nil
		},
	}

	return cmd
}
