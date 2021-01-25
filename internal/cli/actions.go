package cli

import (
	"github.com/spf13/cobra"
)

func actionsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "actions",
		Short: "manage resources for actions.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listActionsCmd(cli))

	return cmd
}

func listActionsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lists your existing actions",
		Long: `$ auth0 actions list
Lists your existing actions. To create one try:

    $ auth0 actions create
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			list, err := cli.api.Action.List()
			if err != nil {
				return err
			}

			cli.renderer.ActionList(list.Actions)
			return nil
		},
	}

	return cmd
}
