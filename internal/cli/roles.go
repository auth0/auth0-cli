package cli

import (
	"errors"
	"fmt"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

// errNoRoles signifies no roles exist in a tenant
var errNoRoles = errors.New("there are currently no roles")

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
		Name:      "Description",
		LongForm:  "description",
		ShortForm: "d",
		Help:      "Description of the role.",
		// IsRequired: true,
	}
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
	cmd.AddCommand(showRoleCmd(cli))
	cmd.AddCommand(createRoleCmd(cli))
	cmd.AddCommand(updateRoleCmd(cli))
	cmd.AddCommand(deleteRoleCmd(cli))

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

func showRoleCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show a role",
		Long:  "Show a role.",
		Example: `auth0 roles show
auth0 roles show <id>`,
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
				r, err = cli.api.Role.Read(inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("Unable to load role: %w", err)
			}

			cli.renderer.RoleShow(r)
			return nil
		},
	}

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
		Long:  "Create a new role.",
		Example: `auth0 roles create
auth0 roles create --name myrole
auth0 roles create -n myrole --description "awesome role"`,
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
				return cli.api.Role.Create(r)
			}); err != nil {
				return fmt.Errorf("Unable to create role: %v", err)
			}

			// Render role creation specific view
			cli.renderer.RoleCreate(r)
			return nil
		},
	}

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
		Long:  "Update a role.",
		Example: `auth0 roles update
auth0 roles update <id> --name myrole
auth0 roles update <id> -n myrole --description "awesome role"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := roleID.Pick(cmd, &inputs.ID, cli.rolePickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			// Prompt for role name
			if err := roleName.AskU(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			// Prompt for role description
			if err := roleDescription.AskU(cmd, &inputs.Description, nil); err != nil {
				return err
			}

			// Start with an empty role object. We'll conditionally
			// hydrate it based on the provided parameters since
			// we'll do PATCH semantics.
			r := &management.Role{}

			if inputs.Name != "" {
				r.Name = &inputs.Name
			}

			if inputs.Description != "" {
				r.Description = &inputs.Description
			}

			// Update role
			if err := ansi.Waiting(func() error {
				return cli.api.Role.Update(inputs.ID, r)
			}); err != nil {
				return fmt.Errorf("Unable to update role: %v", err)
			}

			// Render role creation specific view
			cli.renderer.RoleUpdate(r)
			return nil
		},
	}

	roleName.RegisterStringU(cmd, &inputs.Name, "")
	roleDescription.RegisterStringU(cmd, &inputs.Description, "")

	return cmd
}

func deleteRoleCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "delete",
		Args:  cobra.MaximumNArgs(1),
		Short: "Delete a role",
		Long:  "Delete a role.",
		Example: `auth0 roles delete
auth0 roles delete <id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := roleID.Pick(cmd, &inputs.ID, cli.rolePickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			return ansi.Spinner("Deleting Role", func() error {
				_, err := cli.api.Role.Read(inputs.ID)

				if err != nil {
					return fmt.Errorf("Unable to delete role: %w", err)
				}

				return cli.api.Role.Delete(inputs.ID)
			})
		},
	}

	return cmd
}

func (c *cli) rolePickerOptions() (pickerOptions, error) {
	list, err := c.api.Role.List()
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
		return nil, errNoRoles
	}

	return opts, nil
}
