package cli

import (
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

func clientsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients",
		Short: "manage resources for clients.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listClientsCmd(cli))

	return cmd
}

func listClientsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lists your existing clients",
		Long: `$ auth0 client list
Lists your existing clients. To create one try:

    $ auth0 client create
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var list *management.ClientList

			err := ansi.Spinner("Listing clients", func() (err error) {
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
