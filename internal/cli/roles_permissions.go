package cli

import (
	"fmt"
	"net/url"

	"github.com/AlecAivazis/survey/v2"
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/spf13/cobra"
	"github.com/auth0/go-auth0"
	"github.com/auth0/go-auth0/management"
)

var (
	roleAPIIdentifier = Flag{
		Name:       "API",
		LongForm:   "api-id",
		ShortForm:  "a",
		Help:       "API Identifier.",
		IsRequired: true,
	}

	roleAPIPermissions = Flag{
		Name:       "Permissions",
		LongForm:   "permissions",
		ShortForm:  "p",
		Help:       "Permissions.",
		IsRequired: true,
	}
)

func rolePermissionsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "permissions",
		Short: "Manage permissions within the role resource",
		Long:  "Manage permissions within the role resource.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listRolePermissionsCmd(cli))
	cmd.AddCommand(addRolePermissionsCmd(cli))
	cmd.AddCommand(removeRolePermissionsCmd(cli))

	return cmd
}

func listRolePermissionsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "List permissions defined within a role",
		Long: `List existing permissions defined in a role. To add a permission try:
auth0 roles permissions add <role-id>`,
		Example: `auth0 roles permissions list <role-id>
auth0 roles permissions ls`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := roleID.Pick(cmd, &inputs.ID, cli.rolePickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var list *management.PermissionList

			if err := ansi.Waiting(func() error {
				var err error
				list, err = cli.api.Role.Permissions(inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("An unexpected error occurred: %w", err)
			}

			cli.renderer.RolePermissionList(list.Permissions)
			return nil
		},
	}

	return cmd
}

func addRolePermissionsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID            string
		APIIdentifier string
		Permissions   []string
	}

	cmd := &cobra.Command{
		Use:   "add",
		Args:  cobra.MaximumNArgs(1),
		Short: "Add a permission to a role",
		Long: `Add an existing permission defined in one of your APIs.
To add a permission try:

    auth0 roles permissions add <role-id> -p <permission-name>`,
		Example: `auth0 roles permissions add <role-id> -p <permission-name>
auth0 roles permissions add`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := roleID.Pick(cmd, &inputs.ID, cli.rolePickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			if err := roleAPIIdentifier.Pick(cmd, &inputs.APIIdentifier, cli.apiPickerOptionsWithoutAuth0); err != nil {
				return err
			}

			var rs *management.ResourceServer
			rs, err := cli.api.ResourceServer.Read(url.PathEscape(inputs.APIIdentifier))
			if err != nil {
				return err
			}

			if len(inputs.Permissions) == 0 {
				err := cli.pickRolePermissions(rs.Scopes, &inputs.Permissions)
				if err != nil {
					return err
				}
			}

			ps := makePermissions(rs.GetIdentifier(), inputs.Permissions)
			if err := cli.api.Role.AssociatePermissions(inputs.ID, ps); err != nil {
				return err
			}

			role, err := cli.api.Role.Read(inputs.ID)
			if err != nil {
				return err
			}

			cli.renderer.RolePermissionAdd(role, rs, inputs.Permissions)
			return nil
		},
	}

	roleAPIIdentifier.RegisterString(cmd, &inputs.APIIdentifier, "")
	roleAPIPermissions.RegisterStringSlice(cmd, &inputs.Permissions, nil)
	return cmd
}

func removeRolePermissionsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID            string
		APIIdentifier string
		Permissions   []string
	}

	cmd := &cobra.Command{
		Use:     "remove",
		Aliases: []string{"rm"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "Remove a permission from a role",
		Long: `Remove an existing permission defined in one of your APIs.
To remove a permission try:

    auth0 roles permissions remove <role-id> -p <permission-name>`,
		Example: `auth0 roles permissions remove <role-id> -p <permission-name>
auth0 roles permissions rm`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := roleID.Pick(cmd, &inputs.ID, cli.rolePickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			if err := roleAPIIdentifier.Pick(cmd, &inputs.APIIdentifier, cli.apiPickerOptionsWithoutAuth0); err != nil {
				return err
			}

			var rs *management.ResourceServer
			rs, err := cli.api.ResourceServer.Read(url.PathEscape(inputs.APIIdentifier))
			if err != nil {
				return err
			}

			if len(inputs.Permissions) == 0 {
				err := cli.pickRolePermissions(rs.Scopes, &inputs.Permissions)
				if err != nil {
					return err
				}
			}

			ps := makePermissions(rs.GetIdentifier(), inputs.Permissions)
			if err := cli.api.Role.RemovePermissions(inputs.ID, ps); err != nil {
				return err
			}

			role, err := cli.api.Role.Read(inputs.ID)
			if err != nil {
				return err
			}

			cli.renderer.RolePermissionRemove(role, rs, inputs.Permissions)
			return nil
		},
	}

	roleAPIIdentifier.RegisterString(cmd, &inputs.APIIdentifier, "")
	roleAPIPermissions.RegisterStringSlice(cmd, &inputs.Permissions, nil)
	return cmd
}

func (c *cli) apiPickerOptionsWithoutAuth0() (pickerOptions, error) {
	ten, err := c.getTenant()
	if err != nil {
		return nil, err
	}

	return c.filteredAPIPickerOptions(func(r *management.ResourceServer) bool {
		u, err := url.Parse(r.GetIdentifier())
		if err != nil {
			// We really should't get an error here, but for
			// correctness it's indeterminate, therefore we return
			// false.
			return false
		}

		// We only allow API Identifiers not matching the tenant
		// domain, similar to the dashboard UX.
		return u.Host != ten.Domain
	})
}

func (c *cli) pickRolePermissions(apiScopes []*management.ResourceServerScope, permissions *[]string) error {
	// NOTE(cyx): We're inlining this for now since we have no generic
	// usecase for this particular picker type yet.
	var options []string
	for _, s := range apiScopes {
		options = append(options, s.GetValue())
	}

	p := &survey.MultiSelect{
		Message: "Permissions",
		Options: options,
	}

	if err := survey.AskOne(p, permissions); err != nil {
		return err
	}

	return nil
}

func makePermissions(id string, permissions []string) []*management.Permission {
	var result []*management.Permission
	for _, p := range permissions {
		result = append(result, &management.Permission{
			ResourceServerIdentifier: auth0.String(id),
			Name:                     auth0.String(p),
		})
	}
	return result
}
