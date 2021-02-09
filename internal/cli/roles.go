package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

type roleFlags struct {
	roleID                    string
	roleIDs                   []string
	permissionNames           []string
	resourceServerIdentifiers []string
}

func (f *roleFlags) WriteAnswer(name string, value interface{}) error {
	switch name {
	case "roleID":
		f.roleID = value.(string)
	case "roleIDs":
		f.roleIDs = append(f.roleIDs, value.(string))
	case "permissionName":
		f.permissionNames = append(f.permissionNames, value.(string))
	case "resourceServerIdentifier":
		f.resourceServerIdentifiers = append(f.resourceServerIdentifiers, value.(string))
	default:
		return fmt.Errorf("Unsupported name: %s", name)
	}
	return nil
}

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
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get roles",
		Long: `auth0 roles get myRoleID1 myRoleID2
Get one or more roles.

`,
		RunE: func(cmd *cobra.Command, args []string) error {
			roleIDs := args

			if len(roleIDs) == 0 {
				resp := []string{}
				opts, err := auth0.GetRoles(cli.api.Role)
				if err != nil {
					return err
				}
				if opts == nil {
					return errors.New("No roles found.")
				}

				prompt := &survey.Select{
					Message: "Choose a role:",
					Options: opts,
					Help:    "IDs of the roles to get.",
				}
				if err = survey.AskOne(prompt, &resp); err != nil {
					return err
				}

				for _, i := range resp {
					s := strings.Fields(i)
					roleIDs = append(roleIDs, s[0])
				}
			}

			type result struct {
				id   string
				role *management.Role
				err  error
			}

			ch := make(chan *result, 5)
			timer := time.NewTimer(30 * time.Second)

			for _, id := range roleIDs {
				go func() {
					role, err := cli.api.Role.Read(id)
					ch <- &result{id: id, role: role, err: err}
				}()
			}
			close(ch)

			roles := []*management.Role{}

			err := ansi.Spinner("Getting roles", func() error {
				for i := 1; i <= len(roleIDs); i++ {
					select {
					case res := <-ch:
						if res.err != nil {
							timer.Stop()
							return fmt.Errorf("Failed to get role: %s, %s", res.id, res.err)
						}
						roles = append(roles, res.role)
					case <-timer.C:
						return errors.New("failed to get roles")
					}
				}
				return nil
			})
			if err != nil {
				return err
			}

			switch i := len(roles); {
			case i > 1:
				cli.renderer.RoleList(roles)
			default:
				cli.renderer.RoleGet(roles[0])
			}
			return nil
		},
	}

	return cmd
}

func rolesDeleteCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete roles",
		Long: `auth0 roles delete myRoleID1 myRoleID2
Delete one or more roles.

`,
		RunE: func(cmd *cobra.Command, args []string) error {

			roleIDs := args

			if len(roleIDs) == 0 {
				resp := []string{}
				opts, err := auth0.GetRoles(cli.api.Role)
				if err != nil {
					return err
				}
				if opts == nil {
					return errors.New("No roles found.")
				}

				prompt := &survey.MultiSelect{
					Message: "Choose roles:",
					Options: opts,
					Help:    "IDs of the roles to delete.",
				}
				if err = survey.AskOne(prompt, &resp); err != nil {
					return err
				}

				for _, i := range resp {
					s := strings.Fields(i)
					roleIDs = append(roleIDs, s[0])
				}
			}

			type result struct {
				id  string
				err error
			}

			ch := make(chan *result, 5)
			timer := time.NewTimer(30 * time.Second)

			for _, id := range roleIDs {
				go func() {
					ch <- &result{id: id, err: cli.api.Role.Delete(id)}
				}()
			}
			close(ch)

			for i := 1; i <= len(roleIDs); i++ {
				select {
				case res := <-ch:
					if res.err != nil {
						timer.Stop()
						return fmt.Errorf("Failed to delete role: %s, %s", res.id, res.err)
					}
				case <-timer.C:
					return errors.New("Failed to delete roles")
				}
			}
			return nil
		},
	}

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
				roleIDs, err := auth0.GetRoles(cli.api.Role)
				if err != nil {
					return err
				}
				if roleIDs == nil {
					return errors.New("No roles found.")
				}

				prompt := &survey.Select{
					Message: "Choose a role:",
					Options: roleIDs,
					Filter: func(filter, opt string, _ int) bool {
						return strings.Contains(opt, filter)
					},
					Help: "ID of the role to update.",
				}
				if err = survey.AskOne(prompt, &flags); err != nil {
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
						Name: "name",
						Prompt: &survey.Input{
							Message: "Name:",
							Help:    "Name of the role.",
						},
					},
				}
				if err := survey.Ask(qs, &flags); err != nil {
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

func rolesRemovePermissionsCmd(cli *cli) *cobra.Command {
	flags := roleFlags{}

	cmd := &cobra.Command{
		Use:   "remove-permissions",
		Short: "Remove permissions from a role",
		Long: `auth0 roles remove-permissions --role-id myRoleID --permission-name "read:resource" --resource-server-identifier "https://api.example.com/role" --permission-name "update:resource" --resource-server-identifier "https://api.example.com/role"
Remove permissions associated with a role.

`,
		RunE: func(cmd *cobra.Command, args []string) error {

			if !cmd.Flags().Changed("role-id") {
				roleIDs, err := auth0.GetRoles(cli.api.Role)
				if err != nil {
					return err
				}
				if roleIDs == nil {
					return errors.New("No roles found.")
				}

				prompt := &survey.Select{
					Message: "Choose a role:",
					Options: roleIDs,
					Filter: func(filter, opt string, _ int) bool {
						return strings.Contains(opt, filter)
					},
					Help: "ID of the role to remove permissions from.",
				}
				if err = survey.AskOne(prompt, &flags); err != nil {
					return err
				}
			}

			if !cmd.Flags().Changed("permission-name") {
				qs := []*survey.Question{
					{
						Name: "permissionName",
						Prompt: &survey.Input{
							Message: "Permission Name:",
							Help:    "Permission name to remove.",
						},
					},
				}
				if err := survey.Ask(qs, &flags); err != nil {
					return err
				}
			}

			if !cmd.Flags().Changed("resource-server-identifier") {
				qs := []*survey.Question{
					{
						Name: "resourceServerIdentifier",
						Prompt: &survey.Input{
							Message: "Resource Server Identifier:",
							Help:    "Resource server identifier to remove.",
						},
					},
				}
				err := survey.Ask(qs, &flags)
				if err != nil {
					return err
				}
			}

			permissions := []*management.Permission{}
			for i, p := range flags.permissionNames {
				resourceServerIdentifier := flags.resourceServerIdentifiers[i]
				permission := &management.Permission{
					Name:                     auth0.String(p),
					ResourceServerIdentifier: auth0.String(resourceServerIdentifier),
				}
				permissions = append(permissions, permission)
			}

			err := ansi.Spinner("Removing permissions from role", func() error {
				return cli.api.Role.RemovePermissions(flags.roleID, permissions)
			})
			if err != nil {
				return err
			}

			var permissionList *management.PermissionList
			permissionList, err = cli.api.Role.Permissions(flags.roleID)
			if err != nil {
				return err
			}

			cli.renderer.RoleGetPermissions(flags.roleID, permissionList.Permissions)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.roleID, "role-id", "i", "", "ID of the role to remove permissions from")
	cmd.Flags().StringSliceVarP(&flags.permissionNames, "permission-name", "", []string{}, "Permission name to remove.")
	cmd.Flags().StringSliceVarP(&flags.resourceServerIdentifiers, "resource-server-identifier", "", []string{}, "Resource server identifier to remove.")

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
				roleIDs, err := auth0.GetRoles(cli.api.Role)
				if err != nil {
					return err
				}
				if roleIDs == nil {
					return errors.New("No roles found.")
				}

				err = survey.AskOne(
					&survey.Select{
						Message: "Choose a role:",
						Options: roleIDs,
						Filter: func(filter, opt string, _ int) bool {
							return strings.Contains(opt, filter)
						},
						Help: "ID of the role to list granted permissions.",
					},
					&flags.RoleID)
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

			cli.renderer.RoleGetPermissions(flags.RoleID, permissionList.Permissions)
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
	flags := roleFlags{}

	cmd := &cobra.Command{
		Use:   "associate-permissions",
		Short: "Associate permissions with a role",
		Long: `auth0 roles associate-permissions --role-id myRoleID --permission-name "read:resource" --resource-server-identifier "https://api.example.com/role" --permission-name "update:resource" --resource-server-identifier "https://api.example.com/role"
Associate permissions with a role.

`,
		RunE: func(cmd *cobra.Command, args []string) error {

			if !cmd.Flags().Changed("role-id") {
				roleIDs, err := auth0.GetRoles(cli.api.Role)
				if err != nil {
					return err
				}
				if roleIDs == nil {
					return errors.New("No roles found.")
				}

				prompt := &survey.Select{
					Message: "Choose a role:",
					Options: roleIDs,
					Filter: func(filter, opt string, _ int) bool {
						return strings.Contains(opt, filter)
					},
					Help: "ID of the role to add permissions to.",
				}
				if err = survey.AskOne(prompt, &flags); err != nil {
					return err
				}
			}

			if !cmd.Flags().Changed("permission-name") {
				qs := []*survey.Question{
					{
						Name: "permissionName",
						Prompt: &survey.Input{
							Message: "Permission Name:",
							Help:    "Permission name.",
						},
					},
				}
				if err := survey.Ask(qs, &flags); err != nil {
					return err
				}
			}

			if !cmd.Flags().Changed("resource-server-identifier") {
				qs := []*survey.Question{
					{
						Name: "resourceServerIdentifier",
						Prompt: &survey.Input{
							Message: "Resource Server Identifier:",
							Help:    "Resource Server Identifier.",
						},
					},
				}
				err := survey.Ask(qs, &flags)
				if err != nil {
					return err
				}
			}

			if len(flags.permissionNames) != len(flags.resourceServerIdentifiers) {
				return errors.New("Permission names dont match resource server identifiers")
			}

			permissions := []*management.Permission{}
			for i, p := range flags.permissionNames {
				resourceServerIdentifier := flags.resourceServerIdentifiers[i]
				permission := &management.Permission{
					Name:                     auth0.String(p),
					ResourceServerIdentifier: auth0.String(resourceServerIdentifier),
				}
				permissions = append(permissions, permission)
			}

			err := ansi.Spinner("Associating permissions with role", func() error {
				return cli.api.Role.AssociatePermissions(flags.roleID, permissions)
			})
			if err != nil {
				return err
			}

			var permissionList *management.PermissionList
			permissionList, err = cli.api.Role.Permissions(flags.roleID)
			if err != nil {
				return err
			}

			cli.renderer.RoleGetPermissions(flags.roleID, permissionList.Permissions)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.roleID, "role-id", "i", "", "ID of the role to list granted permissions.")
	cmd.Flags().StringSliceVarP(&flags.permissionNames, "permission-name", "", []string{}, "Permission name.")
	cmd.Flags().StringSliceVarP(&flags.resourceServerIdentifiers, "resource-server-identifier", "", []string{}, "Resource server identifier.")

	return cmd
}
