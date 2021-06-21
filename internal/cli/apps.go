package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

// errNoApps signifies no applications exist in a tenant
var errNoApps = errors.New("there are currently no applications")

const (
	appTypeNative         = "native"
	appTypeSPA            = "spa"
	appTypeRegularWeb     = "regular_web"
	appTypeNonInteractive = "non_interactive"
	appDefaultURL         = "http://localhost:3000"
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
	reveal = Flag{
		Name:       "Reveal",
		LongForm:   "reveal",
		ShortForm:  "r",
		Help:       "Display the Client Secret as part of the command output.",
	}
)

func appsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "apps",
		Short:   "Manage resources for applications",
		Long:    "Manage resources for applications.",
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
		Use:     "use",
		Args:    cobra.MaximumNArgs(1),
		Short:   "Choose a default application for the Auth0 CLI",
		Long:    "Specify your preferred application for interaction with the Auth0 CLI.",
		Example: "auth0 apps use <client-id>",
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.None {
				inputs.ID = ""
			} else {
				if len(args) == 0 {
					err := appID.Pick(cmd, &inputs.ID, cli.appPickerOptions)
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
				cli.renderer.Infof("%s You might wanna try 'auth0 quickstarts download %s'", ansi.Faint("Hint:"), inputs.ID)
			}

			return nil
		},
	}

	appNone.RegisterBool(cmd, &inputs.None, false)
	return cmd
}

func listAppsCmd(cli *cli) *cobra.Command {
	var inputs struct{
		Reveal bool
	}
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your applications",
		Long: `List your existing applications. To create one try:
auth0 apps create`,
		Example: `auth0 apps list
auth0 apps ls`,
		RunE: func(cmd *cobra.Command, args []string) error {

			var list *management.ClientList

			if err := ansi.Waiting(func() error {
				var err error
				list, err = cli.api.Client.List()
				return err
			}); err != nil {
				return fmt.Errorf("An unexpected error occurred: %w", err)
			}

			cli.renderer.ApplicationList(list.Clients, inputs.Reveal)
			return nil
		},
	}

	reveal.RegisterBool(cmd, &inputs.Reveal, false)

	return cmd
}

func showAppCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID     string
		Reveal bool
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show an application",
		Long:  "Show an application.",
		Example: `auth0 apps show 
auth0 apps show <id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := appID.Pick(cmd, &inputs.ID, cli.appPickerOptions)
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

			cli.renderer.ApplicationShow(a, inputs.Reveal)
			return nil
		},
	}

	reveal.RegisterBool(cmd, &inputs.Reveal, false)

	return cmd
}

func deleteAppCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "delete",
		Args:  cobra.MaximumNArgs(1),
		Short: "Delete an application",
		Long:  "Delete an application.",
		Example: `auth0 apps delete 
