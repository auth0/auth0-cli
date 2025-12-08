package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/iostream"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/auth0/auth0-cli/internal/users"
)

var (
	userID = Argument{
		Name: "User ID",
		Help: "Id of the user.",
	}

	userIdentifier = Argument{
		Name: "User Identifier",
		Help: "User ID, username, email or phone number.",
	}

	userConnectionName = Flag{
		Name:       "Connection Name",
		LongForm:   "connection-name",
		ShortForm:  "c",
		Help:       "Name of the database connection this user should be created in.",
		IsRequired: true,
	}

	userEmail = Flag{
		Name:         "Email",
		LongForm:     "email",
		ShortForm:    "e",
		Help:         "The user's email.",
		IsRequired:   false,
		AlwaysPrompt: true,
	}

	userPhoneNumber = Flag{
		Name:         "Phone Number",
		LongForm:     "phone-number",
		ShortForm:    "m",
		Help:         "The user's phone number.",
		IsRequired:   false,
		AlwaysPrompt: true,
	}

	userPassword = Flag{
		Name:         "Password",
		LongForm:     "password",
		ShortForm:    "p",
		Help:         "Initial password for this user (mandatory for non-SMS connections).",
		IsRequired:   false,
		AlwaysPrompt: true,
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
		IsRequired:   false,
		AlwaysPrompt: true,
	}

	userBlock = Flag{
		Name:       "Block",
		LongForm:   "blocked",
		ShortForm:  "b",
		Help:       "Block the user authentication.",
		IsRequired: false,
	}

	userQuery = Flag{
		Name:       "Query",
		LongForm:   "query",
		ShortForm:  "q",
		Help:       "Search query in Lucene query syntax.\n\nFor example: `email:\"user123@*.com\" OR (user_id:\"user-id-123\" AND name:\"Bob\")`\n\n For more info: https://auth0.com/docs/users/user-search/user-search-query-syntax.",
		IsRequired: true,
	}

	userSort = Flag{
		Name:      "Sort",
		LongForm:  "sort",
		ShortForm: "s",
		Help:      "Field to sort by. Use 'field:order' where 'order' is '1' for ascending and '-1' for descending. e.g. 'created_at:1'.",
	}

	userNumber = Flag{
		Name:      "Number",
		LongForm:  "number",
		ShortForm: "n",
		Help:      "Number of users, that match the search criteria, to retrieve. Minimum 1, maximum 1000. If limit is hit, refine the search query.",
	}

	userImportTemplate = Flag{
		Name:      "Template",
		LongForm:  "template",
		ShortForm: "t",
		Help: "Name of JSON example to be used. Cannot be used if the '--users' flag is passed. " +
			"Options include: 'Empty', 'Basic Example', 'Custom Password Hash Example' and 'MFA Factors Example'.",
		IsRequired: false,
	}

	userImportBody = Flag{
		Name:       "Users Payload",
		LongForm:   "users",
		ShortForm:  "u",
		Help:       "JSON payload that contains an array of user(s) to be imported. Cannot be used if the '--template' flag is passed.",
		IsRequired: false,
	}

	userEmailResults = Flag{
		Name:       "Email Completion Results",
		LongForm:   "email-results",
		Help:       "When true, sends a completion email to all tenant owners when the job is finished. The default is true, so you must explicitly set this parameter to false if you do not want emails sent.",
		IsRequired: false,
	}

	userImportUpsert = Flag{
		Name:       "Upsert",
		LongForm:   "upsert",
		Help:       "When set to false, pre-existing users that match on email address, user ID, or username will fail. When set to true, pre-existing users that match on any of these fields will be updated, but only with upsertable attributes.",
		IsRequired: false,
	}

	userPicker = Flag{
		Name:      "Interactive picker option on rendered users during search",
		LongForm:  "picker",
		ShortForm: "p",
		Help:      "Allows to toggle from list of users and view a user in detail",
	}

	userImportOptions = pickerOptions{
		{"Empty", users.EmptyExample},
		{"Basic Example", users.BasicExample},
		{"Custom Password Hash Example", users.CustomPasswordHashExample},
		{"MFA Factors Example", users.MFAFactors},
	}
)

func usersCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "Manage resources for users",
		Long:  "Manage resources for users.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(searchUsersCmd(cli))
	cmd.AddCommand(searchUsersByEmailCmd(cli))
	cmd.AddCommand(createUserCmd(cli))
	cmd.AddCommand(showUserCmd(cli))
	cmd.AddCommand(updateUserCmd(cli))
	cmd.AddCommand(deleteUserCmd(cli))
	cmd.AddCommand(userRolesCmd(cli))
	cmd.AddCommand(openUserCmd(cli))
	cmd.AddCommand(userBlocksCmd(cli))
	cmd.AddCommand(importUsersCmd(cli))

	return cmd
}

func searchUsersCmd(cli *cli) *cobra.Command {
	var inputs struct {
		query  string
		sort   string
		number int
		picker bool
	}

	cmd := &cobra.Command{
		Use:   "search",
		Args:  cobra.NoArgs,
		Short: "Search for users",
		Long:  "Search for users. To create one, run: `auth0 users create`.",
		Example: `  auth0 users search
  auth0 users search --query user_id:"<user-id>"
  auth0 users search --query name:"Bob" --sort "name:1"
  auth0 users search --query name:"Bob" --sort "name:1 --picker"
  auth0 users search -q name:"Bob" -s "name:1" --number 200
  auth0 users search -q name:"Bob" -s "name:1" -n 200 -p --json
  auth0 users search -q name:"Bob" -s "name:1" -n 200 --csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := userQuery.Ask(cmd, &inputs.query, nil); err != nil {
				return err
			}

			queryParams := []management.RequestOption{
				management.Query(inputs.query),
			}
			if inputs.sort != "" {
				queryParams = append(queryParams, management.Parameter("sort", inputs.sort))
			}

			if inputs.number < 1 || inputs.number > 1000 {
				return fmt.Errorf("number flag invalid, please pass a number between 1 and 1000")
			}

			list, err := getWithPagination(
				inputs.number,
				func(opts ...management.RequestOption) (result []interface{}, hasNext bool, err error) {
					opts = append(opts, queryParams...)

					userList, err := cli.api.User.Search(cmd.Context(), opts...)
					if err != nil {
						return nil, false, err
					}

					var output []interface{}
					for _, user := range userList.Users {
						output = append(output, user)
					}

					return output, userList.HasNext(), nil
				},
			)
			if err != nil {
				return fmt.Errorf("failed to search for users: %w", err)
			}

			var foundUsers []*management.User
			for _, item := range list {
				foundUsers = append(foundUsers, item.(*management.User))
			}

			if !inputs.picker || len(foundUsers) == 0 {
				cli.renderer.UserSearch(foundUsers)
			} else {
				var (
					selectedUserID string
					currentIndex   = auth0.Int(0)
				)
				for {
					selectedUserID = cli.renderer.UserPrompt(foundUsers, currentIndex)

					userDetail, err := cli.api.User.Read(cmd.Context(), selectedUserID)
					if err != nil {
						fmt.Println("Failed to fetch details:", err)
						continue
					}

					fmt.Println("\nUser Details:")
					cli.renderer.JSONResult(userDetail)

					if cli.renderer.QuitPrompt() {
						break
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")

	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	userQuery.RegisterString(cmd, &inputs.query, "")
	userSort.RegisterString(cmd, &inputs.sort, "")
	userPicker.RegisterBool(cmd, &inputs.picker, false)
	userNumber.RegisterInt(cmd, &inputs.number, defaultPageSize)

	return cmd
}

func searchUsersByEmailCmd(cli *cli) *cobra.Command {
	var inputs struct {
		email  string
		picker bool
	}

	cmd := &cobra.Command{
		Use:   "search-by-email",
		Args:  cobra.MaximumNArgs(1),
		Short: "Search for users",
		Long:  "Search for users. To create one, run: `auth0 users create`.",
		Example: `  auth0 users search-by-email
  auth0 users search-by-email <user-email>,
  auth0 users search-by-email <user-email> -p`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var emailID string

			if len(args) == 0 {
				if err := userEmail.Ask(cmd, &emailID, nil); err != nil {
					return err
				}
			} else {
				emailID = args[0]
			}

			usersList, err := cli.api.User.ListByEmail(cmd.Context(), emailID)
			if err != nil {
				return fmt.Errorf("failed to search for users with email - %v: %w", emailID, err)
			}

			if !inputs.picker || len(usersList) == 0 {
				cli.renderer.UserSearch(usersList)
			} else {
				var (
					selectedUserID string
					currentIndex   = auth0.Int(0)
				)
				for {
					selectedUserID = cli.renderer.UserPrompt(usersList, currentIndex)

					userDetail, err := cli.api.User.Read(cmd.Context(), selectedUserID)
					if err != nil {
						fmt.Println("Failed to fetch details:", err)
						continue
					}

					fmt.Println("\nUser Details:")
					cli.renderer.JSONResult(userDetail)

					if cli.renderer.QuitPrompt() {
						break
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	userPicker.RegisterBool(cmd, &inputs.picker, false)

	return cmd
}

type userInput struct {
	connectionName string
	name           string
	username       string
	password       string
	email          string
	phoneNumber    string
}

func createUserCmd(cli *cli) *cobra.Command {
	var inputs userInput

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new user",
		Long: "Create a new user.\n\n" +
			"To create interactively, use `auth0 users create` with no flags.\n\n" +
			"To create non-interactively, supply the name and other information through the available flags.",
		Example: `  auth0 users create 
  auth0 users create --name "John Doe" 
  auth0 users create --name "John Doe" --email john@example.com
  auth0 users create --name "John Doe" --email john@example.com --connection-name "Username-Password-Authentication" --username "example"
  auth0 users create -n "John Doe" -e john@example.com -c "Username-Password-Authentication" -u "example" --json
  auth0 users create -n "John Doe" -e john@example.com -c "Username-Password-Authentication" -u "example" --json-compact
  auth0 users create -n "John Doe" -e john@example.com -c "email" --json
  auth0 users create -e john@example.com -c "email"
  auth0 users create --phone-number +916898989898 --connection-name "sms"
  auth0 users create -m +916898989898 -c "sms" --json
  auth0 users create -m +916898989898 -c "sms" --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate provided flags basis on the given connection type.
			if cli.noInput {
				if err := validateRequiredFlags(&inputs); err != nil {
					return err
				}
			}

			options, err := cli.databaseAndPasswordlessConnectionOptions(cmd.Context())
			if err != nil {
				return err
			}

			if err := userConnectionName.Select(cmd, &inputs.connectionName, options, nil); err != nil {
				return err
			}

			connection, err := cli.api.Connection.ReadByName(cmd.Context(), inputs.connectionName)
			if err != nil {
				return fmt.Errorf("failed to find connection with name %q: %w", inputs.connectionName, err)
			}

			if len(connection.GetEnabledClients()) == 0 {
				return fmt.Errorf(
					"failed to continue due to the connection with name %q being disabled, enable an application on this connection and try again",
					inputs.connectionName,
				)
			}

			var (
				user     *management.User
				strategy = connection.GetStrategy()
			)

			// Fetch user info based on the connection's strategy.
			switch strategy {
			case management.ConnectionStrategyAuth0:
				user, err = retrieveAuth0UserDetails(cmd, &inputs)
				if err != nil {
					return err
				}

			case management.ConnectionStrategySMS:
				user, err = retrieveSMSUserDetails(cmd, &inputs)
				if err != nil {
					return err
				}

			case management.ConnectionStrategyEmail:
				user, err = retrieveEmailUserDetails(cmd, &inputs)
				if err != nil {
					return err
				}
			}

			// The getConnReqUsername returns the value for the requires_username field for the selected connection
			// The result will be used to determine whether to prompt for username.
			conn := cli.getConnReqUsername(cmd.Context(), auth0.StringValue(&inputs.connectionName))
			requiredUsername := auth0.BoolValue(conn)

			// Prompt for username if the requireUsername is set to true
			// Load values including the username's field into a fresh users instance.
			if requiredUsername {
				if err := userUsername.Ask(cmd, &inputs.username, nil); err != nil {
					return err
				}

				user.Username = &inputs.username
			}

			if err := ansi.Waiting(func() error {
				return cli.api.User.Create(cmd.Context(), user)
			}); err != nil {
				return fmt.Errorf("failed to create user: %w", err)
			}

			cli.renderer.UserCreate(user, requiredUsername)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	registerDetailsInfo(cmd, &inputs)

	return cmd
}

// retrieveAuth0UserDetails retrieves required fields: email, and password for Auth0 strategy.
func retrieveAuth0UserDetails(cmd *cobra.Command, input *userInput) (*management.User, error) {
	if err := userEmail.Ask(cmd, &input.email, nil); err != nil {
		return nil, err
	}

	if err := userPassword.AskPassword(cmd, &input.password); err != nil {
		return nil, err
	}

	userInfo := &management.User{
		Email:      &input.email,
		Password:   &input.password,
		Connection: &input.connectionName,
	}

	// User's name is optional for auth0 connection and takes the email-id as default.
	if input.name != "" {
		userInfo.Name = &input.name
	}

	return userInfo, nil
}

// retrieveSMSUserDetails retrieves required fields: phone-number for sms strategy.
func retrieveSMSUserDetails(cmd *cobra.Command, input *userInput) (*management.User, error) {
	if err := userPhoneNumber.Ask(cmd, &input.phoneNumber, nil); err != nil {
		return nil, err
	}

	userInfo := &management.User{
		PhoneNumber:   &input.phoneNumber,
		PhoneVerified: auth0.Bool(true),
		Connection:    &input.connectionName,
	}

	return userInfo, nil
}

// retrieveEmailUserDetails retrieves required fields: email for email strategy.
func retrieveEmailUserDetails(cmd *cobra.Command, input *userInput) (*management.User, error) {
	if err := userEmail.Ask(cmd, &input.email, nil); err != nil {
		return nil, err
	}

	userInfo := &management.User{
		Email:      &input.email,
		Connection: &input.connectionName,
	}

	// User's name is optional for email connection and takes the email-id as default.
	if input.name != "" {
		userInfo.Name = &input.name
	}

	return userInfo, nil
}

func registerDetailsInfo(cmd *cobra.Command, input *userInput) {
	userConnectionName.RegisterString(cmd, &input.connectionName, "")
	userUsername.RegisterString(cmd, &input.username, "")
	userName.RegisterString(cmd, &input.name, "")
	userPassword.RegisterString(cmd, &input.password, "")
	userEmail.RegisterString(cmd, &input.email, "")
	userPhoneNumber.RegisterString(cmd, &input.phoneNumber, "")
}

func validateRequiredFlags(inputs *userInput) error {
	switch inputs.connectionName {
	case "email":
		if inputs.email == "" {
			return fmt.Errorf("required flag email not set")
		}
	case "sms":
		if inputs.phoneNumber == "" {
			return fmt.Errorf("required flag phone-number not set")
		}
	default:
		if inputs.email == "" || inputs.password == "" {
			return fmt.Errorf("required flag email or password not set")
		}
	}

	return nil
}

func showUserCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show an existing user",
		Long:  "Display information about an existing user.",
		Example: `  auth0 users show 
  auth0 users show <user-id>
  auth0 users show <user-id> --json
  auth0 users show <user-id> --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := userID.Ask(cmd, &inputs.ID); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			user := &management.User{ID: &inputs.ID}

			if err := ansi.Waiting(func() error {
				var err error
				user, err = cli.api.User.Read(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to load user with ID %q: %w", inputs.ID, err)
			}

			// Get the current connection.
			conn := stringSliceToCommaSeparatedString(cli.getUserConnection(user))
			user.Connection = auth0.String(conn)

			// Parse the connection name to get the requireUsername status.
			u := cli.getConnReqUsername(cmd.Context(), auth0.StringValue(user.Connection))
			requireUsername := auth0.BoolValue(u)

			cli.renderer.UserShow(user, requireUsername)

			if auth0.BoolValue(user.Blocked) && !cli.json {
				cli.renderer.Newline()
				cli.renderer.Warnf("This user is %s and cannot authenticate.\n", ansi.BrightRed("blocked"))
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func deleteUserCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Short:   "Delete a user",
		Long: "Delete a user.\n\n" +
			"To delete interactively, use `auth0 users delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the user id and the `--force` flag to skip confirmation.",
		Example: `  auth0 users delete 
  auth0 users rm
  auth0 users delete <user-id>
  auth0 users delete <user-id> --force
  auth0 users delete <user-id> <user-id2> <user-idn>
  auth0 users delete <user-id> <user-id2> <user-idn> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ids := make([]string, len(args))
			if len(args) == 0 {
				var id string
				if err := userID.Ask(cmd, &id); err != nil {
					return err
				}
				ids = append(ids, id)
			} else {
				ids = append(ids, args...)
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			return ansi.ProgressBar("Deleting user(s)", ids, func(_ int, id string) error {
				if id != "" {
					if _, err := cli.api.User.Read(cmd.Context(), id); err != nil {
						return fmt.Errorf("failed to delete user with ID %q: %w", id, err)
					}

					if err := cli.api.User.Delete(cmd.Context(), id); err != nil {
						return fmt.Errorf("failed to delete user with ID %q: %w", id, err)
					}
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

func updateUserCmd(cli *cli) *cobra.Command {
	var (
		inputs  = &userInput{}
		blocked bool
		id      string
	)

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update a user",
		Long: "Update a user.\n\n" +
			"To update interactively, use `auth0 users update` with no arguments.\n\n" +
			"To update non-interactively, supply the user id and other information through the available flags.",
		Example: `  auth0 users update 
  auth0 users update <user-id> 
  auth0 users update <user-id> --name "John Doe"
  auth0 users update <user-id> --blocked=true"
  auth0 users update <user-id> --blocked=false"
  auth0 users update <user-id> -n "John Kennedy" -e johnk@example.com --json
  auth0 users update <user-id> -n "John Kennedy" -e johnk@example.com --json-compact
  auth0 users update <user-id> -n "John Kennedy" -p <newPassword>
  auth0 users update <user-id> -b
  auth0 users update <user-id> -p <newPassword>
  auth0 users update <user-id> -e johnk@example.com
  auth0 users update <user-id> --phone-number +916898989899
  auth0 users update <user-id> -m +916898989899 --json`,

		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := userID.Ask(cmd, &id); err != nil {
					return err
				}
			} else {
				id = args[0]
			}

			var current *management.User

			if err := ansi.Waiting(func() error {
				var err error
				current, err = cli.api.User.Read(cmd.Context(), id)
				return err
			}); err != nil {
				return fmt.Errorf("failed to read user with ID %q: %w", id, err)
			}
			// Using getUserConnection to get connection name from user Identities
			// just using current.connection will return empty.
			conn := stringSliceToCommaSeparatedString(cli.getUserConnection(current))
			current.Connection = auth0.String(conn)

			if err := fetchUserInputByConnection(cmd, inputs, current); err != nil {
				return err
			}

			user := fetchUpdateUserDetails(inputs, current)

			if blocked {
				user.Blocked = auth0.Bool(true)
			} else {
				user.Blocked = auth0.Bool(false)
			}

			if err := ansi.Waiting(func() error {
				return cli.api.User.Update(cmd.Context(), current.GetID(), user)
			}); err != nil {
				return fmt.Errorf("failed to update user with ID %q: %w", id, err)
			}

			con := cli.getConnReqUsername(cmd.Context(), auth0.StringValue(user.Connection))
			requireUsername := auth0.BoolValue(con)

			cli.renderer.UserUpdate(user, requireUsername)

			if *user.Blocked && !cli.json {
				cli.renderer.Newline()
				cli.renderer.Warnf("This user is %s and cannot authenticate.\n", ansi.BrightRed("blocked"))
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	registerDetailsInfo(cmd, inputs)
	userBlock.RegisterBool(cmd, &blocked, false)

	return cmd
}

func fetchUserInputByConnection(cmd *cobra.Command, inputs *userInput, current *management.User) error {
	switch *current.Connection {
	case "email":
		if err := userEmail.AskU(cmd, &inputs.email, current.Email); err != nil {
			return err
		}
	case "sms":
		if err := userPhoneNumber.AskU(cmd, &inputs.phoneNumber, current.PhoneNumber); err != nil {
			return err
		}
	default:
		if err := userName.AskU(cmd, &inputs.name, current.Name); err != nil {
			return err
		}
		if err := userEmail.AskU(cmd, &inputs.email, current.Email); err != nil {
			return err
		}
		if err := userPassword.AskPasswordU(cmd, &inputs.password); err != nil {
			return err
		}
	}
	return nil
}

func fetchUpdateUserDetails(inputs *userInput, current *management.User) *management.User {
	user := &management.User{}

	switch *current.Connection {
	case "email":
		if len(inputs.email) != 0 {
			user.Email = &inputs.email
		}
	case "sms":
		if len(inputs.phoneNumber) != 0 {
			user.PhoneNumber = &inputs.phoneNumber
		}
	default:
		if len(inputs.email) != 0 && current.Email != &inputs.email {
			user.Email = &inputs.email
		}

		if len(inputs.password) != 0 {
			user.Password = &inputs.password
		}
	}

	if len(inputs.name) == 0 {
		user.Name = current.Name
	} else {
		user.Name = &inputs.name
	}

	if len(inputs.connectionName) == 0 {
		user.Connection = current.Connection
	} else {
		user.Connection = &inputs.connectionName
	}

	return user
}

func openUserCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "open",
		Args:  cobra.MaximumNArgs(1),
		Short: "Open the user's settings page",
		Long:  "Open the settings page of a user in the Auth0 Dashboard.",
		Example: `  auth0 users open <id>
  auth0 users open "auth0|xxxxxxxxxx"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := userID.Ask(cmd, &inputs.ID); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			openManageURL(cli, cli.Config.DefaultTenant, formatUserDetailsPath(url.PathEscape(inputs.ID)))
			return nil
		},
	}

	return cmd
}

func importUsersCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ConnectionName      string
		ConnectionID        string
		Template            string
		UsersBody           string
		Upsert              bool
		SendCompletionEmail bool
	}
	cmd := &cobra.Command{
		Use:   "import",
		Args:  cobra.NoArgs,
		Short: "Import users from schema",
		Long: `Import users from schema. Issues a Create Import Users Job. 
The file size limit for a bulk import is 500KB. You will need to start multiple imports if your data exceeds this size.`,
		Example: `  auth0 users import
  auth0 users import --connection-name "Username-Password-Authentication"
  auth0 users import --connection-name "Username-Password-Authentication" --users "[]"
  auth0 users import --connection-name "Username-Password-Authentication" --users "$(cat path/to/users.json)"
  cat path/to/users.json | auth0 users import --connection-name "Username-Password-Authentication"
  auth0 users import -c "Username-Password-Authentication" --template "Basic Example"
  auth0 users import -c "Username-Password-Authentication" --users "$(cat path/to/users.json)" --upsert --email-results
  auth0 users import -c "Username-Password-Authentication" --users "$(cat path/to/users.json)" --upsert --email-results --no-input
  cat path/to/users.json | auth0 users import -c "Username-Password-Authentication" --upsert --email-results --no-input
  auth0 users import -c "Username-Password-Authentication" -u "$(cat path/to/users.json)" --upsert --email-results
  cat path/to/users.json | auth0 users import -c "Username-Password-Authentication" --upsert --email-results
  auth0 users import -c "Username-Password-Authentication" -t "Basic Example" --upsert --email-results
  auth0 users import -c "Username-Password-Authentication" -t "Basic Example" --upsert=false --email-results=false
  auth0 users import -c "Username-Password-Authentication" -t "Basic Example" --upsert=false --email-results=false`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Users API currently only supports database connections.
			dbConnectionOptions, err := cli.databaseAndPasswordlessConnectionOptions(cmd.Context())
			if err != nil {
				return err
			}

			if err := userConnectionName.Select(cmd, &inputs.ConnectionName, dbConnectionOptions, nil); err != nil {
				return err
			}

			connection, err := cli.api.Connection.ReadByName(cmd.Context(), inputs.ConnectionName)
			if err != nil {
				return fmt.Errorf("failed to read connection with name %q: %w", inputs.ConnectionName, err)
			}

			if len(connection.GetEnabledClients()) == 0 {
				return fmt.Errorf(
					"failed to continue due to the connection with name %q being disabled, enable an application on this connection and try again",
					inputs.ConnectionName,
				)
			}

			inputs.ConnectionID = connection.GetID()

			pipedUsersBody := iostream.PipedInput()
			if len(pipedUsersBody) > 0 && inputs.UsersBody == "" {
				inputs.UsersBody = string(pipedUsersBody)
			}

			if inputs.UsersBody == "" {
				err := userImportTemplate.Select(cmd, &inputs.Template, userImportOptions.labels(), nil)
				if err != nil {
					return err
				}

				if err := userImportBody.OpenEditor(
					cmd,
					&inputs.UsersBody,
					userImportOptions.getValue(inputs.Template),
					inputs.Template+".*.json",
					cli.userImportEditorHint,
				); err != nil {
					return fmt.Errorf("failed to capture input from the editor: %w", err)
				}
			}

			if canPrompt(cmd) {
				var confirmed bool
				if err := prompt.AskBool("Do you want to import these user(s)?", &confirmed, true); err != nil {
					return fmt.Errorf("failed to capture prompt input: %w", err)
				}
				if !confirmed {
					return nil
				}
			}

			var usersBody []map[string]interface{}
			if err := json.Unmarshal([]byte(inputs.UsersBody), &usersBody); err != nil {
				return fmt.Errorf("invalid JSON input: %w", err)
			}

			job := &management.Job{
				ConnectionID:        &inputs.ConnectionID,
				Users:               usersBody,
				Upsert:              &inputs.Upsert,
				SendCompletionEmail: &inputs.SendCompletionEmail,
			}

			if err := ansi.Waiting(func() error {
				return cli.api.Jobs.ImportUsers(cmd.Context(), job)
			}); err != nil {
				return fmt.Errorf("failed to import users: %w", err)
			}

			cli.renderer.Heading("started user import job")
			cli.renderer.Infof("Job with ID '%s' successfully started.", ansi.Bold(job.GetID()))
			cli.renderer.Infof("Run '%s' to get the status of the job.", ansi.Cyan("auth0 api jobs/"+job.GetID()))

			if inputs.SendCompletionEmail {
				cli.renderer.Infof("Results of your user import job will be sent to your email.")
			}

			return nil
		},
	}

	userConnectionName.RegisterString(cmd, &inputs.ConnectionName, "")
	userImportTemplate.RegisterString(cmd, &inputs.Template, "")
	userImportBody.RegisterString(cmd, &inputs.UsersBody, "")
	userEmailResults.RegisterBool(cmd, &inputs.SendCompletionEmail, true)
	userImportUpsert.RegisterBool(cmd, &inputs.Upsert, false)
	cmd.MarkFlagsMutuallyExclusive("template", "users")

	return cmd
}

