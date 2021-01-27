package cli

import (
	"errors"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

func usersCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "manage users.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(getusersCmd(cli))

	return cmd
}

func getusersCmd(cli *cli) *cobra.Command {
	var flags struct {
		id     string
		email  string
		fields string
	}

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a user's details",
		Long: `$ auth0 users get
Get a user
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			userID, err := cmd.LocalFlags().GetString("id")
			if err != nil {
				return err
			}

			userEmail, err := cmd.LocalFlags().GetString("email")
			if err != nil {
				return err
			}

			if userID == "" && userEmail == "" {
				return errors.New("User id or email flag must be specified")
			}

			if userID != "" && userEmail != "" {
				return errors.New("User id and email flags cannot be combined")
			}

			var users []*management.User
			var user *management.User

			if userID != "" {
				err := ansi.Spinner("Getting user", func() error {
					var err error
					user, err = cli.api.User.Read(flags.id)
					return err
				})

				if err != nil {
					return err
				}

				users = append(users, user)

				cli.renderer.UserList(users)
				return nil
			}

			if userEmail != "" {
				err := ansi.Spinner("Getting user(s)", func() error {
					var err error
					users, err = cli.api.User.ListByEmail(userEmail)
					return err
				})

				if err != nil {
					return err
				}

				cli.renderer.UserList(users)
				return nil
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.id, "id", "i", "", "User ID of user to get.")
	cmd.Flags().StringVarP(&flags.email, "email", "e", "", "Email of user to get.")

	return cmd
}
