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
	appID          = "id"
	appName        = "name"
	appType        = "type"
	appDescription = "description"
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
				return err
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
				if shouldPrompt(cmd, appID) {
					input := prompt.TextInput(appID, "Id:", "Id of the application.", true)

					if err := prompt.AskOne(input, &inputs); err != nil {
						return err
					}
				} else {
					return errors.New("missing application id")
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
				return err
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
				if shouldPrompt(cmd, appID) {
					input := prompt.TextInput(appID, "Id:", "Id of the application.", true)

					if err := prompt.AskOne(input, &inputs); err != nil {
						return err
					}
				} else {
					return errors.New("missing application id")
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
		Name        string
		Type        string
		Description string
		Callbacks   []string
		AuthMethod  string
	}

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
			if shouldPrompt(cmd, appName) {
				input := prompt.TextInput(
					appName, "Name:",
					"Name of the application. You can change the name later in the application settings.",
					true)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			if shouldPrompt(cmd, appType) {
				input := prompt.SelectInput(
					appType,
					"Type:",
					"\n- Native: Mobile, desktop, CLI and smart device apps running natively."+
						"\n- Single Page Web Application: A JavaScript front-end app that uses an API."+
						"\n- Regular Web Application: Traditional web app using redirects."+
						"\n- Machine To Machine: CLIs, daemons or services running on your backend.",
					[]string{"Native", "Single Page Web Application", "Regular Web Application", "Machine to Machine"},
					true)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			if shouldPrompt(cmd, appDescription) {
				input := prompt.TextInput(appDescription, "Description:", "Description of the application.", false)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			a := &management.Client{
				Name:                    &flags.Name,
				Description:             &flags.Description,
				AppType:                 auth0.String(apiTypeFor(flags.Type)),
				Callbacks:               apiCallbacksFor(flags.Callbacks),
				TokenEndpointAuthMethod: apiAuthMethodFor(flags.AuthMethod),
			}

			err := ansi.Spinner("Creating application", func() error {
				return cli.api.Client.Create(a)
			})

			if err != nil {
				return err
			}

			// note: c is populated with the rest of the client fields by the API during creation.
			revealClientSecret := auth0.StringValue(a.AppType) != "native" && auth0.StringValue(a.AppType) != "spa"
			cli.renderer.ApplicationCreate(a, revealClientSecret)

			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.Name, "name", "n", "", "Name of the application.")
	cmd.Flags().StringVarP(&flags.Type, "type", "t", "", "Type of application:\n"+
		"- native: mobile, desktop, CLI and smart device apps running natively.\n"+
		"- spa (single page application): a JavaScript front-end app that uses an API.\n"+
		"- regular: Traditional web app using redirects.\n"+
		"- m2m (machine to machine): CLIs, daemons or services running on your backend.")
	cmd.Flags().StringVarP(&flags.Description, "description", "d", "", "Description of the application. Max character count is 140.")
	cmd.Flags().StringSliceVarP(&flags.Callbacks, "callbacks", "c", nil, "After the user authenticates we will only call back to any of these URLs. You can specify multiple valid URLs by comma-separating them (typically to handle different environments like QA or testing). Make sure to specify the protocol (https://) otherwise the callback may fail in some cases. With the exception of custom URI schemes for native apps, all callbacks should use protocol https://.")
	cmd.Flags().StringVar(&flags.AuthMethod, "auth-method", "", "Defines the requested authentication method for the token endpoint. Possible values are 'None' (public application without a client secret), 'Post' (application uses HTTP POST parameters) or 'Basic' (application uses HTTP Basic).")
	mustRequireFlags(cmd, appName, appType)

	return cmd
}

func updateAppCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID          string
		Name        string
		Type        string
		Description string
		Callbacks   []string
		AuthMethod  string
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
				if shouldPrompt(cmd, appID) {
					input := prompt.TextInput(appID, "Id:", "Id of the application.", true)

					if err := prompt.AskOne(input, &inputs); err != nil {
						return err
					}
				} else {
					return errors.New("missing application id")
				}
			} else {
				inputs.ID = args[0]
			}

			if shouldPrompt(cmd, appName) {
				input := prompt.TextInput(appName, "Name:", "Name of the application", true)

				if err := prompt.AskOne(input, &inputs); err != nil {
					return err
				}
			}

			if shouldPrompt(cmd, appType) {
				input := prompt.SelectInput(
					appType,
					"Type:",
					"\n- Native: Mobile, desktop, CLI and smart device apps running natively."+
						"\n- Single Page Web Application: A JavaScript front-end app that uses an API."+
						"\n- Regular Web Application: Traditional web app using redirects."+
						"\n- Machine To Machine: CLIs, daemons or services running on your backend.",
					[]string{"Native", "Single Page Web Application", "Regular Web Application", "Machine to Machine"},
					true)

				if err := prompt.AskOne(input, &inputs); err != nil {
					return err
				}
			}

			if shouldPrompt(cmd, appDescription) {
				input := prompt.TextInput(appDescription, "Description:", "Description of the application.", false)

				if err := prompt.AskOne(input, &inputs); err != nil {
					return err
				}
			}

			a := &management.Client{
				Name:                    &inputs.Name,
				Description:             &inputs.Description,
				AppType:                 auth0.String(apiTypeFor(inputs.Type)),
				Callbacks:               apiCallbacksFor(inputs.Callbacks),
				TokenEndpointAuthMethod: apiAuthMethodFor(inputs.AuthMethod),
			}

			err := ansi.Spinner("Updating application", func() error {
				return cli.api.Client.Update(inputs.ID, a)
			})

			if err != nil {
				return err
			}

			// note: c is populated with the rest of the client fields by the API during creation.
			revealClientSecret := auth0.StringValue(a.AppType) != "native" && auth0.StringValue(a.AppType) != "spa"
			cli.renderer.ApplicationUpdate(a, revealClientSecret)

			return nil
		},
	}

	cmd.Flags().StringVarP(&inputs.Name, "name", "n", "", "Name of the application.")
	cmd.Flags().StringVarP(&inputs.Type, "type", "t", "", "Type of application:\n"+
		"- native: mobile, desktop, CLI and smart device apps running natively.\n"+
		"- spa (single page application): a JavaScript front-end app that uses an API.\n"+
		"- regular: Traditional web app using redirects.\n"+
		"- m2m (machine to machine): CLIs, daemons or services running on your backend.")
	cmd.Flags().StringVarP(&inputs.Description, "description", "d", "", "Description of the application. Max character count is 140.")
	cmd.Flags().StringSliceVarP(&inputs.Callbacks, "callbacks", "c", nil, "After the user authenticates we will only call back to any of these URLs. You can specify multiple valid URLs by comma-separating them (typically to handle different environments like QA or testing). Make sure to specify the protocol (https://) otherwise the callback may fail in some cases. With the exception of custom URI schemes for native apps, all callbacks should use protocol https://.")
	cmd.Flags().StringVar(&inputs.AuthMethod, "auth-method", "", "Defines the requested authentication method for the token endpoint. Possible values are 'None' (public application without a client secret), 'Post' (application uses HTTP POST parameters) or 'Basic' (application uses HTTP Basic).")

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

func apiCallbacksFor(s []string) []interface{} {
	res := make([]interface{}, len(s))
	for i, v := range s {
		res[i] = v
	}
	return res
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

func callbacksFor(s []interface{}) []string {
	res := make([]string, len(s))
	for i, v := range s {
		res[i] = fmt.Sprintf("%s", v)
	}
	return res
}
