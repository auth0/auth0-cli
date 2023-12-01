package cli

import (
	"errors"
	"fmt"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
)

var (
	roleID = Argument{
		Name: "Role ID",
		Help: "Id of the role.",
	}
	roleName = Flag{
		Name:       "Name",
		LongForm:   "name",
		ShortForm:  "n",
		Help:       "Name of the role.",
		IsRequired: true,
	}
	roleDescription = Flag{
		Name:       "Description",
		LongForm:   "description",
		ShortForm:  "d",
		Help:       "Description of the role.",
		IsRequired: true,
	}
	roleNumber = Flag{
		Name:      "Number",
		LongForm:  "number",
		ShortForm: "n",
		Help:      "Number of roles to retrieve. Minimum 1, maximum 1000.",
	}
)

func rolesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "roles",
		Short: "Manage resources for roles",
		Long: "Manage resources for roles. To learn more about roles and their behavior, read " +
			"[Role-based Access Control](https://auth0.com/docs/manage-users/access-control/rbac).",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listRolesCmd(cli))
	cmd.AddCommand(showRoleCmd(cli))
	cmd.AddCommand(createRoleCmd(cli))
	cmd.AddCommand(updateRoleCmd(cli))
	cmd.AddCommand(deleteRoleCmd(cli))
	cmd.AddCommand(rolePermissionsCmd(cli))

	return cmd
}

func listRolesCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Number int
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your roles",
		Long:    "List your existing roles. To create one, run: `auth0 roles create`.",
		Example: `  auth0 roles list
  auth0 roles ls
  auth0 roles ls --number 100
  auth0 roles ls -n 100 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Number < 1 || inputs.Number > 1000 {
				return fmt.Errorf("number flag invalid, please pass a number between 1 and 1000")
			}

			list, err := getWithPagination(
				inputs.Number,
				func(opts ...management.RequestOption) (result []interface{}, hasNext bool, err error) {
					roleList, err := cli.api.Role.List(cmd.Context(), opts...)
					if err != nil {
						return nil, false, err
					}

					for _, role := range roleList.Roles {
						result = append(result, role)
					}

					return result, roleList.HasNext(), nil
				},
			)
			if err != nil {
				return fmt.Errorf("An unexpected error occurred: %w", err)
			}

			var roles []*management.Role
			for _, item := range list {
				roles = append(roles, item.(*management.Role))
			}

			cli.renderer.RoleList(roles)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	roleNumber.RegisterInt(cmd, &inputs.Number, defaultPageSize)

	return cmd
}

func showRoleCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show a role",
		Long:  "Display information about a role.",
		Example: `  auth0 roles show
  auth0 roles show <role-id>
  auth0 roles show <role-id> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := roleID.Pick(cmd, &inputs.ID, cli.rolePickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			r := &management.Role{ID: &inputs.ID}

			if err := ansi.Waiting(func() error {
				var err error
				r, err = cli.api.Role.Read(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("Unable to load role: %w", err)
			}

			cli.renderer.RoleShow(r)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

	return cmd
}

func createRoleCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Name        string
		Description string
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new role",
		Long: "Create a new role.\n\n" +
			"To create interactively, use `auth0 roles create` with no arguments.\n\n" +
			"To create non-interactively, supply the role name and description through the flags.",
		Example: `  auth0 roles create
  auth0 roles create --name myrole --description "awesome role"
  auth0 roles create -n myrole -d "awesome role" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Prompt for role name
			if err := roleName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			// Prompt for role description
			if err := roleDescription.Ask(cmd, &inputs.Description, nil); err != nil {
				return err
			}

			// Load values into a fresh role instance
			r := &management.Role{
				Name:        &inputs.Name,
				Description: &inputs.Description,
			}

			// Create role
			if err := ansi.Waiting(func() error {
				return cli.api.Role.Create(cmd.Context(), r)
			}); err != nil {
				return fmt.Errorf("Unable to create role: %v", err)
			}

			// Render role creation specific view
			cli.renderer.RoleCreate(r)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	roleName.RegisterString(cmd, &inputs.Name, "")
	roleDescription.RegisterString(cmd, &inputs.Description, "")

	return cmd
}

func updateRoleCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID          string
		Name        string
		Description string
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update a role",
		Long: "Update a role.\n\n" +
			"To update interactively, use `auth0 roles update` with no arguments.\n\n" +
			"To update non-interactively, supply the role id, name and description through the flags.",
		Example: `  auth0 roles update
  auth0 roles update <role-id> --name myrole
  auth0 roles update <role-id> --name myrole --description "awesome role"
  auth0 roles update <role-id> -n myrole -d "awesome role" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := roleID.Pick(cmd, &inputs.ID, cli.rolePickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var currentRole *management.Role
			if err := ansi.Waiting(func() (err error) {
				currentRole, err = cli.api.Role.Read(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to find role with ID %q: %v", inputs.ID, err)
			}

			if err := roleName.AskU(cmd, &inputs.Name, currentRole.Name); err != nil {
				return err
			}

			if err := roleDescription.AskU(cmd, &inputs.Description, currentRole.Description); err != nil {
				return err
			}

			updatedRole := &management.Role{}

			if inputs.Name != "" {
				updatedRole.Name = &inputs.Name
			}
			if inputs.Description != "" {
				updatedRole.Description = &inputs.Description
			}

			if err := ansi.Waiting(func() error {
				return cli.api.Role.Update(cmd.Context(), inputs.ID, updatedRole)
			}); err != nil {
				return fmt.Errorf("failed to update role with ID %q: %v", inputs.ID, err)
			}

			cli.renderer.RoleUpdate(updatedRole)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	roleName.RegisterStringU(cmd, &inputs.Name, "")
	roleDescription.RegisterStringU(cmd, &inputs.Description, "")

	return cmd
}

func deleteRoleCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Args:    cobra.MinimumNArgs(0),
		Short:   "Delete a role",
		Long: "Delete a role.\n\n" +
			"To delete interactively, use `auth0 roles delete`.\n\n" +
			"To delete non-interactively, supply the role id and the `--force` flag to skip confirmation.",
		Example: `  auth0 roles delete
  auth0 roles rm
  auth0 roles delete <role-id>
  auth0 roles delete <role-id> --force
  auth0 roles delete <role-id> <role-id2> <role-idn>
  auth0 roles delete <role-id> <role-id2> <role-idn> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ids := make([]string, len(args))
			if len(args) == 0 {
				err := roleID.PickMany(cmd, &ids, cli.rolePickerOptions)
				if err != nil {
					return err
				}
			} else {
				for _, id := range args {
					ids = append(ids, id)
				}
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			return ansi.Spinner("Deleting Role", func() error {
				var errs []error
				for _, id := range ids {
					if _, err := cli.api.Role.Read(cmd.Context(), id); err != nil {
						errs = append(errs, fmt.Errorf("Unable to read role for deletion: %w", err))
					}

					if err := cli.api.Role.Delete(cmd.Context(), id); err != nil {
						errs = append(errs, fmt.Errorf("Unable to delete role: %w", err))
					}
				}
				return errors.Join(errs...)
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

func (c *cli) rolePickerOptions(ctx context.Context) (pickerOptions, error) {
	list, err := c.api.Role.List(ctx)
	if err != nil {
		return nil, err
	}

	var opts pickerOptions

	for _, c := range list.Roles {
		value := c.GetID()
		label := fmt.Sprintf("%s %s", c.GetName(), ansi.Faint("("+value+")"))
		opts = append(opts, pickerOption{value: value, label: label})
	}

	if len(opts) == 0 {
		return nil, errors.New("There are currently no roles.")
	}

	return opts, nil
}
