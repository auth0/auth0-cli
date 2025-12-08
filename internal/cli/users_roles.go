package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
)

var (
	userRolesNumber = Flag{
		Name:      "Number",
		LongForm:  "number",
		ShortForm: "n",
		Help:      "Number of user roles to retrieve. Minimum 1, maximum 1000.",
	}
)

var (
	userRoles = Flag{
		Name:       "Roles",
		LongForm:   "roles",
		ShortForm:  "r",
		Help:       "Roles to assign to a user.",
		IsRequired: true,
	}

	errNoRolesSelected = errors.New("required to select at least one role")
)

type userRolesInput struct {
	ID     string
	Number int
	Roles  []string
}

type userRolesFetcher func(ctx context.Context, cli *cli, userID string) ([]string, error)
type userRolesSelector func(options []string) ([]string, error)

func userRolesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "roles",
		Short: "Manage a user's roles",
		Long: "Manage a user's assigned roles. To learn more about roles and their behavior, read " +
			"[Role-based Access Control](https://auth0.com/docs/manage-users/access-control/rbac).",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(showUserRolesCmd(cli))
	cmd.AddCommand(addUserRolesCmd(cli))
	cmd.AddCommand(removeUserRolesCmd(cli))

	return cmd
}

func showUserRolesCmd(cli *cli) *cobra.Command {
	var inputs userRolesInput

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show a user's roles",
		Long:  "Display information about an existing user's assigned roles.",
		Example: `  auth0 users roles show
  auth0 users roles show <user-id>
  auth0 users roles show <user-id> --number 100
  auth0 users roles show <user-id> -n 100 --json
  auth0 users roles show <user-id> -n 100 --json-compact
  auth0 users roles show <user-id> --csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := userID.Ask(cmd, &inputs.ID); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			if inputs.Number < 1 || inputs.Number > 1000 {
				return fmt.Errorf("number flag invalid, please pass a number between 1 and 1000")
			}

			list, err := getWithPagination(
				inputs.Number,
				func(opts ...management.RequestOption) (result []interface{}, hasNext bool, err error) {
					userRoleList, err := cli.api.User.Roles(cmd.Context(), inputs.ID, opts...)
					if err != nil {
						return nil, false, err
					}

					var output []interface{}
					for _, userRole := range userRoleList.Roles {
						output = append(output, userRole)
					}

					return output, userRoleList.HasNext(), nil
				},
			)
			if err != nil {
				return fmt.Errorf("failed to read roles for user with ID %q: %w", inputs.ID, err)
			}

			var userRoles []*management.Role
			for _, item := range list {
				userRoles = append(userRoles, item.(*management.Role))
			}

			cli.renderer.UserRoleList(userRoles)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	userRolesNumber.RegisterInt(cmd, &inputs.Number, defaultPageSize)

	return cmd
}

func addUserRolesCmd(cli *cli) *cobra.Command {
	var inputs userRolesInput

	cmd := &cobra.Command{
		Use:     "assign",
		Aliases: []string{"add"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "Assign roles to a user",
		Long:    "Assign existing roles to a user.",
		Example: `  auth0 users roles assign <user-id>
  auth0 users roles add <user-id> --roles <role-id1,role-id2>
  auth0 users roles add <user-id> -r "rol_1eKJp3jV04SiU04h,rol_2eKJp3jV04SiU04h" --json
  auth0 users roles add <user-id> -r "rol_1eKJp3jV04SiU04h,rol_2eKJp3jV04SiU04h" --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := userID.Ask(cmd, &inputs.ID); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			if len(inputs.Roles) == 0 {
				if err := cli.getUserRoles(cmd.Context(), &inputs, userRolesToAddPickerOptions, pickUserRoles); err != nil {
					return err
				}
			}

			var rolesToAssign []*management.Role
			for _, roleID := range inputs.Roles {
				rolesToAssign = append(rolesToAssign, &management.Role{
					ID: auth0.String(roleID),
				})
			}

			if err := ansi.Waiting(func() (err error) {
				return cli.api.User.AssignRoles(cmd.Context(), inputs.ID, rolesToAssign)
			}); err != nil {
				return fmt.Errorf("failed to assign roles for user with ID %q: %w", inputs.ID, err)
			}

			var userRoleList *management.RoleList
			if err := ansi.Waiting(func() (err error) {
				userRoleList, err = cli.api.User.Roles(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to read roles for user with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.UserRoleList(userRoleList.Roles)

			return nil
		},
	}

	userRoles.RegisterStringSlice(cmd, &inputs.Roles, nil)
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func removeUserRolesCmd(cli *cli) *cobra.Command {
	var inputs userRolesInput

	cmd := &cobra.Command{
		Use:     "remove",
		Aliases: []string{"rm"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "Remove roles from a user",
		Long:    "Remove existing roles from a user.",
		Example: `  auth0 users roles remove <user-id>
  auth0 users roles remove <user-id> --roles <role-id1,role-id2>
  auth0 users roles rm <user-id> -r "rol_1eKJp3jV04SiU04h,rol_2eKJp3jV04SiU04h" --json
  auth0 users roles rm <user-id> -r "rol_1eKJp3jV04SiU04h,rol_2eKJp3jV04SiU04h" --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := userID.Ask(cmd, &inputs.ID); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			if len(inputs.Roles) == 0 {
				if err := cli.getUserRoles(cmd.Context(), &inputs, userRolesToRemovePickerOptions, pickUserRoles); err != nil {
					return err
				}
			}

			var rolesToRemove []*management.Role
			for _, roleID := range inputs.Roles {
				rolesToRemove = append(rolesToRemove, &management.Role{
					ID: auth0.String(roleID),
				})
			}

			if err := ansi.Waiting(func() (err error) {
				return cli.api.User.RemoveRoles(cmd.Context(), inputs.ID, rolesToRemove)
			}); err != nil {
				return fmt.Errorf("failed to remove roles for user with ID %q: %w", inputs.ID, err)
			}

			var userRoleList *management.RoleList
			if err := ansi.Waiting(func() (err error) {
				userRoleList, err = cli.api.User.Roles(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to read roles for user with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.UserRoleList(userRoleList.Roles)

			return nil
		},
	}

	userRoles.RegisterStringSlice(cmd, &inputs.Roles, nil)
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func (cli *cli) getUserRoles(ctx context.Context, inputs *userRolesInput, fetchUserRoles userRolesFetcher, selectUserRoles userRolesSelector) error {
	var options []string
	if err := ansi.Waiting(func() (err error) {
		options, err = fetchUserRoles(ctx, cli, inputs.ID)
		return err
	}); err != nil {
		return err
	}

	selectedRoles, err := selectUserRoles(options)
	if err != nil {
		return err
	}

	for _, selectedRole := range selectedRoles {
		indexOfFirstEmptySpace := strings.Index(selectedRole, " ")
		inputs.Roles = append(inputs.Roles, selectedRole[:indexOfFirstEmptySpace])
	}

	if len(inputs.Roles) == 0 {
		return errNoRolesSelected
	}

	return err
}

func pickUserRoles(options []string) ([]string, error) {
	rolesPrompt := &survey.MultiSelect{
		Message: "Roles",
		Options: options,
	}

	var selectedRoles []string
	if err := survey.AskOne(rolesPrompt, &selectedRoles); err != nil {
		return nil, err
	}

	return selectedRoles, nil
}

func userRolesToAddPickerOptions(ctx context.Context, cli *cli, userID string) ([]string, error) {
	currentUserRoleList, err := cli.api.User.Roles(ctx, userID, management.PerPage(100))
	if err != nil {
		return nil, fmt.Errorf("failed to read the current roles for user with ID %q: %w", userID, err)
	}

	var roleList *management.RoleList
	roleList, err = cli.api.Role.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list all roles: %w", err)
	}

	if len(roleList.Roles) == len(currentUserRoleList.Roles) {
		return nil, fmt.Errorf("the user with ID %q has all roles assigned already", userID)
	}

	var options []string
	for _, role := range roleList.Roles {
		if !containsRole(currentUserRoleList.Roles, role.GetID()) {
			options = append(options, fmt.Sprintf("%s (Name: %s)", role.GetID(), role.GetName()))
		}
	}

	return options, nil
}

func userRolesToRemovePickerOptions(ctx context.Context, cli *cli, userID string) ([]string, error) {
	currentUserRoleList, err := cli.api.User.Roles(ctx, userID, management.PerPage(100))
	if err != nil {
		return nil, fmt.Errorf("failed to read the current roles for user with ID %q: %w", userID, err)
	}

	var options []string
	for _, role := range currentUserRoleList.Roles {
		options = append(options, fmt.Sprintf("%s (Name: %s)", role.GetID(), role.GetName()))
	}

	return options, nil
}

func containsRole(roles []*management.Role, roleID string) bool {
	for _, role := range roles {
		if role.GetID() == roleID {
			return true
		}
	}
	return false
}
