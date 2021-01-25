package cli

import (
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/validators"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5"
	"gopkg.in/auth0.v5/management"
)

func actionsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "actions",
		Short: "manage resources for actions.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listActionsCmd(cli))
	cmd.AddCommand(createActionCmd(cli))

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

func createActionCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Creates a new action",
		Long: `$ auth0 actions list
Lists your existing actions. To create one try:

    $ auth0 actions create my-action --trigger post-login
`,
		Args: func(cmd *cobra.Command, args []string) error {
			if err := validators.ExactArgs("name")(cmd, args); err != nil {
				return err
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			trigger, err := cmd.LocalFlags().GetString("trigger")
			if err != nil {
				return err
			}

			if err := validators.TriggerID(trigger); err != nil {
				return err
			}

			triggerID := management.TriggerID(trigger)
			triggers := []management.Trigger{
				{
					ID:      &triggerID,
					Version: auth0.String("v1"),
				},
			}

			action := &management.Action{
				Name:              auth0.String(args[0]),
				SupportedTriggers: &triggers,
			}

			err = ansi.Spinner("Creating action", func() error {
				return cli.api.Action.Create(action)
			})

			if err != nil {
				return err
			}

			cli.renderer.ActionCreate(action)
			return nil
		},
	}

	cmd.LocalFlags().StringP("trigger", "t", string(management.PostLogin), "Trigger type for action.")

	return cmd
}
