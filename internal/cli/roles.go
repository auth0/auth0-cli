package cli

import (
	"fmt"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

func rolesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "roles",
		Short:   "Manage resources for roles",
		Long:    "Manage resources for roles.",
		Aliases: []string{"role"},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listRolesCmd(cli))

	return cmd
}

func listRolesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your roles",
		Long: `List your existing roles. To create one try:
auth0 roles create`,
		Example: `auth0 roles list
auth0 roles ls`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var list *management.RoleList

			if err := ansi.Waiting(func() error {
				var err error
				list, err = cli.api.Role.List()
				return err
			}); err != nil {
				return fmt.Errorf("An unexpected error occurred: %w", err)
			}

			cli.renderer.RoleList(list.Roles)
			return nil
		},
	}

	return cmd
}
