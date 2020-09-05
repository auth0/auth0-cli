package cli

import (
	"fmt"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/config"
	"github.com/auth0/auth0-cli/internal/validators"
	"github.com/cyx/auth0/management"
	"github.com/spf13/cobra"
)

func actionsCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "actions",
		Short: "manage resources for actions.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listActionsCmd(cfg))
	cmd.AddCommand(createActionCmd(cfg))
	cmd.AddCommand(renameActionCmd(cfg))
	cmd.AddCommand(deleteActionCmd(cfg))
	cmd.AddCommand(deployActionCmd(cfg))

	return cmd
}

func listActionsCmd(cfg *config.Config) *cobra.Command {
	var flags struct {
		trigger string
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List existing actions",
		Long:  `List actions within a specific trigger.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			list, err := cfg.API.Action.List(management.WithTriggerID(management.TriggerID(flags.trigger)))
			if err != nil {
				return err
			}

			cfg.Renderer.ActionList(list.Actions)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.trigger,
		"trigger", "t", "", "Only list actions within this trigger.",
	)
	mustRequireFlags(cmd, "trigger")

	return cmd
}

func createActionCmd(cfg *config.Config) *cobra.Command {
	var flags struct {
		trigger string
	}

	cmd := &cobra.Command{
		Use:   "create <name>",
		Args:  validators.ExactArgs("<name>"),
		Short: "Create an action.",
		Long: `Creates an action, and generates a few files for working with actions:

- code.js       - function signature.
- testdata.json - sample payload for testing the action.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO(cyx): cache / list the set of triggers
			// somewhere maybe? From there we can use them to
			// determine what the valid triggers are.
			action := &management.Action{
				Name: args[0],
				SupportedTriggers: []management.Trigger{
					{
						ID:      management.TriggerID(flags.trigger),
						Version: "v1",
					},
				},
			}

			return ansi.Spinner("Creating action", func() error {
				return cfg.API.Action.Create(action)
			})

			// TODO: add some more help text here.
		},
	}

	cmd.Flags().StringVarP(&flags.trigger,
		"trigger", "t", "", "Supported trigger for the action.",
	)
	mustRequireFlags(cmd, "trigger")

	return cmd
}

func deployActionCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy <name>",
		Short: "Deploy an action.",
		Long: `Deploy an action. This creates a new version.

The deploy lifecycle is as follows:

1. Build the code artifact. Produces a new version.
2. Route production traffic at it.
3. Bind it to the associated trigger (if not already bound).
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("deploy called")
			return nil
		},
	}

	return cmd
}

func renameActionCmd(cfg *config.Config) *cobra.Command {
	var flags struct {
		newname string
	}

	cmd := &cobra.Command{
		Use:   "rename <name>",
		Args:  validators.ExactArgs("<name>"),
		Short: "Rename an existing action.",
		Long: `Renames an action. If any generated files are found those files are also renamed.:

The following generated files will be moved:

- code.js
- testdata.json
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			action, err := findActionByName(cfg, name)
			if err != nil {
				return err
			}

			return ansi.Spinner("Renaming action", func() error {
				return cfg.API.Action.Update(action.ID, &management.Action{Name: flags.newname})
			})
		},
	}

	cmd.Flags().StringVarP(&flags.newname,
		"newname", "n", "", "New name of the action.",
	)
	mustRequireFlags(cmd, "newname")

	return cmd
}

func deleteActionCmd(cfg *config.Config) *cobra.Command {
	var flags struct {
		confirm string
	}

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Args:  validators.ExactArgs("<name>"),
		Short: "Delete an existing action.",
		Long: `Deletes an existing action. Only actions not bound to triggers can be deleted.

To delete an action already bound, you have to:

1. Remove it from the trigger.
2. Delete the action after.

Note that all code artifacts will also be deleted.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if flags.confirm != args[0] {
				return fmt.Errorf("Confirmation required. Try running `auth0 actions delete %s --confirm %s`", name, name)
			}

			action, err := findActionByName(cfg, name)
			if err != nil {
				return err
			}

			return ansi.Spinner("Deleting action", func() error {
				return cfg.API.Action.Delete(action.ID)
			})
		},
	}

	cmd.Flags().StringVarP(&flags.confirm,
		"confirm", "c", "", "Confirm the action name to be deleted.",
	)
	mustRequireFlags(cmd, "confirm")

	return cmd
}

func findActionByName(cfg *config.Config, name string) (*management.Action, error) {
	// TODO(cyx): add a WithName and a filter by name in
	// the management API. For now we're gonna use
	// post-login since that's all we're creating to test
	// it out.
	list, err := cfg.API.Action.List(management.WithTriggerID(management.TriggerID("post-login")))
	if err != nil {
		return nil, err
	}

	// Temporary shim: when we have a list by name, we'll
	// just straight check the count and ensure it's 1
	// then.
	for _, a := range list.Actions {
		if a.Name == name {
			return a, nil
		}
	}

	return nil, fmt.Errorf("Action with name `%s` not found.", name)
}