func formatUserDetailsPath(id string) string {
	if len(id) == 0 {
		return ""
	}
	return fmt.Sprintf("users/%s", id)
}

func (c *cli) databaseAndPasswordlessConnectionOptions(ctx context.Context) ([]string, error) {
	connectionList, err := c.api.Connection.List(
		ctx,
		management.Parameter("strategy[0]", management.ConnectionStrategyAuth0),
		management.Parameter("strategy[1]", management.ConnectionStrategyEmail),
		management.Parameter("strategy[2]", management.ConnectionStrategySMS),
		management.PerPage(100),
	)
	if err != nil {
		return nil, err
	}

	var connectionNames []string
	for _, connection := range connectionList.Connections {
		if len(connection.GetEnabledClients()) == 0 {
			continue
		}

		connectionNames = append(connectionNames, connection.GetName())
	}

	if len(connectionNames) == 0 {
		return nil, errors.New("there are currently no active database or passwordless connections to choose from")
	}

	return connectionNames, nil
}

func (c *cli) getUserConnection(users *management.User) []string {
	var res []string
	for _, i := range users.Identities {
		res = append(res, i.GetConnection())
	}
	return res
}

// This is a workaround to get the requires_username field nested inside Options field.
func (c *cli) getConnReqUsername(ctx context.Context, s string) *bool {
	conn, err := c.api.Connection.ReadByName(ctx, s)
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

func (c *cli) userImportEditorHint() {
	c.renderer.Infof("%s Once you close the editor, the user(s) will be imported. To cancel, CTRL+C.", ansi.Faint("Hint:"))
}
