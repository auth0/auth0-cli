package cli

import (
	"errors"
	"fmt"

	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/spf13/cobra"
)

func logoutCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Logout of a tenant's session",
		Long:  `auth0 logout <tenant>`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// NOTE(cyx): This was mostly copy/pasted from tenants
			// use command. Consider refactoring.
			var selectedTenant string
			if len(args) == 0 {
				tens, err := cli.listTenants()
				if err != nil {
					return err // This error is already formatted for display
				}

				if len(tens) == 0 {
					return errors.New("there are no tenants available to perform the logout")
				}

				tenNames := make([]string, len(tens))
				for i, t := range tens {
					tenNames[i] = t.Name
				}

				input := prompt.SelectInput("tenant", "Tenant:", "Tenant to logout", tenNames, tenNames[0], true)
				if err := prompt.AskOne(input, &selectedTenant); err != nil {
					return fmt.Errorf("An unexpected error occurred: %w", err)
				}
			} else {
				requestedTenant := args[0]
				t, ok := cli.config.Tenants[requestedTenant]
				if !ok {
					return fmt.Errorf("Unable to find tenant %s; run 'auth0 tenants use' to see your configured tenants or run 'auth0 login' to configure a new tenant", requestedTenant)
				}
				selectedTenant = t.Name
			}

			if err := cli.removeTenant(selectedTenant); err != nil {
				return err // This error is already formatted for display
			}

			cli.renderer.Infof("Successfully logged out tenant: %s", selectedTenant)
			return nil
		},
	}

	return cmd
}
