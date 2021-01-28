package cli

import (
	"encoding/json"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

func rolesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "roles",
		Short: "Manage resources for roles",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(rolesListCmd(cli))
	cmd.AddCommand(rolesGetCmd(cli))
	cmd.AddCommand(rolesDeleteCmd(cli))
	cmd.AddCommand(rolesUpdateCmd(cli))
	cmd.AddCommand(rolesCreateCmd(cli))
	cmd.AddCommand(rolesGetPermissionsCmd(cli))
	cmd.AddCommand(rolesAssociatePermissionsCmd(cli))
	cmd.AddCommand(rolesRemovePermissionsCmd(cli))

	return cmd
}

func rolesListCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all roles",
		Long: `auth0 roles list
Retrieve filtered list of roles that can be assigned to users or groups

`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var list *management.RoleList
			err := ansi.Spinner("Getting roles", func() error {
				var err error
				list, err = cli.api.Role.List()
				return err
			})

			if err != nil {
				return err
			}

			cli.renderer.RoleList(list.Roles)
			return nil
		},
	}

	return cmd
}

func rolesGetCmd(cli *cli) *cobra.Command {
	var flags struct {
		RoleID string
	}
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a role",
		Long: `auth0 roles get --role-id myRoleID
Get a role

`,
		RunE: func(cmd *cobra.Command, args []string) error {

			if !cmd.Flags().Changed("role-id") {
				qs := []*survey.Question{
					{
						Name: "RoleID",
						Prompt: &survey.Input{
							Message: "RoleID:",
							Help:    "ID of the role to get.",
						},
					},
				}
				err := survey.Ask(qs, &flags)
				if err != nil {
					return err
				}
			}

			var role *management.Role
			err := ansi.Spinner("Getting role", func() error {
				var err error
				role, err = cli.api.Role.Read(flags.RoleID)
				return err
			})

			if err != nil {
				return err
			}

			cli.renderer.RoleGet(role)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.RoleID, "role-id", "i", "", "ID of the role to get.")

	return cmd
}

func rolesDeleteCmd(cli *cli) *cobra.Command {
	var flags struct {
		RoleID string
	}
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a role",
		Long: `auth0 roles delete --role-id myRoleID
Delete a role.

`,
		RunE: func(cmd *cobra.Command, args []string) error {

			if !cmd.Flags().Changed("role-id") {
				qs := []*survey.Question{
					{
						Name: "RoleID",
						Prompt: &survey.Input{
							Message: "RoleID:",
							Help:    "ID of the role to delete.",
						},
					},
				}
				err := survey.Ask(qs, &flags)
				if err != nil {
					return err
				}
			}

			return ansi.Spinner("Deleting role", func() error {
				return cli.api.Role.Delete(flags.RoleID)
			})
		},
	}

	cmd.Flags().StringVarP(&flags.RoleID, "role-id", "i", "", "ID of the role to delete.")

	return cmd
}

func rolesUpdateCmd(cli *cli) *cobra.Command {
	var flags struct {
		RoleID      string
		Name        string
		Description string
	}
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a role",
		Long: `auth0 roles update --role-id myRoleID --name myName --description myDescription
Update a role.

`,
		RunE: func(cmd *cobra.Command, args []string) error {

			if !cmd.Flags().Changed("role-id") {
				qs := []*survey.Question{
					{
						Name: "RoleID",
						Prompt: &survey.Input{
							Message: "RoleID:",
							Help:    "ID of the role to update.",
						},
					},
				}
				err := survey.Ask(qs, &flags)
				if err != nil {
					return err
				}
			}

			role := &management.Role{}

			if cmd.Flags().Changed("name") {
				role.Name = auth0.String(flags.Name)
			}

			if cmd.Flags().Changed("description") {
				role.Description = auth0.String(flags.Description)
			}

			err := ansi.Spinner("Updating role", func() error {
				return cli.api.Role.Update(flags.RoleID, role)
			})
			if err != nil {
				return err
			}

			cli.renderer.RoleUpdate(role)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.RoleID, "role-id", "i", "", "ID of the role to update.")
	cmd.Flags().StringVarP(&flags.Name, "name", "n", "", "Name of this role.")
	cmd.Flags().StringVarP(&flags.Description, "description", "d", "", "Description of this role.")

	return cmd
}

