package cli

import (
	"github.com/spf13/cobra"
)

func connectionsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connections",
		Short: "manage resources for connections.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listConnectionsCmd(cli))

	return cmd
}

func listConnectionsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lists your existing connections",
		Long: `$ auth0 connections list
Lists your existing connections. To create one try:

    $ auth0 connections create
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			list, err := cli.api.Connection.List()
			if err != nil {
				return err
			}

			cli.renderer.ConnectionList(list.Connections)
			return nil
		},
	}

	return cmd
}
