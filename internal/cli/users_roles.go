package cli

import (
	"fmt"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"
)

var (
	userRolesNumber = Flag{
		Name:      "Number",
		LongForm:  "number",
		ShortForm: "n",
		Help:      "Number of user roles to retrieve. Minimum 1, maximum 1000.",
	}
)

func userRolesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "roles",
		Short: "Manage a user's roles",
		Long: "Manage a user's assigned roles. To learn more about roles and their behavior, read " +
			"[Role-based Access Control](https://auth0.com/docs/manage-users/access-control/rbac).",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(showUserRolesCmd(cli))

	return cmd
}

func showUserRolesCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID     string
		Number int
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show a user's roles",
		Long:  "Display information about an existing user's assigned roles.",
		Example: `  auth0 users roles show
  auth0 users roles show <user-id>
  auth0 users roles show <user-id> --number 100
  auth0 users roles show <user-id> -n 100 --json`,
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
				cmd.Context(),
				inputs.Number,
				func(opts ...management.RequestOption) (result []interface{}, hasNext bool, err error) {
					userRoleList, err := cli.api.User.Roles(inputs.ID, opts...)
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
				return fmt.Errorf("failed to find roles for user with ID %s: %w", inputs.ID, err)
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
	userRolesNumber.RegisterInt(cmd, &inputs.Number, defaultPageSize)

	return cmd
}
