package cli

import (
	"github.com/spf13/cobra"
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
			list, err := cli.api.Client.List()
			if err != nil {
				return err
			}

			cli.renderer.ClientList(list.Clients)
			return nil
		},
	}

	return cmd
}
