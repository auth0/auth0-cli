package cli

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/validators"
	"github.com/cyx/auth0/management"
	"github.com/spf13/cobra"
)

func actionsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "actions",
		Short: "manage resources for actions.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(initActionsCmd(cli))
	cmd.AddCommand(listActionsCmd(cli))
	cmd.AddCommand(showActionCmd(cli))
	cmd.AddCommand(createActionCmd(cli))
	cmd.AddCommand(renameActionCmd(cli))
	cmd.AddCommand(deleteActionCmd(cli))
	cmd.AddCommand(deployActionCmd(cli))

	return cmd
}

func initActionsCmd(cli *cli) *cobra.Command {
	var flags struct {
		sync      bool
		overwrite bool
	}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize actions project structure.",
		Long:  `Initialize actions project structure. Optionally sync your actions.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return initActions(cli, flags.sync, flags.overwrite)
		},
	}

	cmd.Flags().BoolVarP(&flags.sync,
		"sync", "", false, "Sync actions code from the management API.",
	)

	cmd.Flags().BoolVarP(&flags.overwrite,
		"overwrite", "", false, "Overwrite existing files.",
	)

	return cmd

}

func listActionsCmd(cli *cli) *cobra.Command {
	var flags struct {
		trigger string
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List existing actions",
		Long:  `List actions within a specific trigger.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			list, err := cli.api.Action.List(management.WithTriggerID(management.TriggerID(flags.trigger)))
			if err != nil {
				return err
			}

			cli.renderer.ActionList(list.Actions)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.trigger,
		"trigger", "t", "", "Only list actions within this trigger.",
	)
	mustRequireFlags(cmd, "trigger")

	return cmd
}

func showActionCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <name>",
		Args:  validators.ExactArgs("<name>"),
		Short: "Show action information.",
		Long:  "Show action information. Shows existing versions deployed.",
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			var (
				action *management.Action
				list   *management.ActionVersionList
			)

			err := ansi.Spinner("Fetching action", func() error {
				var err error
				if action, err = findActionByName(cli, name); err != nil {
					return err
				}

				list, err = cli.api.ActionVersion.List(action.ID)
				return err
			})

			if err != nil {
				return err
			}

			cli.renderer.ActionInfo(action, list.Versions)
			return nil
		},
	}

	return cmd
}

func createActionCmd(cli *cli) *cobra.Command {
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

			err := ansi.Spinner("Creating action", func() error {
				return cli.api.Action.Create(action)
			})

			if err != nil {
				return err
			}

			code := codeTemplateFor(action)
			f, relPath := defaultActionCodePath(cli.tenant, action.Name)

			if err := os.MkdirAll(path.Dir(f), 0755); err != nil {
				return err
			}

			if err := ioutil.WriteFile(f, code, 0644); err != nil {
				return err
			}

			cli.renderer.Infof("A template was generated in %s", relPath)
			cli.renderer.Infof("Use `auth0 deploy actions %s` to publish a new version.", action.Name)

			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.trigger,
		"trigger", "t", "", "Supported trigger for the action.",
	)
	mustRequireFlags(cmd, "trigger")

	return cmd
}

func deployActionCmd(cli *cli) *cobra.Command {
	var flags struct {
		file string
	}

	cmd := &cobra.Command{
		Use:   "deploy <name>",
		Args:  validators.ExactArgs("<name>"),
		Short: "Deploy an action.",
		Long: `Deploy an action. This creates a new version.

The deploy lifecycle is as follows:

1. Build the code artifact. Produces a new version.
2. Route production traffic at it.
3. Bind it to the associated trigger (if not already bound).
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if flags.file == "" {
				f, relPath := defaultActionCodePath(cli.tenant, name)
				if !fileExists(f) {
					return fmt.Errorf("`%s` does not exist. Try `auth0 actions init --sync`", relPath)
				}

				flags.file = f
			}

			action, err := findActionByName(cli, name)
			if err != nil {
				return err
			}

			code, err := ioutil.ReadFile(flags.file)
			if err != nil {
				return err
			}

			dependencies := []management.Dependency{
				{Name: "lodash", Version: "v4.17.20"},
			} // TODO
			runtime := "node12" // TODO

			version := &management.ActionVersion{
				Code:         string(code),
				Dependencies: dependencies,
				Runtime:      runtime,
			}

			return ansi.Spinner("Deploying action: "+name, func() error {
				return cli.api.ActionVersion.Deploy(action.ID, version)
			})
		},
	}

	cmd.Flags().StringVarP(&flags.file,
		"file", "f", "", "File which contains code to deploy.",
	)

	return cmd
}

func renameActionCmd(cli *cli) *cobra.Command {
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
			action, err := findActionByName(cli, name)
			if err != nil {
				return err
			}

			return ansi.Spinner("Renaming action", func() error {
				return cli.api.Action.Update(action.ID, &management.Action{Name: flags.newname})
			})
		},
	}

	cmd.Flags().StringVarP(&flags.newname,
		"newname", "n", "", "New name of the action.",
	)
	mustRequireFlags(cmd, "newname")

	return cmd
}

func deleteActionCmd(cli *cli) *cobra.Command {
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

			action, err := findActionByName(cli, name)
			if err != nil {
				return err
			}

			return ansi.Spinner("Deleting action", func() error {
				return cli.api.Action.Delete(action.ID)
			})
		},
	}

	cmd.Flags().StringVarP(&flags.confirm,
		"confirm", "c", "", "Confirm the action name to be deleted.",
	)
	mustRequireFlags(cmd, "confirm")

	return cmd
}

func initActions(cli *cli, sync, overwrite bool) error {
	// TODO(cyx): should allow lising all actions. for now just limiting to
	// post-login
	list, err := cli.api.Action.List(management.WithTriggerID(management.TriggerID("post-login")))
	if err != nil {
		return err
	}

	for _, a := range list.Actions {
		f, relPath := defaultActionCodePath(cli.tenant, a.Name)
		if fileExists(f) && !overwrite {
			cli.renderer.Warnf("skip: %s", relPath)
			continue
		}

		if err := os.MkdirAll(path.Dir(f), 0755); err != nil {
			return err
		}

		if err := ioutil.WriteFile(f, codeTemplateFor(a), 0644); err != nil {
			return err
		}
		cli.renderer.Infof("%s initialized", relPath)

		if sync {
			panic("NOT IMPLEMENTED")
		}
	}

	return nil
}

func codeTemplateFor(action *management.Action) []byte {
	// TODO(cyx): need to find the right template based on supported trigger.
	return []byte(`module.exports = function(user, context, cb) {
    cb(null, user, context)
}
`)
}

func findActionByName(cli *cli, name string) (*management.Action, error) {
	// TODO(cyx): add a WithName and a filter by name in
	// the management API. For now we're gonna use
	// post-login since that's all we're creating to test
	// it out.
	list, err := cli.api.Action.List(management.WithTriggerID(management.TriggerID("post-login")))
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

func defaultActionCodePath(tenant, name string) (fullPath, relativePath string) {
	pwd, err := os.Getwd()
	if err != nil {
		// This is really exceptional behavior if we can't figure out
		// the current working directory.
		panic(err)
	}

	relativePath = path.Join(tenant, "actions", name, "code.js")
	return path.Join(pwd, relativePath), relativePath
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
