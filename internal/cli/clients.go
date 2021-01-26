package cli

import (
	"strings"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

func clientsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients",
		Short: "manage resources for clients.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(clientsListCmd(cli))
	cmd.AddCommand(clientsCreateCmd(cli))

	return cmd
}

func clientsListCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lists your existing clients",
		Long: `$ auth0 client list
Lists your existing clients. To create one try:

    $ auth0 clients create
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var list *management.ClientList
			err := ansi.Spinner("Getting clients", func() error {
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
		name                    string
		appType                 string
		description             string
		reveal                  bool
		callbacks               []string
		tokenEndpointAuthMethod string
	}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new client (also know as application)",
		Long: `Creates a new client (or application):

auth0 clients create --name myapp --type [native|spa|regular|m2m]

- supported application type:
	- native: mobile, desktop, CLI and smart device apps running natively.
	- spa (single page application): a JavaScript front-end app that uses an API.
	- regular: Traditional web app using redirects.
	- m2m (machine to machine): CLIs, daemons or services running on your backend.
`,
		RunE: func(cmd *cobra.Command, args []string) error {

			c := &management.Client{
				Name:                    &flags.name,
				Description:             &flags.description,
				AppType:                 auth0.String(apiAppTypeFor(flags.appType)),
				Callbacks:               apiCallbacksFor(flags.callbacks),
				TokenEndpointAuthMethod: apiTokenEndpointAuthMethodFor(flags.tokenEndpointAuthMethod),
			}

			err := ansi.Spinner("Creating client", func() error {
				return cli.api.Client.Create(c)
			})

			if err != nil {
				return err
			}

			// note: c is populated with the rest of the client fields by the API during creation.
			cli.renderer.ClientCreate(c, flags.reveal)
			return nil
		},
	}
	cmd.Flags().StringVarP(&flags.name, "name", "n", "", "Name of the client.")
	cmd.Flags().StringVarP(&flags.appType, "type", "t", "", "Type of the client: [native|spa|regular|m2m]")
	cmd.Flags().StringVarP(&flags.description, "description", "d", "", "A free text description of the application. Max character count is 140.")
	cmd.Flags().BoolVarP(&flags.reveal, "reveal", "r", false, "⚠️  Reveal the SECRET of the created client.")
	cmd.Flags().StringSliceVarP(&flags.callbacks, "callbacks", "c", nil, "After the user authenticates we will only call back to any of these URLs. You can specify multiple valid URLs by comma-separating them (typically to handle different environments like QA or testing). Make sure to specify the protocol (https://) otherwise the callback may fail in some cases. With the exception of custom URI schemes for native clients, all callbacks should use protocol https://.")

	cmd.Flags().StringVar(&flags.tokenEndpointAuthMethod, "auth-method", "", "Defines the requested authentication method for the token endpoint. Possible values are 'None' (public application without a client secret), 'Post' (application uses HTTP POST parameters) or 'Basic' (application uses HTTP Basic).")
	mustRequireFlags(cmd, "name", "type")

	return cmd
}

func apiAppTypeFor(v string) string {
	switch strings.ToLower(v) {
	case "native":
		return "native"
	case "spa":
		return "spa"
	case "regular":
		return "regular_web"
	case "m2m":
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
