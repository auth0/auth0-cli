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

const (
	appID = "id"
)

var (
	appName = Flag{
		Name:       "Name",
		LongForm:   "name",
		ShortForm:  "n",
		Help:       "Name of the application.",
		IsRequired: true,
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
		Name:       "Callback URLs",
		LongForm:   "callbacks",
		ShortForm:  "c",
		Help:       "After the user authenticates we will only call back to any of these URLs. You can specify multiple valid URLs by comma-separating them (typically to handle different environments like QA or testing). Make sure to specify the protocol (https://) otherwise the callback may fail in some cases. With the exception of custom URI schemes for native apps, all callbacks should use protocol https://.",
		IsRequired: false,
	}
	appOrigins = Flag{
		Name:       "Allowed Origin URLs",
		LongForm:   "origins",
		ShortForm:  "o",
		Help:       "Comma-separated list of URLs allowed to make requests from JavaScript to Auth0 API (typically used with CORS). By default, all your callback URLs will be allowed. This field allows you to enter other origins if necessary. You can also use wildcards at the subdomain level (e.g., https://*.contoso.com). Query strings and hash information are not taken into account when validating these URLs.",
		IsRequired: false,
	}
	appWebOrigins = Flag{
		Name:       "Allowed Web Origin URLs",
		LongForm:   "web-origins",
		ShortForm:  "w",
		Help:       "Comma-separated list of allowed origins for use with Cross-Origin Authentication, Device Flow, and web message response mode.",
		IsRequired: false,
	}
	appLogoutURLs = Flag{
		Name:       "Allowed Logout URLs",
		LongForm:   "logout-urls",
		ShortForm:  "l",
		Help:       "Comma-separated list of URLs that are valid to redirect to after logout from Auth0. Wildcards are allowed for subdomains.",
		IsRequired: false,
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
)

func appsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "apps",
		Short:   "Manage resources for applications",
		Aliases: []string{"clients"},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listAppsCmd(cli))
	cmd.AddCommand(showAppCmd(cli))
	cmd.AddCommand(createAppCmd(cli))
	cmd.AddCommand(updateAppCmd(cli))
	cmd.AddCommand(deleteAppCmd(cli))

	return cmd
}

func listAppsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List your applications",
		Long: `auth0 apps list
Lists your existing applications. To create one try:

    auth0 apps create
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var list *management.ClientList
			err := ansi.Spinner("Loading applications", func() error {
				var err error
				list, err = cli.api.Client.List()
				return err
			})

			if err != nil {
				return fmt.Errorf("An unexpected error occurred: %w", err)
			}

			cli.renderer.ApplicationList(list.Clients)
			return nil
		},
	}

	return cmd
}

func showAppCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show an application",
		Long: `Show an application:

auth0 apps show <id>
`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if canPrompt(cmd) {
					input := prompt.TextInput(appID, "Client Id:", "Id of the application.", true)

					if err := prompt.AskOne(input, &inputs); err != nil {
						return fmt.Errorf("An unexpected error occurred: %w", err)
					}
				} else {
					return errors.New("Please provide an application Id")
				}
			} else {
				inputs.ID = args[0]
			}

			a := &management.Client{ClientID: &inputs.ID}

			err := ansi.Spinner("Loading application", func() error {
				var err error
				a, err = cli.api.Client.Read(inputs.ID)
				return err
			})

			if err != nil {
				return fmt.Errorf("Unable to load application. The Id %v specified doesn't exist", inputs.ID)
			}

			revealClientSecret := auth0.StringValue(a.AppType) != "native" && auth0.StringValue(a.AppType) != "spa"
			cli.renderer.ApplicationShow(a, revealClientSecret)
			return nil
		},
	}

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
		Long: `Delete an application:

