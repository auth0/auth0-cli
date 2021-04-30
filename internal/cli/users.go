package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

const (
	connectionTypeUPA = "Username-Password-Authentication"
)
var (
	userID = Argument{
		Name: "User ID",
		Help: "Id of the user.",
	}
	userConnection = Flag{
		Name:       "Connection",
		LongForm:   "connection",
		ShortForm:  "c",
		Help:       "Name of the connection this user should be created in.",
		IsRequired: true,
		AlwaysPrompt: false,
	}
	userEmail = Flag{
		Name:       "Email",
		LongForm:   "email",
		ShortForm:  "e",
		Help:       "The user's email.",
		IsRequired: true,
		AlwaysPrompt: false,
	}
	userPassword = Flag{
		Name:       "Password",
		LongForm:   "password",
		ShortForm:  "p",
		Help:       "Initial password for this user (mandatory for non-SMS connections)",
		IsRequired: true,
		AlwaysPrompt: false,
	}
	userUsername = Flag{
		Name:       "Username",
		LongForm:   "username",
		ShortForm:  "u",
		Help:       "The user's username. Only valid if the connection requires a username.",
		IsRequired: true,
		AlwaysPrompt: false,
	}
	userName = Flag{
		Name:       "Name",
		LongForm:   "name",
		ShortForm:  "n",
		Help:       "The user's full name.",
		IsRequired: true,
		AlwaysPrompt: false,
	}
)

func usersCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "Manage resources for users",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(userBlocksCmd(cli))
	cmd.AddCommand(deleteUserBlocksCmd(cli))
	cmd.AddCommand(createUserCmd(cli))
	cmd.AddCommand(listUserCmd(cli))
	cmd.AddCommand(showUserCmd(cli))
	return cmd
}

func userBlocksCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blocks",
		Short: "Manage brute-force protection user blocks.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listUserBlocksCmd(cli))
	return cmd
}

func listUserBlocksCmd(cli *cli) *cobra.Command {
	var inputs struct {
		userID string
	}

	cmd := &cobra.Command{
		Use:   "list",
		Args:  cobra.MaximumNArgs(1),
		Short: "List brute-force protection blocks for a given user",
		Long: `List brute-force protection blocks for a given user:

auth0 users blocks list <user-id>
`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := userID.Ask(cmd, &inputs.userID); err != nil {
					return err
				}
			} else {
				inputs.userID = args[0]
			}

			var userBlocks []*management.UserBlock

			err := ansi.Waiting(func() error {
				var err error
				userBlocks, err = cli.api.User.Blocks(inputs.userID)
				return err
			})

			if err != nil {
				return fmt.Errorf("Unable to load user blocks %v, error: %w", inputs.userID, err)
			}

			cli.renderer.UserBlocksList(userBlocks)
			return nil
		},
	}

	return cmd
}

func deleteUserBlocksCmd(cli *cli) *cobra.Command {
	var inputs struct {
		userID string
	}

	cmd := &cobra.Command{
		Use:   "unblock",
		Args:  cobra.MaximumNArgs(1),
		Short: "Delete brute-force protection blocks for a given user",
		Long: `Delete brute-force protection blocks for a given user:

auth0 users unblock <user-id>
`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := userID.Ask(cmd, &inputs.userID); err != nil {
					return err
				}
			} else {
				inputs.userID = args[0]
			}

			err := ansi.Spinner("Deleting blocks for user...", func() error {
				return cli.api.User.Unblock(inputs.userID)
			})

			if err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}

func createUserCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Connection string
		Email		string
		Password	string
		Username	string
		Name		string
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new user",
		Long:  "Create a new user.",
		Example: `auth0 users create 
auth0 users create --name myapp 
auth0 users create -n myapp --type [native|spa|regular|m2m]
auth0 users create -n myapp -t [native|spa|regular|m2m] -- description <description>`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Prompt for app name
			if err := userName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			//if err := userUsername.Ask(cmd, &inputs.Username, nil); err != nil {
			//	return err
			//}

			if err := userEmail.Ask(cmd, &inputs.Email, nil); err != nil {
				return err
			}

			if err := userPassword.Ask(cmd, &inputs.Password, nil); err != nil {
				return err
			}

			if err := userConnection.Ask(cmd, &inputs.Connection, nil); err != nil {
				return err
			}

			a := &management.User{

				Connection:    &inputs.Connection,
				Email:         &inputs.Email,
				Name:          &inputs.Name,
				//Username:      &inputs.Username,
				Password:      &inputs.Password,
			}

			// Create app
			if err := ansi.Waiting(func() error {
				return cli.api.User.Create(a)
			}); err != nil {
				return fmt.Errorf("Unable to create user: %w", err)
			}

			// Render Result
			cli.renderer.UserCreate(a)

			return nil
		},
	}
	userName.RegisterString(cmd, &inputs.Name, "")
	userConnection.RegisterString(cmd, &inputs.Connection, "")
	userPassword.RegisterString(cmd, &inputs.Password, "")
	userEmail.RegisterString(cmd, &inputs.Email, "")
	//userUsername.RegisterString(cmd, &inputs.Username, "")

	return cmd
}

func listUserCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your users",
		Long: `List your existing users. To create one try:
auth0 users create`,
		Example: `auth0 users list
auth0 users ls`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var list *management.UserList

			if err := ansi.Waiting(func() error {
				var err error
				list, err = cli.api.User.List(management.ExcludeFields(exludedFields...))
				return err
			}); err != nil {
				return fmt.Errorf("An unexpected error occurred: %w", err)
			}

			cli.renderer.UserList(list.Users)
			return nil
		},
	}

	return cmd
}

func showUserCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show an existing user",
		Long:  "Show an existing user.",
		Example: `auth0 users show 
auth0 users show <id>`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := userID.Pick(cmd, &inputs.ID, cli.userPickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			a := &management.User{ID: &inputs.ID}

			if err := ansi.Waiting(func() error {
				var err error
				a, err = cli.api.User.Read(inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("Unable to load users. The Id %v specified doesn't exist", inputs.ID)
			}

			cli.renderer.UserShow(a)
			return nil
		},
	}

	return cmd
}

func userTypeFor(v string) string {
	switch strings.ToLower(v) {
	case "upa", "Username-Password-Authentication":
		return connectionTypeUPA
	default:
		return v
	}
}

func (c *cli) userPickerOptions() (pickerOptions, error) {
	list, err := c.api.User.List()
	if err != nil {
		return nil, err
	}


	var opts pickerOptions
	for _, r := range list.Users {
		label := fmt.Sprintf("%s %s", r.GetName(), ansi.Faint("("+r.GetID()+")"))

		opts = append(opts, pickerOption{value: r.GetID(), label: label})
	}

	if len(opts) == 0 {
		return nil, errors.New("There are currently no users.")
	}

	return opts, nil
}