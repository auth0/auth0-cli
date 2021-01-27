package cli

import (
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
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
			var list *management.ConnectionList
			err := ansi.Spinner("Loading connections", func() error {
				var err error
				list, err = cli.api.Connection.List()
				return err
			})

			if err != nil {
				return err
			}

			cli.renderer.ConnectionList(list.Connections)
			return nil
		},
	}

	return cmd
}
