package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/prompt"
)

// errNoApps signifies no applications exist in a tenant.
var errNoApps = errors.New("there are currently no applications")

const (
	appTypeNative         = "native"
	appTypeSPA            = "spa"
	appTypeRegularWeb     = "regular_web"
	appTypeNonInteractive = "non_interactive"
	appDefaultURL         = "http://localhost:3000"
	defaultPageSize       = 50
)

var (
	appID = Argument{
		Name: "Client ID",
		Help: "Id of the application.",
	}
	appName = Flag{
		Name:       "Name",
		LongForm:   "name",
		ShortForm:  "n",
		Help:       "Name of the application.",
		IsRequired: true,
	}
	appNone = Flag{
		Name:      "None",
		LongForm:  "none",
		ShortForm: "n",
		Help:      "Specify none of your apps.",
	}
	appType = Flag{
		Name:      "Type",
		LongForm:  "type",
		ShortForm: "t",
		Help: "Type of application:\n" +
			"- native: mobile, desktop, CLI and smart device apps running natively.\n" +
			"- spa (single page application): a JavaScript front-end app that uses an API.\n" +
			"- regular: Traditional web app using redirects.\n" +
			"- m2m (machine to machine): CLIs, daemons or services running on your backend.",
		IsRequired: true,
	}
	appTypeOptions = []string{
		"Native",
		"Single Page Web Application",
		"Regular Web Application",
		"Machine to Machine",
	}
	appDescription = Flag{
		Name:       "Description",
		LongForm:   "description",
		ShortForm:  "d",
		Help:       "Description of the application. Max character count is 140.",
		IsRequired: false,
	}
	appCallbacks = Flag{
		Name:         "Callback URLs",
		LongForm:     "callbacks",
		ShortForm:    "c",
		Help:         "After the user authenticates we will only call back to any of these URLs. You can specify multiple valid URLs by comma-separating them (typically to handle different environments like QA or testing). Make sure to specify the protocol (https://) otherwise the callback may fail in some cases. With the exception of custom URI schemes for native apps, all callbacks should use protocol https://.",
		IsRequired:   false,
		AlwaysPrompt: true,
	}
	appOrigins = Flag{
		Name:         "Allowed Origin URLs",
		LongForm:     "origins",
		ShortForm:    "o",
		Help:         "Comma-separated list of URLs allowed to make requests from JavaScript to Auth0 API (typically used with CORS). By default, all your callback URLs will be allowed. This field allows you to enter other origins if necessary. You can also use wildcards at the subdomain level (e.g., https://*.contoso.com). Query strings and hash information are not taken into account when validating these URLs.",
		IsRequired:   false,
		AlwaysPrompt: true,
	}
	appWebOrigins = Flag{
		Name:         "Allowed Web Origin URLs",
		LongForm:     "web-origins",
		ShortForm:    "w",
		Help:         "Comma-separated list of allowed origins for use with Cross-Origin Authentication, Device Flow, and web message response mode.",
		IsRequired:   false,
		AlwaysPrompt: true,
	}
	appLogoutURLs = Flag{
		Name:         "Allowed Logout URLs",
		LongForm:     "logout-urls",
		ShortForm:    "l",
		Help:         "Comma-separated list of URLs that are valid to redirect to after logout from Auth0. Wildcards are allowed for subdomains.",
		IsRequired:   false,
		AlwaysPrompt: true,
	}
	appAuthMethod = Flag{
		Name:       "Auth Method",
		LongForm:   "auth-method",
		ShortForm:  "a",
		Help:       "Defines the requested authentication method for the token endpoint. Possible values are 'None' (public application without a client secret), 'Post' (application uses HTTP POST parameters) or 'Basic' (application uses HTTP Basic).",
		IsRequired: false,
	}
	appGrants = Flag{
		Name:       "Grants",
		LongForm:   "grants",
		ShortForm:  "g",
		Help:       "List of grant types supported for this application. Can include code, implicit, refresh-token, credentials, password, password-realm, mfa-oob, mfa-otp, mfa-recovery-code, and device-code.",
		IsRequired: false,
	}
	revealSecrets = Flag{
		Name:      "Reveal",
		LongForm:  "reveal-secrets",
		ShortForm: "r",
		Help:      "Display the application secrets ('signing_keys', 'client_secret') as part of the command output.",
	}
	appNumber = Flag{
		Name:      "Number",
		LongForm:  "number",
		ShortForm: "n",
		Help:      "Number of apps to retrieve. Minimum 1, maximum 1000.",
	}
)

func appsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apps",
		Short: "Manage resources for applications",
		Long: "The term application or app in Auth0 does not imply any particular implementation characteristics. " +
			"For example, it could be a native app that executes on a mobile device, a single-page application that " +
			"executes on a browser, or a regular web application that executes on a server.",
		Aliases: []string{"clients"},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(useAppCmd(cli))
	cmd.AddCommand(listAppsCmd(cli))
	cmd.AddCommand(createAppCmd(cli))
	cmd.AddCommand(showAppCmd(cli))
	cmd.AddCommand(updateAppCmd(cli))
	cmd.AddCommand(deleteAppCmd(cli))
	cmd.AddCommand(openAppCmd(cli))

	return cmd
}

func useAppCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID   string
		None bool
	}

	cmd := &cobra.Command{
		Use:   "use",
		Args:  cobra.MaximumNArgs(1),
		Short: "Choose a default application for the Auth0 CLI",
		Long: "Specify the default application used when running other commands. Specifically when downloading " +
			"quickstarts and testing Universal login flow.",
		Example: `  auth0 apps use
  auth0 apps use --none
  auth0 apps use <app-id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.None {
				inputs.ID = ""
			} else {
				if len(args) == 0 {
					err := appID.Pick(cmd, &inputs.ID, cli.appPickerOptions())
					if err != nil {
						return err
					}
				} else {
					inputs.ID = args[0]
				}
			}

			if err := cli.setDefaultAppID(inputs.ID); err != nil {
				return err
			}

			if inputs.ID == "" {
				cli.renderer.Infof("Successfully removed the default application")
			} else {
				cli.renderer.Infof("Successfully set the default application to %s", ansi.Faint(inputs.ID))
				cli.renderer.Infof("%s Consider running `auth0 quickstarts download %s`", ansi.Faint("Hint:"), inputs.ID)
			}

			return nil
		},
	}

	appNone.RegisterBool(cmd, &inputs.None, false)

	return cmd
}

func listAppsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		RevealSecrets bool
		Number        int
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your applications",
		Long:    "List your existing applications. To create one, run: `auth0 apps create`.",
		Example: `  auth0 apps list
  auth0 apps ls
  auth0 apps list --reveal-secrets
  auth0 apps list --reveal-secrets --number 100
  auth0 apps ls -r -n 100 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Number < 1 || inputs.Number > 1000 {
				return fmt.Errorf("number flag invalid, please pass a number between 1 and 1000")
			}

			list, err := getWithPagination(
				cmd.Context(),
				inputs.Number,
				func(opts ...management.RequestOption) (result []interface{}, hasNext bool, err error) {
					opts = append(opts, management.Parameter("is_global", "false")) // Excluding deprecated global client from list
					res, apiErr := cli.api.Client.List(opts...)
					if apiErr != nil {
						return nil, false, apiErr
					}
					var output []interface{}
					for _, client := range res.Clients {
						output = append(output, client)
					}
					return output, res.HasNext(), nil
				})
			if err != nil {
				return fmt.Errorf("An unexpected error occurred: %w", err)
			}
			var typedList []*management.Client
			for _, item := range list {
				typedList = append(typedList, item.(*management.Client))
			}
			cli.renderer.ApplicationList(typedList, inputs.RevealSecrets)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	revealSecrets.RegisterBool(cmd, &inputs.RevealSecrets, false)
	appNumber.RegisterInt(cmd, &inputs.Number, defaultPageSize)

	return cmd
}

func showAppCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID            string
		RevealSecrets bool
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show an application",
		Long:  "Display the name, description, app type, and other information about an application.",
		Example: `  auth0 apps show
  auth0 apps show <app-id>
  auth0 apps show <app-id> --reveal-secrets
  auth0 apps show <app-id> -r --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := appID.Pick(cmd, &inputs.ID, cli.appPickerOptions())
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			a := &management.Client{ClientID: &inputs.ID}

			if err := ansi.Waiting(func() error {
				var err error
				a, err = cli.api.Client.Read(inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("Unable to load application: %w", err)
			}

			cli.renderer.ApplicationShow(a, inputs.RevealSecrets)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	revealSecrets.RegisterBool(cmd, &inputs.RevealSecrets, false)

	return cmd
}

func deleteAppCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "Delete an application",
		Long: "Delete an application.\n\n" +
			"To delete interactively, use `auth0 apps delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the application id and the `--force` " +
			"flag to skip confirmation.",
		Example: `  auth0 apps delete 
  auth0 apps rm
  auth0 apps delete <app-id>
  auth0 apps delete <app-id> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := appID.Pick(cmd, &inputs.ID, cli.appPickerOptions())
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

			return ansi.Spinner("Deleting Application", func() error {
				_, err := cli.api.Client.Read(inputs.ID)

				if err != nil {
					return fmt.Errorf("Unable to delete application: %w", err)
				}

				return cli.api.Client.Delete(inputs.ID)
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

func createAppCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Name              string
		Type              string
		Description       string
		Callbacks         []string
		AllowedOrigins    []string
		AllowedWebOrigins []string
		AllowedLogoutURLs []string
		AuthMethod        string
		Grants            []string
		RevealSecrets     bool
	}
	var oidcConformant = true
	var algorithm = "RS256"

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new application",
		Long: "Create a new application.\n\n" +
			"To create interactively, use `auth0 apps create` with no arguments.\n\n" +
			"To create non-interactively, supply at least the application name, and type through the flags.",
		Example: `  auth0 apps create
  auth0 apps create --name myapp 
  auth0 apps create --name myapp --description <description>
  auth0 apps create --name myapp --description <description> --type [native|spa|regular|m2m]
  auth0 apps create --name myapp --description <description> --type [native|spa|regular|m2m] --reveal-secrets
  auth0 apps create -n myapp -d <description> -t [native|spa|regular|m2m] -r --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Prompt for app name
			if err := appName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			// Prompt for app description
			if err := appDescription.Ask(cmd, &inputs.Description, nil); err != nil {
				return err
			}

			// Prompt for app type
			if err := appType.Select(cmd, &inputs.Type, appTypeOptions, nil); err != nil {
				return err
			}

			appIsM2M := apiTypeFor(inputs.Type) == appTypeNonInteractive
			appIsNative := apiTypeFor(inputs.Type) == appTypeNative
			appIsSPA := apiTypeFor(inputs.Type) == appTypeSPA

			// Prompt for callback URLs if app is not m2m
			if !appIsM2M {
				var defaultValue string

				if !appIsNative {
					defaultValue = appDefaultURL
				}

				if err := appCallbacks.AskMany(cmd, &inputs.Callbacks, &defaultValue); err != nil {
					return err
				}
			}

			// Prompt for logout URLs if app is not m2m
			if !appIsM2M {
				var defaultValue string

				if !appIsNative {
					defaultValue = appDefaultURL
				}

				if err := appLogoutURLs.AskMany(cmd, &inputs.AllowedLogoutURLs, &defaultValue); err != nil {
					return err
				}
			}

			// Prompt for allowed origins URLs if app is SPA
			if appIsSPA {
				defaultValue := appDefaultURL

				if err := appOrigins.AskMany(cmd, &inputs.AllowedOrigins, &defaultValue); err != nil {
					return err
				}
			}

			// Prompt for allowed web origins URLs if app is SPA
			if appIsSPA {
				defaultValue := appDefaultURL

				if err := appWebOrigins.AskMany(cmd, &inputs.AllowedWebOrigins, &defaultValue); err != nil {
					return err
				}
			}

			// Load values into a fresh app instance
			a := &management.Client{
				Name:              &inputs.Name,
				Description:       &inputs.Description,
				AppType:           auth0.String(apiTypeFor(inputs.Type)),
				Callbacks:         stringSliceToPtr(inputs.Callbacks),
				AllowedOrigins:    stringSliceToPtr(inputs.AllowedOrigins),
				WebOrigins:        stringSliceToPtr(inputs.AllowedWebOrigins),
				AllowedLogoutURLs: stringSliceToPtr(inputs.AllowedLogoutURLs),
				OIDCConformant:    &oidcConformant,
				JWTConfiguration:  &management.ClientJWTConfiguration{Algorithm: &algorithm},
			}

			// Set token endpoint auth method
			if len(inputs.AuthMethod) == 0 {
				a.TokenEndpointAuthMethod = apiDefaultAuthMethodFor(inputs.Type)
			} else {
				a.TokenEndpointAuthMethod = apiAuthMethodFor(inputs.AuthMethod)
			}

			// Set grants
			if len(inputs.Grants) == 0 {
				a.GrantTypes = apiDefaultGrantsFor(inputs.Type)
			} else {
				a.GrantTypes = apiGrantsFor(inputs.Grants)
			}

			// Create app
			if err := ansi.Waiting(func() error {
				return cli.api.Client.Create(a)
			}); err != nil {
				return fmt.Errorf("Unable to create application: %v", err)
			}

			if err := cli.setDefaultAppID(a.GetClientID()); err != nil {
				return err
			}

			// Render result
			cli.renderer.ApplicationCreate(a, inputs.RevealSecrets)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	appName.RegisterString(cmd, &inputs.Name, "")
	appType.RegisterString(cmd, &inputs.Type, "")
	appDescription.RegisterString(cmd, &inputs.Description, "")
	appCallbacks.RegisterStringSlice(cmd, &inputs.Callbacks, nil)
	appOrigins.RegisterStringSlice(cmd, &inputs.AllowedOrigins, nil)
	appWebOrigins.RegisterStringSlice(cmd, &inputs.AllowedWebOrigins, nil)
	appLogoutURLs.RegisterStringSlice(cmd, &inputs.AllowedLogoutURLs, nil)
	appAuthMethod.RegisterString(cmd, &inputs.AuthMethod, "")
	appGrants.RegisterStringSlice(cmd, &inputs.Grants, nil)
	revealSecrets.RegisterBool(cmd, &inputs.RevealSecrets, false)

	return cmd
}

func updateAppCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID                string
		Name              string
		Type              string
		Description       string
		Callbacks         []string
		AllowedOrigins    []string
		AllowedWebOrigins []string
		AllowedLogoutURLs []string
		AuthMethod        string
		Grants            []string
		RevealSecrets     bool
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update an application",
		Long: "Update an application.\n\n" +
			"To update interactively, use `auth0 apps update` with no arguments.\n\n" +
			"To update non-interactively, supply the application id, name, type and other information you " +
			"might want to change through the available flags.",
		Example: `  auth0 apps update
  auth0 apps update <app-id> --name myapp
  auth0 apps update <app-id> --name myapp --description <description>
  auth0 apps update <app-id> --name myapp --description <description> --type [native|spa|regular|m2m]
  auth0 apps update <app-id> --name myapp --description <description> --type [native|spa|regular|m2m] --reveal-secrets
  auth0 apps update <app-id> -n myapp -d <description> -t [native|spa|regular|m2m] -r --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var current *management.Client

			if len(args) == 0 {
				err := appID.Pick(cmd, &inputs.ID, cli.appPickerOptions())
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			// Load app by id
			if err := ansi.Waiting(func() error {
				var err error
				current, err = cli.api.Client.Read(inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("Unable to load application: %w", err)
			}

			// Prompt for app name
			if err := appName.AskU(cmd, &inputs.Name, current.Name); err != nil {
				return err
			}

			// Prompt for app type
			if err := appType.SelectU(cmd, &inputs.Type, appTypeOptions, typeFor(current.AppType)); err != nil {
				return err
			}

			appIsM2M := apiTypeFor(inputs.Type) == appTypeNonInteractive
			appIsNative := apiTypeFor(inputs.Type) == appTypeNative
			appIsSPA := apiTypeFor(inputs.Type) == appTypeSPA

			// Prompt for callback URLs if app is not m2m
			if !appIsM2M {
				var defaultValue string

				if !appIsNative {
					defaultValue = appDefaultURL
				}

				if len(current.GetCallbacks()) > 0 {
					defaultValue = stringSliceToCommaSeparatedString(current.GetCallbacks())
				}

				if err := appCallbacks.AskManyU(cmd, &inputs.Callbacks, &defaultValue); err != nil {
					return err
				}
			}

			// Prompt for logout URLs if app is not m2m
			if !appIsM2M {
				var defaultValue string

				if !appIsNative {
					defaultValue = appDefaultURL
				}

				if len(current.GetAllowedLogoutURLs()) > 0 {
					defaultValue = stringSliceToCommaSeparatedString(current.GetAllowedLogoutURLs())
				}

				if err := appLogoutURLs.AskManyU(cmd, &inputs.AllowedLogoutURLs, &defaultValue); err != nil {
					return err
				}
			}

			// Prompt for allowed origins URLs if app is SPA
			if appIsSPA {
				defaultValue := appDefaultURL

				if len(current.GetAllowedOrigins()) > 0 {
					defaultValue = stringSliceToCommaSeparatedString(current.GetAllowedOrigins())
				}

				if err := appOrigins.AskManyU(cmd, &inputs.AllowedOrigins, &defaultValue); err != nil {
					return err
				}
			}

			// Prompt for allowed web origins URLs if app is SPA
			if appIsSPA {
				defaultValue := appDefaultURL

				if len(current.GetWebOrigins()) > 0 {
					defaultValue = stringSliceToCommaSeparatedString(current.GetWebOrigins())
				}

				if err := appWebOrigins.AskManyU(cmd, &inputs.AllowedWebOrigins, &defaultValue); err != nil {
					return err
				}
			}

			// Load updated values into a fresh app instance
			a := &management.Client{}

			if len(inputs.Name) == 0 {
				a.Name = current.Name
			} else {
				a.Name = &inputs.Name
			}

			if len(inputs.Description) == 0 {
				a.Description = current.Description
			} else {
				a.Description = &inputs.Description
			}

			if len(inputs.Type) == 0 {
				a.AppType = current.AppType
			} else {
				a.AppType = auth0.String(apiTypeFor(inputs.Type))
			}

			if len(inputs.Callbacks) == 0 {
				a.Callbacks = current.Callbacks
			} else {
				a.Callbacks = &inputs.Callbacks
			}

			if len(inputs.AllowedOrigins) == 0 {
				a.AllowedOrigins = current.AllowedOrigins
			} else {
				a.AllowedOrigins = &inputs.AllowedOrigins
			}

			if len(inputs.AllowedWebOrigins) == 0 {
				a.WebOrigins = current.WebOrigins
			} else {
				a.WebOrigins = &inputs.AllowedWebOrigins
			}

			if len(inputs.AllowedLogoutURLs) == 0 {
				a.AllowedLogoutURLs = current.AllowedLogoutURLs
			} else {
				a.AllowedLogoutURLs = &inputs.AllowedLogoutURLs
			}

			if len(inputs.AuthMethod) == 0 {
				a.TokenEndpointAuthMethod = current.TokenEndpointAuthMethod
			} else {
				a.TokenEndpointAuthMethod = apiAuthMethodFor(inputs.AuthMethod)
			}

			if len(inputs.Grants) == 0 {
				a.GrantTypes = current.GrantTypes
			} else {
				a.GrantTypes = apiGrantsFor(inputs.Grants)
			}

			// Update app
			if err := ansi.Waiting(func() error {
				return cli.api.Client.Update(inputs.ID, a)
			}); err != nil {
				return fmt.Errorf("Unable to update application %v: %v", inputs.ID, err)
			}

			// Render result
			cli.renderer.ApplicationUpdate(a, inputs.RevealSecrets)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	appName.RegisterStringU(cmd, &inputs.Name, "")
	appType.RegisterStringU(cmd, &inputs.Type, "")
	appDescription.RegisterStringU(cmd, &inputs.Description, "")
	appCallbacks.RegisterStringSliceU(cmd, &inputs.Callbacks, nil)
	appOrigins.RegisterStringSliceU(cmd, &inputs.AllowedOrigins, nil)
	appWebOrigins.RegisterStringSliceU(cmd, &inputs.AllowedWebOrigins, nil)
	appLogoutURLs.RegisterStringSliceU(cmd, &inputs.AllowedLogoutURLs, nil)
	appAuthMethod.RegisterStringU(cmd, &inputs.AuthMethod, "")
	appGrants.RegisterStringSliceU(cmd, &inputs.Grants, nil)
	revealSecrets.RegisterBool(cmd, &inputs.RevealSecrets, false)

	return cmd
}

func openAppCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "open",
		Args:  cobra.MaximumNArgs(1),
		Short: "Open the settings page of an application",
		Long:  "Open an application's settings page in the Auth0 Dashboard.",
		Example: `  auth0 apps open
  auth0 apps open <app-id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := appID.Pick(cmd, &inputs.ID, cli.appPickerOptions())
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			openManageURL(cli, cli.config.DefaultTenant, formatAppSettingsPath(inputs.ID))
			return nil
		},
	}

	return cmd
}