auth0 apps delete <id>
`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if canPrompt(cmd) {
					input := prompt.TextInput(appID, "Client Id:", "Id of the application.", true)

					if err := prompt.AskOne(input, &inputs); err != nil {
						return fmt.Errorf("An unexpected error occurred: %w", err)
					}
				} else {
					return errors.New("Please provide an application Id")
				}
			} else {
				inputs.ID = args[0]
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			return ansi.Spinner("Deleting application", func() error {
				return cli.api.Client.Delete(inputs.ID)
			})
		},
	}

	return cmd
}

func createAppCmd(cli *cli) *cobra.Command {
	var flags struct {
		Name              string
		Type              string
		Description       string
		Callbacks         []string
		AllowedOrigins    []string
		AllowedWebOrigins []string
		AllowedLogoutURLs []string
		AuthMethod        string
		Grants            []string
	}
	var oidcConformant = true
	var algorithm = "RS256"

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new application",
		Long: `Create a new application:

auth0 apps create --name myapp --type [native|spa|regular|m2m]
`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := appName.Ask(cmd, &flags.Name); err != nil {
				return err
			}

			if err := appType.Select(cmd, &flags.Type, appTypeOptions); err != nil {
				return err
			}

			if err := appDescription.Ask(cmd, &flags.Description); err != nil {
				return err
			}

			a := &management.Client{
				Name:                    &flags.Name,
				Description:             &flags.Description,
				AppType:                 auth0.String(apiTypeFor(flags.Type)),
				Callbacks:               stringToInterfaceSlice(flags.Callbacks),
				AllowedOrigins:          stringToInterfaceSlice(flags.AllowedOrigins),
				WebOrigins:              stringToInterfaceSlice(flags.AllowedWebOrigins),
				AllowedLogoutURLs:       stringToInterfaceSlice(flags.AllowedLogoutURLs),
				TokenEndpointAuthMethod: apiAuthMethodFor(flags.AuthMethod),
				OIDCConformant:          &oidcConformant,
				JWTConfiguration:        &management.ClientJWTConfiguration{Algorithm: &algorithm},
			}

			if len(flags.Grants) == 0 {
				a.GrantTypes = apiDefaultGrantsFor(flags.Type)
			} else {
				a.GrantTypes = apiGrantsFor(flags.Grants)
			}

			err := ansi.Spinner("Creating application", func() error {
				return cli.api.Client.Create(a)
			})

			if err != nil {
				return fmt.Errorf("Unable to create application: %w", err)
			}

			// note: a is populated with the rest of the client fields by the API during creation.
			revealClientSecret := auth0.StringValue(a.AppType) != "native" && auth0.StringValue(a.AppType) != "spa"
			cli.renderer.ApplicationCreate(a, revealClientSecret)

			return nil
		},
	}

	appName.RegisterString(cmd, &flags.Name, "")
	appType.RegisterString(cmd, &flags.Type, "")
	appDescription.RegisterString(cmd, &flags.Description, "")
	appCallbacks.RegisterStringSlice(cmd, &flags.Callbacks, nil)
	appOrigins.RegisterStringSlice(cmd, &flags.AllowedOrigins, nil)
	appWebOrigins.RegisterStringSlice(cmd, &flags.AllowedWebOrigins, nil)
	appLogoutURLs.RegisterStringSlice(cmd, &flags.AllowedLogoutURLs, nil)
	appAuthMethod.RegisterString(cmd, &flags.AuthMethod, "")
	appGrants.RegisterStringSlice(cmd, &flags.Grants, nil)

	return cmd
}

func updateAppCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID                string
		Name              string
		Type              string
		Description       string
		Callbacks         []string
		CallbacksString   string
		AllowedOrigins    []string
		AllowedWebOrigins []string
		AllowedLogoutURLs []string
		AuthMethod        string
		Grants            []string
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update an application",
		Long: `Update an application:

