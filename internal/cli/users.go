package cli

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
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
	}
	userEmail = Flag{
		Name:       "Email",
		LongForm:   "email",
		ShortForm:  "e",
		Help:       "The user's email.",
		IsRequired: true,
	}
	userPassword = Flag{
		Name:       "Password",
		LongForm:   "password",
		ShortForm:  "p",
		Help:       "Initial password for this user (mandatory for non-SMS connections).",
		IsRequired: true,
	}
	userUsername = Flag{
		Name:      "Username",
		LongForm:  "username",
		ShortForm: "u",
		Help:      "The user's username. Only valid if the connection requires a username.",
	}
	userName = Flag{
		Name:         "Name",
		LongForm:     "name",
		ShortForm:    "n",
		Help:         "The user's full name.",
		IsRequired:   true,
		AlwaysPrompt: true,
	}
	userQuery = Flag{
		Name:       "Query",
		LongForm:   "query",
		ShortForm:  "q",
		Help:       "Query in Lucene query syntax. See https://auth0.com/docs/users/user-search/user-search-query-syntax for more details.",
		IsRequired: true,
	}
	userSort = Flag{
		Name:      "Sort",
		LongForm:  "sort",
		ShortForm: "s",
		Help:      "Field to sort by. Use 'field:order' where 'order' is '1' for ascending and '-1' for descending. e.g. 'created_at:1'.",
	}
)

func usersCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "Manage resources for users",
		Long: "Manage resources for users.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(searchUsersCmd(cli))
	cmd.AddCommand(createUserCmd(cli))
	cmd.AddCommand(showUserCmd(cli))
	cmd.AddCommand(deleteUserCmd(cli))
	cmd.AddCommand(updateUserCmd(cli))
	cmd.AddCommand(openUserCmd(cli))
	cmd.AddCommand(userBlocksCmd(cli))
	cmd.AddCommand(deleteUserBlocksCmd(cli))

	return cmd
}

func searchUsersCmd(cli *cli) *cobra.Command {
	var inputs struct {
		query string
		sort  string
	}

	cmd := &cobra.Command{
		Use:   "search",
		Args:  cobra.NoArgs,
		Short: "Search for users",
		Long: `Search for users. To create one try:
auth0 users create`,
		Example: `auth0 users search
auth0 users search --query id
auth0 users search -q name --sort "name:1"
auth0 users search -q name -s "name:1"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := userQuery.Ask(cmd, &inputs.query, nil); err != nil {
				return err
			}

			search := &management.UserList{}

			var queryParams []management.RequestOption

			if len(inputs.sort) == 0 {
				queryParams = append(queryParams, management.Query(auth0.StringValue(&inputs.query)))
			} else {
				queryParams = append(queryParams,
					management.Query(auth0.StringValue(&inputs.query)),
					management.Parameter("sort", auth0.StringValue(&inputs.sort)),
				)
			}

			if err := ansi.Waiting(func() error {
				var err error
				search, err = cli.api.User.Search(queryParams...)
				return err
			}); err != nil {
				return fmt.Errorf("An unexpected error occurred: %w", err)
			}

			cli.renderer.UserSearch(search.Users)
			return nil
		},
	}

	userQuery.RegisterString(cmd, &inputs.query, "")
	userSort.RegisterString(cmd, &inputs.sort, "")

	return cmd
}

func createUserCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Connection string
		Email      string
		Password   string
		Username   string
		Name       string
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new user",
		Long:  "Create a new user.",
		Example: `auth0 users create 
auth0 users create --name "John Doe" 
auth0 users create -n "John Doe" --email john@example.com
auth0 users create -n "John Doe" --e john@example.com --connection "Username-Password-Authentication"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Select from the available connection types
			// Users API currently support  database connections
			if err := userConnection.Select(cmd, &inputs.Connection, cli.connectionPickerOptions(), nil); err != nil {
				return err
			}

			// Prompt for user's name
			if err := userName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			// Prompt for user email
			if err := userEmail.Ask(cmd, &inputs.Email, nil); err != nil {
				return err
			}

			////Prompt for user password
			if err := userPassword.AskPassword(cmd, &inputs.Password, nil); err != nil {
				return err
			}

			// The getConnReqUsername returns the value for the requires_username field for the selected connection
			// The result will be used to determine whether to prompt for username
			conn := cli.getConnReqUsername(auth0.StringValue(&inputs.Connection))
			requireUsername := auth0.BoolValue(conn)

			// Prompt for username if the requireUsername is set to true
			// Load values including the username's field into a fresh users instance
			a := &management.User{
				Connection: &inputs.Connection,
				Email:      &inputs.Email,
				Name:       &inputs.Name,
				Password:   &inputs.Password,
			}

			if requireUsername {
				if err := userUsername.Ask(cmd, &inputs.Username, nil); err != nil {
					return err
				}
				a.Username = &inputs.Username
			}
			// Create app
			if err := ansi.Waiting(func() error {
				return cli.api.User.Create(a)
			}); err != nil {
				return fmt.Errorf("Unable to create user: %w", err)
			}

			// Render Result
			cli.renderer.UserCreate(a, requireUsername)

			return nil
		},
	}
	userName.RegisterString(cmd, &inputs.Name, "")
	userConnection.RegisterString(cmd, &inputs.Connection, "")
	userPassword.RegisterString(cmd, &inputs.Password, "")
	userEmail.RegisterString(cmd, &inputs.Email, "")
	userUsername.RegisterString(cmd, &inputs.Username, "")

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
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := userID.Ask(cmd, &inputs.ID); err != nil {
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
				return fmt.Errorf("Unable to load user: %w", err)
			}

			// get the current connection
			conn := stringSliceToCommaSeparatedString(cli.getUserConnection(a))
			a.Connection = auth0.String(conn)

			// parse the connection name to get the requireUsername status
			u := cli.getConnReqUsername(auth0.StringValue(a.Connection))
			requireUsername := auth0.BoolValue(u)

			cli.renderer.UserShow(a, requireUsername)
			return nil
		},
	}

	return cmd
}

func deleteUserCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "delete",
		Args:  cobra.MaximumNArgs(1),
		Short: "Delete a user",
		Long:  "Delete a user.",
		Example: `auth0 users delete 
auth0 users delete <id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := userID.Ask(cmd, &inputs.ID); err != nil {
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

			return ansi.Spinner("Deleting user", func() error {
				_, err := cli.api.User.Read(inputs.ID)

				if err != nil {
					return fmt.Errorf("Unable to delete user: %w", err)
				}

				return cli.api.User.Delete(inputs.ID)
			})
		},
	}

	return cmd
}

func updateUserCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID         string
		Email      string
		Password   string
		Name       string
		Connection string
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update a user",
		Long:  "Update a user.",
		Example: `auth0 users update 
auth0 users update <id> 
auth0 users update <id> --name John Doe
auth0 users update -n John Doe --email john.doe@example.com`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := userID.Ask(cmd, &inputs.ID); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var current *management.User

			if err := ansi.Waiting(func() error {
				var err error
				current, err = cli.api.User.Read(inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("Unable to load user: %w", err)
			}
			// using getUserConnection to get connection name from user Identities
			// just using current.connection will return empty
			conn := stringSliceToCommaSeparatedString(cli.getUserConnection(current))
			current.Connection = auth0.String(conn)

			if err := userName.AskU(cmd, &inputs.Name, current.Name); err != nil {
				return err
			}

			if err := userEmail.AskU(cmd, &inputs.Email, current.Email); err != nil {
				return err
			}

			if err := userPassword.AskPasswordU(cmd, &inputs.Password, current.Password); err != nil {
				return err
			}

			// username cannot be updated for database connections
			//if err := userUsername.AskU(cmd, &inputs.Username, current.Username); err != nil {
			//	return err
			//}

			user := &management.User{}

			if len(inputs.Name) == 0 {
				user.Name = current.Name
			} else {
				user.Name = &inputs.Name
			}

			if len(inputs.Email) == 0 {
				user.Email = current.Email
			} else {
				user.Email = &inputs.Email
			}

			if len(inputs.Password) == 0 {
				user.Password = current.Password
			} else {
				user.Password = &inputs.Password
			}

			if len(inputs.Connection) == 0 {
				user.Connection = current.Connection
			} else {
				user.Connection = &inputs.Connection
			}

			if err := ansi.Waiting(func() error {
				return cli.api.User.Update(current.GetID(), user)
			}); err != nil {
				return fmt.Errorf("An unexpected error occurred while trying to update an user with Id '%s': %w", inputs.ID, err)
			}

			con := cli.getConnReqUsername(auth0.StringValue(user.Connection))
			requireUsername := auth0.BoolValue(con)

			cli.renderer.UserUpdate(user, requireUsername)
			return nil
		},
	}

	userName.RegisterStringU(cmd, &inputs.Name, "")
	userConnection.RegisterStringU(cmd, &inputs.Connection, "")
	userPassword.RegisterStringU(cmd, &inputs.Password, "")
	userEmail.RegisterStringU(cmd, &inputs.Email, "")

	return cmd
}

func openUserCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "open",
		Args:  cobra.MaximumNArgs(1),
		Short: "Open user details page in the Auth0 Dashboard",
		Long:  "Open user details page in the Auth0 Dashboard.",
		Example: `auth0 users open <id>
auth0 users open "auth0|xxxxxxxxxx"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := userID.Ask(cmd, &inputs.ID); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			openManageURL(cli, cli.config.DefaultTenant, formatUserDetailsPath(url.PathEscape(inputs.ID)))
			return nil
		},
	}

	return cmd
}

func userBlocksCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blocks",
		Short: "Manage brute-force protection user blocks",
		Long: "Manage brute-force protection user blocks.",
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
		Use:     "list",
		Args:    cobra.MaximumNArgs(1),
		Short:   "List brute-force protection blocks for a given user",
		Long:    "List brute-force protection blocks for a given user.",
		Example: "auth0 users blocks list <user-id>",
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
		Use:     "unblock",
		Args:    cobra.MaximumNArgs(1),
		Short:   "Remove brute-force protection blocks for a given user",
		Long:    "Remove brute-force protection blocks for a given user.",
		Example: "auth0 users unblock <user-id>",
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

func formatUserDetailsPath(id string) string {
	if len(id) == 0 {
		return ""
	}
	return fmt.Sprintf("users/%s", id)
}

func (c *cli) connectionPickerOptions() []string {
	var res []string

	list, err := c.api.Connection.List()
	if err != nil {
		fmt.Println(err)
	}
	for _, conn := range list.Connections {
		res = append(res, conn.GetName())
	}

	return res
}

func (c *cli) getUserConnection(users *management.User) []string {
	var res []string
	for _, i := range users.Identities {
		res = append(res, fmt.Sprintf("%v", auth0.StringValue(i.Connection)))

	}

	return res
}

// This is a workaround to get the requires_username field nested inside Options field
func (c *cli) getConnReqUsername(s string) *bool {
	conn, err := c.api.Connection.ReadByName(s)
	if err != nil {
		fmt.Println(err)
	}
	res := fmt.Sprintln(conn.Options)

	opts := &management.ConnectionOptions{}
	if err := json.Unmarshal([]byte(res), &opts); err != nil {
		fmt.Println(err)
	}

	return opts.RequiresUsername
}