func formatAppSettingsPath(id string) string {
	if len(id) == 0 {
		return ""
	}
	return fmt.Sprintf("applications/%s/settings", id)
}

func apiTypeFor(v string) string {
	switch strings.ToLower(v) {
	case "native":
		return appTypeNative
	case "spa", "single page web application":
		return appTypeSPA
	case "regular", "regular web application":
		return appTypeRegularWeb
	case "m2m", "machine to machine":
		return appTypeNonInteractive
	default:
		return v
	}
}

func apiAuthMethodFor(v string) *string {
	switch strings.ToLower(v) {
	case "none":
		return auth0.String("none")
	case "post":
		return auth0.String("client_secret_post")
	case "basic":
		return auth0.String("client_secret_basic")
	default:
		return nil
	}
}

func apiDefaultAuthMethodFor(t string) *string {
	switch apiTypeFor(strings.ToLower(t)) {
	case appTypeNative, appTypeSPA:
		return auth0.String("none")
	default:
		return nil
	}
}

func apiGrantsFor(s []string) *[]string {
	res := make([]string, len(s))

	for i, v := range s {
		switch strings.ToLower(v) {
		case "authorization-code", "code":
			res[i] = "authorization_code"
		case "implicit":
			res[i] = "implicit"
		case "refresh-token":
			res[i] = "refresh_token"
		case "client-credentials", "credentials":
			res[i] = "client_credentials"
		case "password":
			res[i] = "password"
		case "password-realm":
			res[i] = "http://auth0.com/oauth/grant-type/password-realm"
		case "mfa-oob":
			res[i] = "http://auth0.com/oauth/grant-type/mfa-oob"
		case "mfa-otp":
			res[i] = "http://auth0.com/oauth/grant-type/mfa-otp"
		case "mfa-recovery-code":
			res[i] = "http://auth0.com/oauth/grant-type/mfa-recovery-code"
		case "device-code":
			res[i] = "urn:ietf:params:oauth:grant-type:device_code"
		default:
		}
	}

	return &res
}