auth0 apps update <id> --name myapp --type [native|spa|regular|m2m]
`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if canPrompt(cmd) {
					input := prompt.TextInput(appID, "Client Id:", "Id of the application.", true)

					if err := prompt.AskOne(input, &inputs); err != nil {
						return fmt.Errorf("An unexpected error occurred: %w", err)
					}
				} else {
					return errors.New("Please provide an application Id")
				}
			} else {
				inputs.ID = args[0]
			}

			if err := appName.AskU(cmd, &inputs.Name); err != nil {
				return err
			}

			if err := appType.SelectU(cmd, &inputs.Type, appTypeOptions); err != nil {
				return err
			}

			if err := appDescription.AskU(cmd, &inputs.Description); err != nil {
				return err
			}

			if err := appCallbacks.AskU(cmd, &inputs.CallbacksString); err != nil {
				return err
			}

			a := &management.Client{}

			err := ansi.Spinner("Updating application", func() error {
				current, err := cli.api.Client.Read(inputs.ID)

				if err != nil {
					return fmt.Errorf("Unable to load application. The Id %v specified doesn't exist", inputs.ID)
				}

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
					if len(inputs.CallbacksString) == 0 {
						a.Callbacks = current.Callbacks
					} else {
						a.Callbacks = stringToInterfaceSlice(commaSeparatedStringToSlice(inputs.CallbacksString))
					}
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

				return cli.api.Client.Update(inputs.ID, a)
			})

			if err != nil {
				return fmt.Errorf("Unable to update application %v: %v", inputs.ID, err)
			}

			revealClientSecret := auth0.StringValue(a.AppType) != "native" && auth0.StringValue(a.AppType) != "spa"
			cli.renderer.ApplicationUpdate(a, revealClientSecret)

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

	return cmd
}

func apiTypeFor(v string) string {
	switch strings.ToLower(v) {
	case "native":
		return "native"
	case "spa", "single page web application":
		return "spa"
	case "regular", "regular web application":
		return "regular_web"
	case "m2m", "machine to machine":
		return "non_interactive"

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

func apiGrantsFor(s []string) []interface{} {
	res := make([]interface{}, len(s))

	for i, v := range s {
		switch strings.ToLower(v) {
		case "authorization-code", "code":
			res[i] = auth0.String("authorization_code")
		case "implicit":
			res[i] = auth0.String("implicit")
		case "refresh-token":
			res[i] = auth0.String("refresh_token")
		case "client-credentials", "credentials":
			res[i] = auth0.String("client_credentials")
		case "password":
			res[i] = auth0.String("password")
		case "password-realm":
			res[i] = auth0.String("http://auth0.com/oauth/grant-type/password-realm")
		case "mfa-oob":
			res[i] = auth0.String("http://auth0.com/oauth/grant-type/mfa-oob")
		case "mfa-otp":
			res[i] = auth0.String("http://auth0.com/oauth/grant-type/mfa-otp")
		case "mfa-recovery-code":
			res[i] = auth0.String("http://auth0.com/oauth/grant-type/mfa-recovery-code")
		case "device-code":
			res[i] = auth0.String("urn:ietf:params:oauth:grant-type:device_code")
		default:
		}
	}

	return res
}

func apiDefaultGrantsFor(t string) []interface{} {
	switch apiTypeFor(strings.ToLower(t)) {
	case "native":
		return stringToInterfaceSlice([]string{"implicit", "authorization_code", "refresh_token"})
	case "spa":
		return stringToInterfaceSlice([]string{"implicit", "authorization_code", "refresh_token"})
	case "regular_web":
		return stringToInterfaceSlice([]string{"implicit", "authorization_code", "refresh_token", "client_credentials"})
	case "non_interactive":
		return stringToInterfaceSlice([]string{"client_credentials"})
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
	return strings.Split(strings.Join(strings.Fields(s), ""), ",")
}

func stringToInterfaceSlice(s []string) []interface{} {
	var result []interface{} = make([]interface{}, len(s))
	for i, d := range s {
		result[i] = d
	}
	return result
}
