package cli

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

func clientsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients",
		Short: "Manage resources for clients",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(clientsListCmd(cli))
	cmd.AddCommand(clientsCreateCmd(cli))
	cmd.AddCommand(clientsQuickstartCmd(cli))

	return cmd
}

func clientsListCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List your existing clients",
		Long: `auth0 client list
Lists your existing clients. To create one try:

    auth0 clients create
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var list *management.ClientList
			err := ansi.Spinner("Loading clients", func() error {
				var err error
				list, err = cli.api.Client.List()
				return err
			})

			if err != nil {
				return err
			}

			cli.renderer.ClientList(list.Clients)
			return nil
		},
	}

	return cmd
}

func clientsCreateCmd(cli *cli) *cobra.Command {
	var flags struct {
		Name                    string
		AppType                 string
		Description             string
		Callbacks               []string
		TokenEndpointAuthMethod string
	}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new client (also know as application)",
		Long: `Create a new client (or application):

auth0 clients create --name myapp --type [native|spa|regular|m2m]

- supported application type:
	`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// todo(jfatta) on non-interactive the cmd should fail on missing mandatory args (name, type)
			if !cmd.Flags().Changed("name") {
				qs := []*survey.Question{
					{
						Name: "Name",
						Prompt: &survey.Input{
							Message: "Name:",
							Default: "My App",
							Help:    "Name of the client (also known as application). You can change the application name later in the application settings.",
						},
					},
				}

				err := survey.Ask(qs, &flags)
				if err != nil {
					return err
				}
			}

			if !cmd.Flags().Changed("type") {
				qs := []*survey.Question{
					{
						Name: "AppType",
						Prompt: &survey.Select{
							Message: "Type:",
							Help: "\n- Native: Mobile, desktop, CLI and smart device apps running natively." +
								"\n- Single Page Web Application: A JavaScript front-end app that uses an API." +
								"\n- Regular Web Application: Traditional web app using redirects." +
								"\n- Machine To Machine: CLIs, daemons or services running on your backend.",
							Options: []string{"Native", "Single Page Web Application", "Regular Web Application", "Machine to Machine"},
						},
					},
				}
				err := survey.Ask(qs, &flags)
				if err != nil {
					return err
				}
			}

			if !cmd.Flags().Changed("description") {
				qs := []*survey.Question{
					{
						Name: "Description",
						Prompt: &survey.Input{
							Message: "Description:",
							Help:    "A free text description of the application.",
						},
					},
				}
				err := survey.Ask(qs, &flags)
				if err != nil {
					return err
				}
			}

			c := &management.Client{
				Name:                    &flags.Name,
				Description:             &flags.Description,
				AppType:                 auth0.String(apiAppTypeFor(flags.AppType)),
				Callbacks:               apiCallbacksFor(flags.Callbacks),
				TokenEndpointAuthMethod: apiTokenEndpointAuthMethodFor(flags.TokenEndpointAuthMethod),
			}

			err := ansi.Spinner("Creating client", func() error {
				return cli.api.Client.Create(c)
			})

			if err != nil {
				return err
			}

			// note: c is populated with the rest of the client fields by the API during creation.
			revealClientSecret := auth0.StringValue(c.AppType) != "native" && auth0.StringValue(c.AppType) != "spa"
			cli.renderer.ClientCreate(c, revealClientSecret)
			return nil
		},
	}
	cmd.Flags().StringVarP(&flags.Name, "name", "n", "", "Name of the client.")
	cmd.Flags().StringVarP(&flags.AppType, "type", "t", "", "Type of the client:\n"+
		"- native: mobile, desktop, CLI and smart device apps running natively.\n"+
		"- spa (single page application): a JavaScript front-end app that uses an API.\n"+
		"- regular: Traditional web app using redirects.\n"+
		"- m2m (machine to machine): CLIs, daemons or services running on your backend.")
	cmd.Flags().StringVarP(&flags.Description, "description", "d", "", "A free text description of the application. Max character count is 140.")
	cmd.Flags().StringSliceVarP(&flags.Callbacks, "callbacks", "c", nil, "After the user authenticates we will only call back to any of these URLs. You can specify multiple valid URLs by comma-separating them (typically to handle different environments like QA or testing). Make sure to specify the protocol (https://) otherwise the callback may fail in some cases. With the exception of custom URI schemes for native clients, all callbacks should use protocol https://.")

	cmd.Flags().StringVar(&flags.TokenEndpointAuthMethod, "auth-method", "", "Defines the requested authentication method for the token endpoint. Possible values are 'None' (public application without a client secret), 'Post' (application uses HTTP POST parameters) or 'Basic' (application uses HTTP Basic).")

	return cmd
}

func apiAppTypeFor(v string) string {
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

func apiTokenEndpointAuthMethodFor(v string) *string {
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
