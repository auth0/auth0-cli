package cli

import (
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5"
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
		name        string
		appType     string
		description string
		reveal      bool
	}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new client (also know as application)",
		Long: `Creates a new client (or application):

auth0 clients create -n myapp -t spa

The application type can be:
- native: Mobile, desktop, CLI and smart device apps running natively.
- spa (single page application): A JavaScript front-end app that uses an API.
- regular: Traditional web app using redirects.
- m2m (machine to machine): CLIs, daemons or services running on your backend.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO(jfatta): depending on the app type, other client properties might be mandatory
			// check: create app dashboard
			c := &management.Client{
				Name:        &flags.name,
				Description: &flags.description,
				AppType:     auth0.String(apiAppTypeFor(flags.appType)),
			}

			err := ansi.Spinner("Creating action", func() error {
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
	cmd.Flags().StringVarP(&flags.appType, "type", "t", "", "Type of the client.")
	cmd.Flags().StringVarP(&flags.description, "description", "d", "", "Description of the client.")
	cmd.Flags().BoolVarP(&flags.reveal, "reveal", "r", false, "⚠️  Reveal the SECRET of the created client.")

	mustRequireFlags(cmd, "name", "type")

	return cmd
}

func apiAppTypeFor(v string) string {
	switch v {
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
