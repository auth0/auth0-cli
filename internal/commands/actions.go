package commands

import (
	"fmt"

	"github.com/auth0/auth0-cli/internal/config"
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
		Long:  `List all actions within a tenant.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("list called")
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.trigger,
		"trigger", "t", "", "Only list actions within this trigger.",
	)

	return cmd
}

func createActionCmd(cfg *config.Config) *cobra.Command {
	var flags struct {
		trigger string
	}

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create an action.",
		Long: `Creates an action, and generates a few files for working with actions:

- code.js       - function signature.
- testdata.json - sample payload for testing the action.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("create called")
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.trigger,
		"trigger", "t", "", "Supported trigger for the action.",
	)

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
	cmd := &cobra.Command{
		Use:   "rename <name>",
		Short: "Rename an existing action.",
		Long: `Renames an action. If any generated files are found those files are also renamed.:

The following generated files will be moved:

- code.js
- testdata.json
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("rename called")
			return nil
		},
	}

	return cmd
}

func deleteActionCmd(cfg *config.Config) *cobra.Command {
	var flags struct {
		confirm string
	}

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete an existing action.",
		Long: `Deletes an existing action. Only actions not bound to triggers can be deleted.

To delete an action already bound, you have to:

1. Remove it from the trigger.
2. Delete the action after.

Note that all code artifacts will also be deleted.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("delete called")
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.confirm,
		"confirm", "c", "", "Confirm the action name to be deleted.",
	)

	return cmd
}
