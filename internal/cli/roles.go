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

				prompt := &survey.MultiSelect{
					Message: "Choose roles:",
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
			defer close(ch)

			for _, id := range roleIDs {
				go func(id string) {
					role, err := cli.api.Role.Read(id)
					ch <- &result{id: id, role: role, err: err}
				}(id)
			}

			roles := []*management.Role{}
			failed := map[string]error{}

			timer := time.NewTimer(30 * time.Second)
			err := ansi.Spinner("Getting roles", func() error {
				for range roleIDs {
					select {
					case res := <-ch:
						if res.err != nil {
							failed[res.id] = res.err
							continue
						}
						roles = append(roles, res.role)
					case <-timer.C:
						return errors.New("Failed to get roles")
					}
				}
				return nil
			})
			if err != nil {
				return err
			}

			if len(failed) != 0 {
				err := errors.New("Failed to get roles:")
				for k, v := range failed {
					err = fmt.Errorf("%w\n\n      - ROLE ID: %s\n        ERROR: %s", err, k, v)
				}
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
			defer close(ch)

			for _, id := range roleIDs {
				go func(id string) {
					ch <- &result{id: id, err: cli.api.Role.Delete(id)}
				}(id)
			}

			failed := map[string]error{}

			timer := time.NewTimer(30 * time.Second)
			for range roleIDs {
				select {
				case res := <-ch:
					if res.err != nil {
						if res.err != nil {
							failed[res.id] = res.err
						}
					}
				case <-timer.C:
					return errors.New("Failed to delete roles")
				}
			}

			if len(failed) != 0 {
				err := errors.New("Failed to delete roles:")
				for k, v := range failed {
					err = fmt.Errorf("%w\n\n      - ROLE ID: %s\n        ERROR: %s", err, k, v)
				}
				return err
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
					Help:    "ID of the role to update.",
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
	flags := rolePermissionFlags{}
	cmd := &cobra.Command{
		Use:   "remove-permissions",
		Short: "Remove permissions from a role",
		Long: `auth0 roles remove-permissions myRoleID1 myRoleID2 --permission-name "read:resource" --resource-server-identifier "https://api.example.com/role" --permission-name "update:resource" --resource-server-identifier "https://api.example.com/role"
Remove permissions associated with a role.

`,
		RunE: func(cmd *cobra.Command, args []string) error {
			roleIDs := args

			if len(roleIDs) == 0 {
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
					Help:    "ID of the roles to remove permissions from.",
				}
				resp := []string{}
				if err = survey.AskOne(prompt, &resp); err != nil {
					return err
				}

				for _, i := range resp {
					s := strings.Fields(i)
					roleIDs = append(roleIDs, s[0])
				}
			}

			if len(flags.permissionNames) == 0 {
				qs := []*survey.Question{
					{
						Name: "permissionName",
						Prompt: &survey.Input{
							Message: "Permission Name:",
							Help:    "Permission name to remove from roles.",
						},
					},
				}
				if err := survey.Ask(qs, &flags); err != nil {
					return err
				}
			}

			if len(flags.resourceServerIdentifiers) == 0 {
				qs := []*survey.Question{
					{
						Name: "resourceServerIdentifier",
						Prompt: &survey.Input{
							Message: "Resource Server Identifier:",
							Help:    "Resource server identifier to remove from roles.",
						},
					},
				}
				if err := survey.Ask(qs, &flags); err != nil {
					return err
				}
			}

			if len(flags.permissionNames) != len(flags.resourceServerIdentifiers) {
				return errors.New("Permission names dont match resource server identifiers")
			}

			type result struct {
				id  string
				err error
			}

			ch := make(chan *result, auth0.DEFAULT_CHANNEL_BUFFER_LENGTH)
			defer close(ch)

			for _, id := range roleIDs {
				permissions := []*management.Permission{}
				for i, p := range flags.permissionNames {
					resourceServerIdentifier := flags.resourceServerIdentifiers[i]
					permission := &management.Permission{
						Name:                     auth0.String(p),
						ResourceServerIdentifier: auth0.String(resourceServerIdentifier),
					}
					permissions = append(permissions, permission)
				}
				go func(id string, permissions []*management.Permission) {
					ch <- &result{id: id, err: cli.api.Role.RemovePermissions(id, permissions)}
				}(id, permissions)
			}

			failed := map[string]error{}

			timer := time.NewTimer(auth0.DEFAULT_TIMER_DURATION)
			err := ansi.Spinner("Removing permissions from role", func() error {
				for range roleIDs {
					select {
					case res := <-ch:
						if res.err != nil {
							failed[res.id] = res.err
							continue
						}
					case <-timer.C:
						return errors.New("Failed to remove role permissions")
					}
				}
				return nil
			})
			if err != nil {
				return err
			}

			if len(failed) != 0 {
				err := errors.New("Failed to remove role permissions:")
				for k, v := range failed {
					err = fmt.Errorf("%w\n\n      - ROLE ID: %s\n        ERROR: %s", err, k, v)
				}
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&flags.permissionNames, "permission-name", "", []string{}, "Permission name to remove.")
	cmd.Flags().StringSliceVarP(&flags.resourceServerIdentifiers, "resource-server-identifier", "", []string{}, "Resource server identifier to remove.")

	return cmd
}

func rolesGetPermissionsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-permissions",
		Short: "Get permissions granted for roles",
		Long: `auth0 roles get-permissions myRoleID1 myRoleID2
Retrieve list of permissions granted for roles.

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
					Help:    "IDs of the roles to list granted permissions.",
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
				id             string
				permissionList *management.PermissionList
				err            error
			}

			ch := make(chan *result, 5)
			defer close(ch)

			for _, id := range roleIDs {
				go func(id string) {
					permissionList, err := cli.api.Role.Permissions(id)
					ch <- &result{id: id, permissionList: permissionList, err: err}
				}(id)
			}

			rolePermissions := map[string][]*management.Permission{}
			failed := map[string]error{}

			timer := time.NewTimer(30 * time.Second)
			err := ansi.Spinner("Getting role permissions", func() error {
				for range roleIDs {
					select {
					case res := <-ch:
						if res.err != nil {
							failed[res.id] = res.err
							continue
						}
						rolePermissions[res.id] = res.permissionList.Permissions
					case <-timer.C:
						return errors.New("Failed to get role permissions")
					}
				}
				return nil
			})
			if err != nil {
				return err
			}

			if len(failed) != 0 {
				err := errors.New("Failed to get role permissions:")
				for k, v := range failed {
					err = fmt.Errorf("%w\n\n      - ROLE ID: %s\n        ERROR: %s", err, k, v)
				}
				return err
			}

			switch i := len(rolePermissions); {
			case i > 1:
				cli.renderer.RolePermissionsList(rolePermissions)
			default:
				roleID := roleIDs[0]
				rolePermission := rolePermissions[roleID]
				cli.renderer.RolePermissionsGet(roleID, rolePermission)
			}

			return nil
		},
	}

	return cmd
}

func rolesAssociatePermissionsCmd(cli *cli) *cobra.Command {
	flags := rolePermissionFlags{}
	cmd := &cobra.Command{
		Use:   "associate-permissions",
		Short: "Associate permissions with a role",
		Long: `auth0 roles associate-permissions myRoleID1 myRoleID2 --permission-name "read:resource" --resource-server-identifier "https://api.example.com/role" --permission-name "update:resource" --resource-server-identifier "https://api.example.com/role"
Associate permissions with a role.

`,
		RunE: func(cmd *cobra.Command, args []string) error {
			roleIDs := args

			if len(roleIDs) == 0 {
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
					Help:    "ID of the roles to associate permissions with.",
				}
				resp := []string{}
				if err = survey.AskOne(prompt, &resp); err != nil {
					return err
				}

				for _, i := range resp {
					s := strings.Fields(i)
					roleIDs = append(roleIDs, s[0])
				}
			}

			if len(flags.permissionNames) == 0 {
				qs := []*survey.Question{
					{
						Name: "permissionName",
						Prompt: &survey.Input{
							Message: "Permission Name:",
							Help:    "Permission name to associate with roles.",
						},
					},
				}
				if err := survey.Ask(qs, &flags); err != nil {
					return err
				}
			}

			if len(flags.resourceServerIdentifiers) == 0 {
				qs := []*survey.Question{
					{
						Name: "resourceServerIdentifier",
						Prompt: &survey.Input{
							Message: "Resource Server Identifier:",
							Help:    "Resource Server Identifier.",
						},
					},
				}
				if err := survey.Ask(qs, &flags); err != nil {
					return err
				}
			}

			if len(flags.permissionNames) != len(flags.resourceServerIdentifiers) {
				return errors.New("Permission names dont match resource server identifiers")
			}

			type resultPermissions struct {
				permissionList *management.PermissionList
				err            error
			}

			type result struct {
				id          string
				err         error
				permissions resultPermissions
			}

			ch := make(chan *result, auth0.DEFAULT_CHANNEL_BUFFER_LENGTH)
			defer close(ch)

			for _, id := range roleIDs {
				permissions := []*management.Permission{}
				for i, p := range flags.permissionNames {
					resourceServerIdentifier := flags.resourceServerIdentifiers[i]
					permission := &management.Permission{
						Name:                     auth0.String(p),
						ResourceServerIdentifier: auth0.String(resourceServerIdentifier),
					}
					permissions = append(permissions, permission)
				}
				go func(id string, permissions []*management.Permission) {
					res := &result{id: id}
					res.err = cli.api.Role.AssociatePermissions(id, permissions)
					p := resultPermissions{}
					p.permissionList, p.err = cli.api.Role.Permissions(id)
					res.permissions = p
					ch <- res
				}(id, permissions)
			}

			rolePermissions := map[string][]*management.Permission{}
			failed := map[string]error{}

			timer := time.NewTimer(auth0.DEFAULT_TIMER_DURATION)
			err := ansi.Spinner("Associating permissions with roles", func() error {
				for range roleIDs {
					select {
					case res := <-ch:
						if res.err != nil {
							failed[res.id] = res.err
							continue
						}
						p := res.permissions
						if p.err != nil {
							failed[res.id] = p.err
							continue
						}
						rolePermissions[res.id] = p.permissionList.Permissions
					case <-timer.C:
						return errors.New("Failed to associate role permissions")
					}
				}
				return nil
			})
			if err != nil {
				return err
			}

			if len(failed) != 0 {
				err := errors.New("Failed to associate role permissions:")
				for k, v := range failed {
					err = fmt.Errorf("%w\n\n      - ROLE ID: %s\n        ERROR: %s", err, k, v)
				}
				return err
			}

			switch i := len(rolePermissions); {
			case i > 1:
				cli.renderer.RolePermissionsList(rolePermissions)
			default:
				roleID := roleIDs[0]
				rolePermission := rolePermissions[roleID]
				cli.renderer.RolePermissionsGet(roleID, rolePermission)
			}
			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&flags.permissionNames, "permission-name", "", []string{}, "Permission name.")
	cmd.Flags().StringSliceVarP(&flags.resourceServerIdentifiers, "resource-server-identifier", "", []string{}, "Resource server identifier.")

	return cmd
}

type rolePermissionFlags struct {
	permissionNames           []string
	resourceServerIdentifiers []string
}

func (f *rolePermissionFlags) WriteAnswer(name string, value interface{}) error {
	switch name {
	case "permissionName":
		f.permissionNames = append(f.permissionNames, value.(string))
	case "resourceServerIdentifier":
		f.resourceServerIdentifiers = append(f.resourceServerIdentifiers, value.(string))
	default:
		return fmt.Errorf("Unsupported name: %s", name)
	}
	return nil
}