func rolesCreateCmd(cli *cli) *cobra.Command {
	var flags struct {
		Name        string
		Description string
	}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a role",
		Long: `auth0 roles create --name myName --description myDescription
Create a new role.

`,
		RunE: func(cmd *cobra.Command, args []string) error {

			if !cmd.Flags().Changed("name") {
				qs := []*survey.Question{
					{
						Name: "Name",
						Prompt: &survey.Input{
							Message: "Name:",
							Help:    "Name of the role.",
						},
					},
				}
				err := survey.Ask(qs, &flags)
				if err != nil {
					return err
				}
			}

			if !cmd.Flags().Changed("description") {
				qs := []*survey.Question{
					{
						Name: "Description",
						Prompt: &survey.Input{
							Message: "Description:",
							Help:    "Description of the role.",
						},
					},
				}
				err := survey.Ask(qs, &flags)
				if err != nil {
					return err
				}
			}

			role := &management.Role{
				Name:        auth0.String(flags.Name),
				Description: auth0.String(flags.Description),
			}

			err := ansi.Spinner("Creating role", func() error {
				return cli.api.Role.Create(role)
			})
			if err != nil {
				return err
			}

			cli.renderer.RoleCreate(role)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.Name, "name", "n", "", "Name of the role.")
	cmd.Flags().StringVarP(&flags.Description, "description", "d", "", "Description of the role.")

	return cmd
}

