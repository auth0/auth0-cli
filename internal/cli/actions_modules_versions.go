package cli

import (
	"context"
	"errors"
	"fmt"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/prompt"
)

var actionModuleVersionID = Argument{
	Name: "Version Id",
	Help: "Id of the action module version.",
}

func actionsModulesVersionsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "versions",
		Short: "Manage action module versions",
		Long: "Every published action module version is an immutable snapshot of the module's code, dependencies, and secrets. " +
			"List and inspect a module's versions here, roll the draft back to a past version, or publish the current draft as a new version.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listActionModuleVersionsCmd(cli))
	cmd.AddCommand(showActionModuleVersionCmd(cli))
	cmd.AddCommand(rollbackActionModuleVersionCmd(cli))
	cmd.AddCommand(publishActionModuleVersionCmd(cli))

	return cmd
}

func listActionModuleVersionsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ModuleID string
		Number   int
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "List the versions of an action module",
		Long:    "List the immutable versions that have been published for an action module.",
		Example: `  auth0 actions modules versions list
  auth0 actions modules versions ls <module-id>
  auth0 actions modules versions list <module-id> --number 100
  auth0 actions modules versions list <module-id> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Number < 1 || inputs.Number > 1000 {
				return fmt.Errorf("number flag invalid, please pass a number between 1 and 1000")
			}

			if len(args) == 0 {
				if err := actionModuleID.Pick(cmd, &inputs.ModuleID, cli.actionModulePickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ModuleID = args[0]
			}

			versions, err := collectV3Pages(cmd.Context(), inputs.Number,
				func(ctx context.Context) (*auth0.ActionModuleVersionPage, error) {
					return cli.apiv3.ActionModuleVersion.List(ctx, inputs.ModuleID, &managementv3.GetActionModuleVersionsRequestParameters{})
				})
			if err != nil {
				return fmt.Errorf("failed to list versions for action module with ID %q: %w", inputs.ModuleID, err)
			}

			cli.renderer.ActionModuleVersionList(versions)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	actionModuleNumber.RegisterInt(cmd, &inputs.Number, defaultPageSize)

	return cmd
}

func showActionModuleVersionCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ModuleID  string
		VersionID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(2),
		Short: "Show an action module version",
		Long:  "Display the code, dependencies, and secrets captured by a specific action module version.",
		Example: `  auth0 actions modules versions show
  auth0 actions modules versions show <module-id> <version-id>
  auth0 actions modules versions show <module-id> <version-id> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := pickActionModuleAndVersion(cli, cmd, args, &inputs.ModuleID, &inputs.VersionID); err != nil {
				return err
			}

			var version *managementv3.GetActionModuleVersionResponseContent
			if err := ansi.Waiting(func() (err error) {
				version, err = cli.apiv3.ActionModuleVersion.Get(cmd.Context(), inputs.ModuleID, inputs.VersionID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to read version %q of action module with ID %q: %w", inputs.VersionID, inputs.ModuleID, err)
			}

			cli.renderer.ActionModuleVersionShow(version)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	return cmd
}

func rollbackActionModuleVersionCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ModuleID  string
		VersionID string
	}

	cmd := &cobra.Command{
		Use:   "rollback",
		Args:  cobra.MaximumNArgs(2),
		Short: "Roll an action module back to a previous version",
		Long: "Copy the code, dependencies, and secrets of a previously published version back into the module's draft.\n\n" +
			"This does not create a new version; it only replaces the draft. Publish afterwards to snapshot the restored draft as a new version.",
		Example: `  auth0 actions modules versions rollback
  auth0 actions modules versions rollback <module-id> <version-id>
  auth0 actions modules versions rollback <module-id> <version-id> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := pickActionModuleAndVersion(cli, cmd, args, &inputs.ModuleID, &inputs.VersionID); err != nil {
				return err
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("This replaces the module's draft with the selected version. Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			if err := ansi.Spinner("Rolling back action module", func() error {
				_, err := cli.apiv3.ActionModule.Rollback(cmd.Context(), inputs.ModuleID, &managementv3.RollbackActionModuleRequestParameters{
					ModuleVersionID: inputs.VersionID,
				})
				return err
			}); err != nil {
				return fmt.Errorf("failed to roll back action module with ID %q to version %q: %w", inputs.ModuleID, inputs.VersionID, err)
			}

			// Re-read the module so the output reflects the restored draft.
			var module *managementv3.GetActionModuleResponseContent
			if err := ansi.Waiting(func() (err error) {
				module, err = cli.apiv3.ActionModule.Get(cmd.Context(), inputs.ModuleID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to read action module with ID %q: %w", inputs.ModuleID, err)
			}

			cli.renderer.ActionModuleShow(module)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	return cmd
}

func publishActionModuleVersionCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ModuleID string
	}

	cmd := &cobra.Command{
		Use:   "publish",
		Args:  cobra.MaximumNArgs(1),
		Short: "Publish an action module draft as a new version",
		Long: "Snapshot an action module's current draft as a new immutable version.\n\n" +
			"This is equivalent to the `--publish` flag on `auth0 actions modules create` and `auth0 actions modules update`, " +
			"but lets you publish a draft on its own without making any other change.",
		Example: `  auth0 actions modules versions publish
  auth0 actions modules versions publish <module-id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := actionModuleID.Pick(cmd, &inputs.ModuleID, cli.actionModulePickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ModuleID = args[0]
			}

			// Read the module first so we can skip a redundant publish when the
			// draft already matches the latest version.
			var current *managementv3.GetActionModuleResponseContent
			if err := ansi.Waiting(func() (err error) {
				current, err = cli.apiv3.ActionModule.Get(cmd.Context(), inputs.ModuleID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to read action module with ID %q: %w", inputs.ModuleID, err)
			}

			if current.GetAllChangesPublished() {
				cli.renderer.Infof("The module is already fully published; there are no draft changes to publish.")
				cli.renderer.ActionModuleShow(current)
				return nil
			}

			if err := ansi.Spinner("Publishing action module", func() error {
				_, err := cli.apiv3.ActionModuleVersion.Create(cmd.Context(), inputs.ModuleID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to publish action module with ID %q: %w", inputs.ModuleID, err)
			}

			// Re-read so the output reflects the newly published version.
			var published *managementv3.GetActionModuleResponseContent
			if err := ansi.Waiting(func() (err error) {
				published, err = cli.apiv3.ActionModule.Get(cmd.Context(), inputs.ModuleID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to read action module with ID %q: %w", inputs.ModuleID, err)
			}

			cli.renderer.ActionModuleShow(published)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	return cmd
}

// pickActionModuleAndVersion resolves the module and version IDs from the
// positional args, falling back to interactive pickers for whichever is
// missing. The version picker is scoped to the resolved module.
func pickActionModuleAndVersion(cli *cli, cmd *cobra.Command, args []string, moduleID, versionID *string) error {
	if len(args) >= 1 {
		*moduleID = args[0]
	} else {
		if err := actionModuleID.Pick(cmd, moduleID, cli.actionModulePickerOptions); err != nil {
			return err
		}
	}

	if len(args) >= 2 {
		*versionID = args[1]
		return nil
	}

	return actionModuleVersionID.Pick(cmd, versionID, cli.actionModuleVersionPickerOptions(*moduleID))
}

// actionModuleVersionPickerOptions returns a picker of the versions for the
// given module, most recent first as returned by the API.
func (c *cli) actionModuleVersionPickerOptions(moduleID string) pickerOptionsFunc {
	return func(ctx context.Context) (pickerOptions, error) {
		versions, err := collectV3Pages(ctx, 0,
			func(ctx context.Context) (*auth0.ActionModuleVersionPage, error) {
				return c.apiv3.ActionModuleVersion.List(ctx, moduleID, &managementv3.GetActionModuleVersionsRequestParameters{})
			})
		if err != nil {
			return nil, err
		}

		var opts pickerOptions
		for _, v := range versions {
			label := fmt.Sprintf("v%d %s", v.GetVersionNumber(), ansi.Faint("("+v.GetID()+")"))
			opts = append(opts, pickerOption{value: v.GetID(), label: label})
		}

		if len(opts) == 0 {
			return nil, errors.New("this action module has no published versions to choose from. Publish one by running: `auth0 actions modules versions publish`")
		}

		return opts, nil
	}
}
