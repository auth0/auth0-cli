package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/keyring"
)

func logoutCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Args:  cobra.MaximumNArgs(1),
		Short: "Log out of a tenant's session",
		Long:  "Log out of a tenant's session.",
		Example: `  auth0 logout
  auth0 logout <tenant>
  auth0 logout "example.us.auth0.com"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			selectedTenant, err := selectValidTenantFromConfig(cli, cmd, args)
			if err != nil {
				return err
			}

			if err := cli.Config.RemoveTenant(selectedTenant); err != nil {
				return fmt.Errorf("failed to log out from the tenant %q: %w", selectedTenant, err)
			}

			if err := keyring.DeleteSecretsForTenant(selectedTenant); err != nil {
				return fmt.Errorf("failed to delete tenant secrets: %w", err)
			}

			cli.renderer.Infof("Successfully logged out from tenant: %s", selectedTenant)
			return nil
		},
	}

	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		_ = cmd.Flags().MarkHidden("tenant")
		cmd.Parent().HelpFunc()(cmd, args)
	})

	return cmd
}