func apiDefaultGrantsFor(t string) *[]string {
	switch apiTypeFor(strings.ToLower(t)) {
	case appTypeNative:
		return &[]string{"implicit", "authorization_code", "refresh_token"}
	case appTypeSPA:
		return &[]string{"implicit", "authorization_code", "refresh_token"}
	case appTypeRegularWeb:
		return &[]string{"implicit", "authorization_code", "refresh_token", "client_credentials"}
	case appTypeNonInteractive:
		return &[]string{"client_credentials"}
	default:
		return nil
	}
}

func typeFor(s *string) *string {
	switch apiTypeFor(strings.ToLower(auth0.StringValue(s))) {
	case appTypeNative:
		return auth0.String("Native")
	case appTypeSPA:
		return auth0.String("Single Page Web Application")
	case appTypeRegularWeb:
		return auth0.String("Regular Web Application")
	case appTypeNonInteractive:
		return auth0.String("Machine to Machine")
	default:
		return nil
	}
}

func commaSeparatedStringToSlice(s string) []string {
	joined := strings.Join(strings.Fields(s), "")
	if len(joined) > 0 {
		return strings.Split(joined, ",")
	}
	return []string{}
}

func stringSliceToCommaSeparatedString(s []string) string {
	return strings.Join(s, ", ")
}

func stringSliceToPtr(s []string) *[]string {
	if s == nil {
		return nil
	}
	return &s
}

func (c *cli) appPickerOptions(requestOpts ...management.RequestOption) pickerOptionsFunc {
	requestOpts = append(requestOpts, management.Parameter("is_global", "false"))

	return func() (pickerOptions, error) {
		clientList, err := c.api.Client.List(requestOpts...)
		if err != nil {
			return nil, err
		}

		tenant, err := c.getTenant()
		if err != nil {
			return nil, err
		}

		var priorityOpts, opts pickerOptions
		for _, client := range clientList.Clients {
			value := client.GetClientID()
			label := fmt.Sprintf(
				"%s [%s] %s",
				client.GetName(),
				display.ApplyColorToFriendlyAppType(display.FriendlyAppType(client.GetAppType())),
				ansi.Faint("("+value+")"),
			)
			option := pickerOption{value: value, label: label}

			if tenant.DefaultAppID == client.GetClientID() {
				priorityOpts = append(priorityOpts, option)
			} else {
				opts = append(opts, option)
			}
		}

		if len(opts)+len(priorityOpts) == 0 {
			return nil, errNoApps
		}

		return append(priorityOpts, opts...), nil
	}
}