auth0 apps delete <id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := appID.Pick(cmd, &inputs.ID, cli.appPickerOptions)
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
		Reveal            bool
	}
	var oidcConformant = true
	var algorithm = "RS256"

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new application",
		Long:  "Create a new application.",
		Example: `auth0 apps create 
auth0 apps create --name myapp 
auth0 apps create -n myapp --type [native|spa|regular|m2m]
auth0 apps create -n myapp -t [native|spa|regular|m2m] --description <description>`,
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
			if !appIsM2M && !appCallbacks.IsSet(cmd) {
				var defaultValue string

				if !appIsNative {
					defaultValue = appDefaultURL
				}

				if err := appCallbacks.AskMany(cmd, &inputs.Callbacks, &defaultValue); err != nil {
					return err
				}
			}

			// Prompt for logout URLs if app is not m2m
			if !appIsM2M && !appLogoutURLs.IsSet(cmd) {
				var defaultValue string

				if !appIsNative {
					defaultValue = appDefaultURL
				}

				if err := appLogoutURLs.AskMany(cmd, &inputs.AllowedLogoutURLs, &defaultValue); err != nil {
					return err
				}
			}

			// Prompt for allowed origins URLs if app is SPA
			if appIsSPA && !appOrigins.IsSet(cmd) {
				defaultValue := appDefaultURL

				if err := appOrigins.AskMany(cmd, &inputs.AllowedOrigins, &defaultValue); err != nil {
					return err
				}
			}

			// Prompt for allowed web origins URLs if app is SPA
			if appIsSPA && !appWebOrigins.IsSet(cmd) {
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
				Callbacks:         stringToInterfaceSlice(inputs.Callbacks),
				AllowedOrigins:    stringToInterfaceSlice(inputs.AllowedOrigins),
				WebOrigins:        stringToInterfaceSlice(inputs.AllowedWebOrigins),
				AllowedLogoutURLs: stringToInterfaceSlice(inputs.AllowedLogoutURLs),
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
			cli.renderer.ApplicationCreate(a, inputs.Reveal)

			return nil
		},
	}

	appName.RegisterString(cmd, &inputs.Name, "")
	appType.RegisterString(cmd, &inputs.Type, "")
	appDescription.RegisterString(cmd, &inputs.Description, "")
	appCallbacks.RegisterStringSlice(cmd, &inputs.Callbacks, nil)
	appOrigins.RegisterStringSlice(cmd, &inputs.AllowedOrigins, nil)
	appWebOrigins.RegisterStringSlice(cmd, &inputs.AllowedWebOrigins, nil)
	appLogoutURLs.RegisterStringSlice(cmd, &inputs.AllowedLogoutURLs, nil)
	appAuthMethod.RegisterString(cmd, &inputs.AuthMethod, "")
	appGrants.RegisterStringSlice(cmd, &inputs.Grants, nil)
	reveal.RegisterBool(cmd, &inputs.Reveal, false)

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
		Reveal            bool
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update an application",
		Long:  "Update an application.",
		Example: `auth0 apps update <id> 
auth0 apps update <id> --name myapp 
auth0 apps update <id> -n myapp --type [native|spa|regular|m2m]`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var current *management.Client

			if len(args) == 0 {
				err := appID.Pick(cmd, &inputs.ID, cli.appPickerOptions)
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
			if !appIsM2M && !appCallbacks.IsSet(cmd) {
				var defaultValue string

				if !appIsNative {
					defaultValue = appDefaultURL
				}

				if len(current.Callbacks) > 0 {
					defaultValue = stringSliceToCommaSeparatedString(interfaceToStringSlice(current.Callbacks))
				}

				if err := appCallbacks.AskManyU(cmd, &inputs.Callbacks, &defaultValue); err != nil {
					return err
				}
			}

			// Prompt for logout URLs if app is not m2m
			if !appIsM2M && !appLogoutURLs.IsSet(cmd) {
				var defaultValue string

				if !appIsNative {
					defaultValue = appDefaultURL
				}

				if len(current.AllowedLogoutURLs) > 0 {
					defaultValue = stringSliceToCommaSeparatedString(interfaceToStringSlice(current.AllowedLogoutURLs))
				}

				if err := appLogoutURLs.AskManyU(cmd, &inputs.AllowedLogoutURLs, &defaultValue); err != nil {
					return err
				}
			}

			// Prompt for allowed origins URLs if app is SPA
			if appIsSPA && !appOrigins.IsSet(cmd) {
				defaultValue := appDefaultURL

				if len(current.AllowedOrigins) > 0 {
					defaultValue = stringSliceToCommaSeparatedString(interfaceToStringSlice(current.AllowedOrigins))
				}

				if err := appOrigins.AskManyU(cmd, &inputs.AllowedOrigins, &defaultValue); err != nil {
					return err
				}
			}

			// Prompt for allowed web origins URLs if app is SPA
			if appIsSPA && !appWebOrigins.IsSet(cmd) {
				defaultValue := appDefaultURL

				if len(current.WebOrigins) > 0 {
					defaultValue = stringSliceToCommaSeparatedString(interfaceToStringSlice(current.WebOrigins))
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
				a.Callbacks = stringToInterfaceSlice(inputs.Callbacks)
			}

			if len(inputs.AllowedOrigins) == 0 {
				a.AllowedOrigins = current.AllowedOrigins
			} else {
				a.AllowedOrigins = stringToInterfaceSlice(inputs.AllowedOrigins)
			}

			if len(inputs.AllowedWebOrigins) == 0 {
				a.WebOrigins = current.WebOrigins
			} else {
				a.WebOrigins = stringToInterfaceSlice(inputs.AllowedWebOrigins)
			}

			if len(inputs.AllowedLogoutURLs) == 0 {
				a.AllowedLogoutURLs = current.AllowedLogoutURLs
			} else {
				a.AllowedLogoutURLs = stringToInterfaceSlice(inputs.AllowedLogoutURLs)
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
			cli.renderer.ApplicationUpdate(a, inputs.Reveal)

			return nil
		},
	}

	appName.RegisterStringU(cmd, &inputs.Name, "")
	appType.RegisterStringU(cmd, &inputs.Type, "")
	appDescription.RegisterStringU(cmd, &inputs.Description, "")
	appCallbacks.RegisterStringSliceU(cmd, &inputs.Callbacks, nil)
	appOrigins.RegisterStringSliceU(cmd, &inputs.AllowedOrigins, nil)
	appWebOrigins.RegisterStringSliceU(cmd, &inputs.AllowedWebOrigins, nil)
	appLogoutURLs.RegisterStringSliceU(cmd, &inputs.AllowedLogoutURLs, nil)
	appAuthMethod.RegisterStringU(cmd, &inputs.AuthMethod, "")
	appGrants.RegisterStringSliceU(cmd, &inputs.Grants, nil)
	reveal.RegisterBool(cmd, &inputs.Reveal, false)
	return cmd
}

func openAppCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:     "open",
		Args:    cobra.MaximumNArgs(1),
		Short:   "Open application settings page in the Auth0 Dashboard",
		Long:    "Open application settings page in the Auth0 Dashboard.",
		Example: "auth0 apps open <id>",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := appID.Pick(cmd, &inputs.ID, cli.appPickerOptions)
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

func apiGrantsFor(s []string) []interface{} {
	res := make([]interface{}, len(s))

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

	return res
}

func apiDefaultGrantsFor(t string) []interface{} {
	switch apiTypeFor(strings.ToLower(t)) {
	case appTypeNative:
		return stringToInterfaceSlice([]string{"implicit", "authorization_code", "refresh_token"})
	case appTypeSPA:
		return stringToInterfaceSlice([]string{"implicit", "authorization_code", "refresh_token"})
	case appTypeRegularWeb:
		return stringToInterfaceSlice([]string{"implicit", "authorization_code", "refresh_token", "client_credentials"})
	case appTypeNonInteractive:
		return stringToInterfaceSlice([]string{"client_credentials"})
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

func urlsFor(s []interface{}) []string {
	res := make([]string, len(s))
	for i, v := range s {
		res[i] = fmt.Sprintf("%s", v)
	}
	return res
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

func stringToInterfaceSlice(s []string) []interface{} {
	var result []interface{} = make([]interface{}, len(s))
	for i, d := range s {
		result[i] = d
	}
	return result
}

func interfaceToStringSlice(s []interface{}) []string {
	var result []string = make([]string, len(s))
	for i, d := range s {
		if val, ok := d.(string); ok {
			result[i] = val
		}
	}
	return result
}

func (c *cli) appPickerOptions() (pickerOptions, error) {
	list, err := c.api.Client.List()
	if err != nil {
		return nil, err
	}

	tenant, err := c.getTenant()
	if err != nil {
		return nil, err
	}

	// NOTE(cyx): To keep the contract for this simple, we'll rely on the
	// implicit knowledge that the default value for the picker is the
	// first option. With that in mind, we'll use the state in
	// tenant.DefaultAppID to determine which should be chosen as the
	// default.
	var (
		priorityOpts, opts pickerOptions
	)
	for _, c := range list.Clients {
		// empty type means the default client that we shouldn't display.
		if c.GetAppType() == "" {
			continue
		}

		value := c.GetClientID()
		label := fmt.Sprintf("%s %s", c.GetName(), ansi.Faint("("+value+")"))
		opt := pickerOption{value: value, label: label}

		// check if this is currently the default application.
		if tenant.DefaultAppID == c.GetClientID() {
			priorityOpts = append(priorityOpts, opt)
		} else {
			opts = append(opts, opt)
		}
	}

	if len(opts)+len(priorityOpts) == 0 {
		return nil, errNoApps
	}

	return append(priorityOpts, opts...), nil
}