func rolesGetPermissionsCmd(cli *cli) *cobra.Command {
	var flags struct {
		RoleID        string
		PerPage       int
		Page          int
		IncludeTotals bool
	}
	cmd := &cobra.Command{
		Use:   "get-permissions",
		Short: "Get permissions granted by role",
		Long: `auth0 roles get-permissions --role-id myRoleID
Retrieve list of permissions granted by a role.

`,
		RunE: func(cmd *cobra.Command, args []string) error {

			if !cmd.Flags().Changed("role-id") {
				qs := []*survey.Question{
					{
						Name: "RoleID",
						Prompt: &survey.Input{
							Message: "RoleID:",
							Help:    "ID of the role to list granted permissions.",
						},
					},
				}
				err := survey.Ask(qs, &flags)
				if err != nil {
					return err
				}
			}

			opts := []management.RequestOption{}
			if cmd.Flags().Changed("per-page") {
				opts = append(opts, management.Page(flags.PerPage))
			}

			if cmd.Flags().Changed("page") {
				opts = append(opts, management.Page(flags.Page))
			}

			if cmd.Flags().Changed("include-totals") {
				opts = append(opts, management.IncludeTotals(flags.IncludeTotals))
			}

			var permissionList *management.PermissionList
			err := ansi.Spinner("Getting permissions granted by role", func() error {
				var err error
				permissionList, err = cli.api.Role.Permissions(flags.RoleID, opts...)
				return err
			})

			if err != nil {
				return err
			}

			cli.renderer.RoleGetPermissions(permissionList.Permissions)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.RoleID, "role-id", "i", "", "ID of the role to list granted permissions.")
	cmd.Flags().IntVarP(&flags.PerPage, "per-page", "", 50, "Number of results per page. Defaults to 50.")
	cmd.Flags().IntVarP(&flags.Page, "page", "", 0, "Page index of the results to return. First page is 0.")
	cmd.Flags().BoolVarP(&flags.IncludeTotals, "include-totals", "", false, "Return results inside an object that contains the total result count (true) or as a direct array of results (false, default).")

	return cmd
}

func rolesAssociatePermissionsCmd(cli *cli) *cobra.Command {
	var flags struct {
		RoleID      string
		Permissions string
	}
	cmd := &cobra.Command{
		Use:   "associate-permissions",
		Short: "Associate permissions with a role",
		Long: `auth0 roles associate-permissions --role-id myRoleID --permissions '[{"permission_name": "read:resource", "resource_server_identifier": "https://api.example.com/role"}]'
Associate permissions with a role.

`,
		RunE: func(cmd *cobra.Command, args []string) error {

			if !cmd.Flags().Changed("role-id") {
				qs := []*survey.Question{
					{
						Name: "RoleID",
						Prompt: &survey.Input{
							Message: "RoleID:",
							Help:    "ID of the role to list granted permissions.",
						},
					},
				}
				err := survey.Ask(qs, &flags)
				if err != nil {
					return err
				}
			}

			if !cmd.Flags().Changed("permissions") {
				qs := []*survey.Question{
					{
						Name: "Permissions",
						Prompt: &survey.Input{
							Message: "Permissions:",
							Help:    "Array of resource_server_identifier, permission_name pairs.",
						},
					},
				}
				err := survey.Ask(qs, &flags)
				if err != nil {
					return err
				}
			}

			var permissions []*management.Permission
			if err := json.Unmarshal([]byte(flags.Permissions), &permissions); err != nil {
				return fmt.Errorf("Failed to parse permissions string: %s", flags.Permissions)
			}

			err := ansi.Spinner("Associating permissions with role", func() error {
				return cli.api.Role.AssociatePermissions(flags.RoleID, permissions)
			})
			if err != nil {
				return err
			}

			var permissionList *management.PermissionList
			permissionList, err = cli.api.Role.Permissions(flags.RoleID)
			if err != nil {
				return err
			}

			cli.renderer.RoleGetPermissions(permissionList.Permissions)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.RoleID, "role-id", "i", "", "ID of the role to list granted permissions.")
	cmd.Flags().StringVarP(&flags.Permissions, "permissions", "p", "", "array of resource_server_identifier, permission_name pairs.")

	return cmd
}

func rolesRemovePermissionsCmd(cli *cli) *cobra.Command {
	var flags struct {
		RoleID      string
		Permissions string
	}
	cmd := &cobra.Command{
		Use:   "remove-permissions",
		Short: "Remove permissions from a role",
		Long: `auth0 roles remove-permissions --role-id myRoleID --permissions '[{"permission_name": "read:resource", "resource_server_identifier": "https://api.example.com/role"}]'
Remove permissions associated with a role.

`,
		RunE: func(cmd *cobra.Command, args []string) error {

			if !cmd.Flags().Changed("role-id") {
				qs := []*survey.Question{
					{
						Name: "RoleID",
						Prompt: &survey.Input{
							Message: "RoleID:",
							Help:    "ID of the role to remove permissions from.",
						},
					},
				}
				err := survey.Ask(qs, &flags)
				if err != nil {
					return err
				}
			}

			if !cmd.Flags().Changed("permissions") {
				qs := []*survey.Question{
					{
						Name: "Permissions",
						Prompt: &survey.Input{
							Message: "Permissions:",
							Help:    "Array of resource_server_identifier, permission_name pairs.",
						},
					},
				}
				err := survey.Ask(qs, &flags)
				if err != nil {
					return err
				}
			}

			var permissions []*management.Permission
			if err := json.Unmarshal([]byte(flags.Permissions), &permissions); err != nil {
				return fmt.Errorf("Failed to parse permissions string: %s", flags.Permissions)
			}

			err := ansi.Spinner("Removing permissions from role", func() error {
				return cli.api.Role.RemovePermissions(flags.RoleID, permissions)
			})
			if err != nil {
				return err
			}

			var permissionList *management.PermissionList
			permissionList, err = cli.api.Role.Permissions(flags.RoleID)
			if err != nil {
				return err
			}

			cli.renderer.RoleGetPermissions(permissionList.Permissions)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.RoleID, "role-id", "i", "", "ID of the role to list granted permissions.")
	cmd.Flags().StringVarP(&flags.Permissions, "permissions", "p", "", "array of resource_server_identifier, permission_name pairs.")

	return cmd
}
