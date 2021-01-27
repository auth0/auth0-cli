package cli

import (
	"errors"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

const (
	userID    = "id"
	userEmail = "email"
)

func usersCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "manage users.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(showUserCmd(cli))

	return cmd
}

func showUserCmd(cli *cli) *cobra.Command {
	var flags struct {
		ID     string
		Email  string
		Fields string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show a user's details",
		Long: `$ auth0 users show --id id --email email
Get a user
`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if shouldPrompt(cmd, userID) && flags.Email == "" {
				input := prompt.TextInput(userID, "Id:", "ID of the user to show.", false)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			if shouldPrompt(cmd, userEmail) && flags.ID == "" {
				input := prompt.TextInput(userEmail, "Email:", "Email of the user to show.", false)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			if flags.ID == "" && flags.Email == "" {
				return errors.New("User id or email flag must be specified")
			}

			if flags.ID != "" && flags.Email != "" {
				return errors.New("User id and email flags cannot be combined")
			}

			var users []*management.User
			var user *management.User

			if flags.ID != "" {
				err := ansi.Spinner("Getting user", func() error {
					var err error
					user, err = cli.api.User.Read(flags.ID)
					return err
				})

				if err != nil {
					return err
				}

				users = append(users, user)

				cli.renderer.UserList(users)
				return nil
			}

			if flags.Email != "" {
				err := ansi.Spinner("Getting user(s)", func() error {
					var err error
					users, err = cli.api.User.ListByEmail(flags.Email)
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

	cmd.Flags().StringVarP(&flags.ID, userID, "i", "", "ID of the user to show.")
	cmd.Flags().StringVarP(&flags.Email, userEmail, "e", "", "Email of the user to show.")

	return cmd
}
