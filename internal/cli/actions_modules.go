package cli

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"sort"
	"strings"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/prompt"
)

// actionModuleNamePattern is the name constraint enforced by the Management
// API: the name must start with a lowercase letter or digit and contain only
// lowercase letters, digits, underscores, and hyphens.
var actionModuleNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

var (
	actionModuleID = Argument{
		Name: "Id",
		Help: "Id of the action module.",
	}
	actionModuleName = Flag{
		Name:       "Name",
		LongForm:   "name",
		ShortForm:  "n",
		Help:       "Name of the action module. Must start with a lowercase letter or digit and contain only lowercase letters, digits, underscores, and hyphens.",
		IsRequired: true,
	}
	actionModuleCode = Flag{
		Name:       "Code",
		LongForm:   "code",
		ShortForm:  "c",
		Help:       "Code content of the action module.",
		IsRequired: true,
	}
	actionModuleDependency = Flag{
		Name:      "Dependency",
		LongForm:  "dependency",
		ShortForm: "d",
		Help:      "Third party npm module, and its version, that the action module depends on.",
	}
	actionModuleSecret = Flag{
		Name:      "Secret",
		LongForm:  "secret",
		ShortForm: "s",
		Help:      "Secrets to be used in the action module.",
	}
	actionModulePublish = Flag{
		Name:     "Publish",
		LongForm: "publish",
		Help:     "Publish the module's draft as a new immutable version once the create or update succeeds.",
	}
	actionModuleAPIVersion = Flag{
		Name:     "API Version",
		LongForm: "api-version",
		Help:     "API version of the action module.",
	}
	actionModuleNumber = Flag{
		Name:      "Number",
		LongForm:  "number",
		ShortForm: "n",
		Help:      "Number of action modules to retrieve. Minimum 1, maximum 1000.",
	}
)

func actionsModulesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "modules",
		Short: "Manage action modules",
		Long: "Action modules are reusable code libraries that actions can import. " +
			"Manage them here, then associate a module with an action using the `--module` flag on `auth0 actions create` or `auth0 actions update`.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listActionModulesCmd(cli))
	cmd.AddCommand(showActionModuleCmd(cli))
	cmd.AddCommand(createActionModuleCmd(cli))
	cmd.AddCommand(updateActionModuleCmd(cli))
	cmd.AddCommand(deleteActionModuleCmd(cli))
	cmd.AddCommand(actionsModulesActionsCmd(cli))
	cmd.AddCommand(actionsModulesVersionsCmd(cli))

	return cmd
}

func listActionModulesCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Number int
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your action modules",
		Long:    "List the action modules in your tenant.",
		Example: `  auth0 actions modules list
  auth0 actions modules ls
  auth0 actions modules list --number 100
  auth0 actions modules list -n 100 --json
  auth0 actions modules list --csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Number < 1 || inputs.Number > 1000 {
				return fmt.Errorf("number flag invalid, please pass a number between 1 and 1000")
			}

			modules, err := collectV3Pages(cmd.Context(), inputs.Number,
				func(ctx context.Context) (*auth0.ActionModulePage, error) {
					return cli.apiv3.ActionModule.List(ctx, &managementv3.GetActionModulesRequestParameters{})
				})
			if err != nil {
				return fmt.Errorf("failed to list action modules: %w", err)
			}

			cli.renderer.ActionModuleList(modules)

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

func showActionModuleCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show an action module",
		Long:  "Display the code, dependencies, secrets, and version information about an action module.",
		Example: `  auth0 actions modules show
  auth0 actions modules show <module-id>
  auth0 actions modules show <module-id> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := actionModuleID.Pick(cmd, &inputs.ID, cli.actionModulePickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var module *managementv3.GetActionModuleResponseContent
			if err := ansi.Waiting(func() (err error) {
				module, err = cli.apiv3.ActionModule.Get(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to read action module with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.ActionModuleShow(module)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	return cmd
}

func createActionModuleCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Name         string
		Code         string
		Dependencies map[string]string
		Secrets      map[string]string
		Publish      bool
		APIVersion   string
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new action module",
		Long: "Create a new action module.\n\n" +
			"To create interactively, use `auth0 actions modules create` with no flags.\n\n" +
			"To create non-interactively, supply the name and code (and optionally dependencies, secrets, and publish) through flags.",
		Example: `  auth0 actions modules create
  auth0 actions modules create --name mymodule --code "$(cat path/to/module.js)"
  auth0 actions modules create --name mymodule --code "$(cat path/to/module.js)" --publish
  auth0 actions modules create --name mymodule --code "$(cat path/to/module.js)" --dependency "lodash=4.0.0" --secret "API_KEY=value"
  auth0 actions modules create --name mymodule --code "$(cat path/to/module.js)" --api-version v1
  auth0 actions modules create -n mymodule -c "$(cat path/to/module.js)" -d "lodash=4.0.0" -s "API_KEY=value" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := actionModuleName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			if !actionModuleNamePattern.MatchString(inputs.Name) {
				return fmt.Errorf("invalid name %q: must start with a lowercase letter or digit and contain only lowercase letters, digits, underscores, and hyphens", inputs.Name)
			}

			if err := actionModuleCode.OpenEditor(
				cmd,
				&inputs.Code,
				"",
				inputs.Name+".*.js",
				cli.actionEditorHint,
			); err != nil {
				return err
			}

			// When run interactively without the corresponding flags, collect
			// the optional fields through guided prompts.
			if canPrompt(cmd) {
				if !actionModuleDependency.IsSet(cmd) {
					deps, err := askActionModuleDependencies(nil)
					if err != nil {
						return err
					}
					inputs.Dependencies = deps
				}

				if !actionModuleSecret.IsSet(cmd) {
					secrets, err := askActionModuleSecrets()
					if err != nil {
						return err
					}
					inputs.Secrets = secrets
				}

				if !actionModulePublish.IsSet(cmd) {
					if err := prompt.AskBool("Publish this module now?", &inputs.Publish, false); err != nil {
						return err
					}
				}

				if !actionModuleAPIVersion.IsSet(cmd) {
					if err := prompt.AskOne(prompt.TextInput("", "API version (optional):", "Leave blank to use the default.", "", false), &inputs.APIVersion); err != nil {
						return err
					}
				}
			}

			module := &managementv3.CreateActionModuleRequestContent{
				Name:         inputs.Name,
				Code:         inputs.Code,
				Dependencies: inputDependenciesToActionModuleDependencies(inputs.Dependencies),
				Secrets:      inputSecretsToActionModuleSecrets(inputs.Secrets),
			}
			if inputs.Publish {
				module.Publish = auth0.Bool(true)
			}
			if inputs.APIVersion != "" {
				module.APIVersion = &inputs.APIVersion
			}

			var created *managementv3.CreateActionModuleResponseContent
			if err := ansi.Waiting(func() (err error) {
				created, err = cli.apiv3.ActionModule.Create(cmd.Context(), module)
				return err
			}); err != nil {
				return fmt.Errorf("failed to create action module: %w", err)
			}

			cli.renderer.ActionModuleCreate(created)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	actionModuleName.RegisterString(cmd, &inputs.Name, "")
	actionModuleCode.RegisterString(cmd, &inputs.Code, "")
	actionModuleDependency.RegisterStringMap(cmd, &inputs.Dependencies, nil)
	actionModuleSecret.RegisterStringMap(cmd, &inputs.Secrets, nil)
	actionModulePublish.RegisterBool(cmd, &inputs.Publish, false)
	actionModuleAPIVersion.RegisterString(cmd, &inputs.APIVersion, "")

	return cmd
}

func updateActionModuleCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID           string
		Code         string
		Dependencies map[string]string
		Secrets      map[string]string
		Publish      bool
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update an action module",
		Long: "Update an action module.\n\n" +
			"The module name is immutable and cannot be changed. Only the fields you pass are updated; " +
			"omitted fields keep their current values.\n\n" +
			"Updates edit the module's draft. Pass `--publish` to also snapshot the draft as a new immutable version once the update succeeds.",
		Example: `  auth0 actions modules update <module-id> --code "$(cat path/to/module.js)"
  auth0 actions modules update <module-id> --dependency "lodash=4.0.0" --secret "API_KEY=value"
  auth0 actions modules update <module-id> --code "$(cat path/to/module.js)" --publish
  auth0 actions modules update <module-id> -c "$(cat path/to/module.js)" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := actionModuleID.Pick(cmd, &inputs.ID, cli.actionModulePickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			module := &managementv3.UpdateActionModuleRequestContent{}

			if actionModuleCode.IsSet(cmd) {
				module.Code = &inputs.Code
			}
			if actionModuleDependency.IsSet(cmd) {
				module.Dependencies = inputDependenciesToActionModuleDependencies(inputs.Dependencies)
			}
			if actionModuleSecret.IsSet(cmd) {
				module.Secrets = inputSecretsToActionModuleSecrets(inputs.Secrets)
			}

			// Current is populated when the interactive path fetches the module,
			// and reused for the no-op case below to avoid a second read.
			var current *managementv3.GetActionModuleResponseContent

			// When run interactively without the update flags, guide the user
			// through editing the code, dependencies, and secrets of the module.
			if canPrompt(cmd) && !actionModuleCode.IsSet(cmd) && !actionModuleDependency.IsSet(cmd) && !actionModuleSecret.IsSet(cmd) {
				var err error
				current, err = cli.apiv3.ActionModule.Get(cmd.Context(), inputs.ID)
				if err != nil {
					return fmt.Errorf("failed to read action module with ID %q: %w", inputs.ID, err)
				}

				var editCode bool
				if err := prompt.AskBool("Edit the code?", &editCode, false); err != nil {
					return err
				}
				if editCode {
					code := current.GetCode()
					if err := actionModuleCode.OpenEditorU(cmd, &code, code, current.GetName()+".*.js"); err != nil {
						return err
					}
					module.Code = &code
				}

				var editDeps bool
				if err := prompt.AskBool("Edit the dependencies?", &editDeps, false); err != nil {
					return err
				}
				if editDeps {
					deps, err := askActionModuleDependencies(currentActionModuleDependencies(current))
					if err != nil {
						return err
					}
					module.Dependencies = inputDependenciesToActionModuleDependencies(deps)
				}

				var replaceSecrets bool
				if err := prompt.AskBool("Replace all secrets?", &replaceSecrets, false); err != nil {
					return err
				}
				if replaceSecrets {
					cli.renderer.Warnf("Existing secret values cannot be read back, so this replaces every secret on the module. Any secret you do not re-enter below will be removed.")
					secrets, err := askActionModuleSecrets()
					if err != nil {
						return err
					}
					module.Secrets = inputSecretsToActionModuleSecrets(secrets)
				}

				// Only offer to publish when there is actually something to
				// publish: either the user edited a field above, or the module's
				// draft already has changes that were never published.
				edited := module.Code != nil || module.Dependencies != nil || module.Secrets != nil
				if !actionModulePublish.IsSet(cmd) && (edited || !current.GetAllChangesPublished()) {
					if err := prompt.AskBool("Publish the module as a new version?", &inputs.Publish, false); err != nil {
						return err
					}
				}
			}

			hasChanges := module.Code != nil || module.Dependencies != nil || module.Secrets != nil

			// With neither field changes nor a publish request there is nothing to
			// do, so show the current module instead of issuing an empty, no-op
			// write request.
			if !hasChanges && !inputs.Publish {
				if current == nil {
					var err error
					if err = ansi.Waiting(func() (err error) {
						current, err = cli.apiv3.ActionModule.Get(cmd.Context(), inputs.ID)
						return err
					}); err != nil {
						return fmt.Errorf("failed to read action module with ID %q: %w", inputs.ID, err)
					}
				}

				cli.renderer.Infof("No changes to apply.")
				cli.renderer.ActionModuleShow(current)

				return nil
			}

			// AllChangesPublished tells us whether the draft already matches the
			// latest version, which lets us skip a redundant publish.
			var allChangesPublished bool
			if hasChanges {
				var updated *managementv3.UpdateActionModuleResponseContent
				if err := ansi.Waiting(func() (err error) {
					updated, err = cli.apiv3.ActionModule.Update(cmd.Context(), inputs.ID, module)
					return err
				}); err != nil {
					return fmt.Errorf("failed to update action module with ID %q: %w", inputs.ID, err)
				}
				allChangesPublished = updated.GetAllChangesPublished()

				if !inputs.Publish {
					cli.renderer.ActionModuleUpdate(updated)
					return nil
				}
			} else {
				// Publish requested without field changes: read the module so we
				// know whether the draft has anything left to publish.
				if current == nil {
					var err error
					if err = ansi.Waiting(func() (err error) {
						current, err = cli.apiv3.ActionModule.Get(cmd.Context(), inputs.ID)
						return err
					}); err != nil {
						return fmt.Errorf("failed to read action module with ID %q: %w", inputs.ID, err)
					}
				}
				allChangesPublished = current.GetAllChangesPublished()
			}

			if allChangesPublished {
				cli.renderer.Infof("The module is already fully published; there are no draft changes to publish.")
			} else {
				if err := ansi.Spinner("Publishing action module", func() error {
					_, err := cli.apiv3.ActionModuleVersion.Create(cmd.Context(), inputs.ID)
					return err
				}); err != nil {
					return fmt.Errorf("failed to publish action module with ID %q: %w", inputs.ID, err)
				}
			}

			// Re-read the module so the output reflects the published version and
			// the updated publish state.
			var published *managementv3.GetActionModuleResponseContent
			if err := ansi.Waiting(func() (err error) {
				published, err = cli.apiv3.ActionModule.Get(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to read action module with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.ActionModuleShow(published)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	actionModuleCode.RegisterStringU(cmd, &inputs.Code, "")
	actionModuleDependency.RegisterStringMapU(cmd, &inputs.Dependencies, nil)
	actionModuleSecret.RegisterStringMapU(cmd, &inputs.Secrets, nil)
	actionModulePublish.RegisterBool(cmd, &inputs.Publish, false)

	return cmd
}

func deleteActionModuleCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Short:   "Delete an action module",
		Long: "Delete an action module.\n\n" +
			"To delete interactively, use `auth0 actions modules delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the module id and the `--force` flag to skip confirmation.\n\n" +
			"A module that is in use by a deployed action version cannot be deleted; such modules are hidden from the interactive picker.",
		Example: `  auth0 actions modules delete
  auth0 actions modules rm
  auth0 actions modules delete <module-id>
  auth0 actions modules delete <module-id> --force
  auth0 actions modules delete <module-id> <module-id2> <module-idn>
  auth0 actions modules delete <module-id> <module-id2> <module-idn> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var ids []string
			if len(args) == 0 {
				if err := actionModuleID.PickMany(cmd, &ids, cli.deletableActionModulePickerOptions); err != nil {
					return err
				}
			} else {
				ids = args
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			return ansi.ProgressBar("Deleting action module(s)", ids, func(_ int, id string) error {
				if id != "" {
					if err := cli.apiv3.ActionModule.Delete(cmd.Context(), id); err != nil {
						return fmt.Errorf("failed to delete action module with ID %q: %w", id, err)
					}
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

func actionsModulesActionsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "actions",
		Short: "Manage the actions using an action module",
		Long:  "Inspect which actions import an action module.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listActionsUsingModuleCmd(cli))

	return cmd
}

func listActionsUsingModuleCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ModuleID string
		Number   int
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "List the actions using an action module",
		Long:    "List the actions that import an action module, along with the module version each action is using.",
		Example: `  auth0 actions modules actions list
  auth0 actions modules actions ls <module-id>
  auth0 actions modules actions list <module-id> --number 100
  auth0 actions modules actions list <module-id> --json`,
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

			actions, err := collectV3Pages(cmd.Context(), inputs.Number,
				func(ctx context.Context) (*auth0.ActionModuleActionPage, error) {
					return cli.apiv3.ActionModule.ListActions(ctx, inputs.ModuleID, &managementv3.GetActionModuleActionsRequestParameters{})
				})
			if err != nil {
				return fmt.Errorf("failed to list actions using action module with ID %q: %w", inputs.ModuleID, err)
			}

			cli.renderer.ActionModuleActionsList(actions)

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

func (c *cli) actionModulePickerOptions(ctx context.Context) (pickerOptions, error) {
	return c.actionModulePickerOptionsFiltered(ctx, false)
}

// deletableActionModulePickerOptions lists only modules that can actually be
// deleted. A module in use by a deployed action version is rejected by the API
// with a 412, so those are hidden from the delete picker to keep it to choices
// that will succeed.
func (c *cli) deletableActionModulePickerOptions(ctx context.Context) (pickerOptions, error) {
	return c.actionModulePickerOptionsFiltered(ctx, true)
}

// actionModulePickerOptionsFiltered builds the module picker. When
// deletableOnly is set it omits modules still referenced by deployed action
// versions, which the API refuses to delete.
func (c *cli) actionModulePickerOptionsFiltered(ctx context.Context, deletableOnly bool) (pickerOptions, error) {
	modules, err := collectV3Pages(ctx, 0,
		func(ctx context.Context) (*auth0.ActionModulePage, error) {
			return c.apiv3.ActionModule.List(ctx, &managementv3.GetActionModulesRequestParameters{})
		})
	if err != nil {
		return nil, err
	}

	var opts pickerOptions
	for _, m := range modules {
		if deletableOnly && m.GetActionsUsingModuleTotal() > 0 {
			continue
		}
		label := fmt.Sprintf("%s %s", m.GetName(), ansi.Faint("("+m.GetID()+")"))
		opts = append(opts, pickerOption{value: m.GetID(), label: label})
	}

	if len(opts) == 0 {
		if deletableOnly {
			return nil, errors.New("there are no action modules available to delete; modules in use by deployed action versions cannot be deleted")
		}
		return nil, errors.New("there are currently no action modules to choose from. Create one by running: `auth0 actions modules create`")
	}

	return opts, nil
}

func inputDependenciesToActionModuleDependencies(dependencies map[string]string) []*managementv3.ActionModuleDependencyRequest {
	if len(dependencies) == 0 {
		return nil
	}

	result := make([]*managementv3.ActionModuleDependencyRequest, 0, len(dependencies))
	for name, version := range dependencies {
		result = append(result, &managementv3.ActionModuleDependencyRequest{
			Name:    name,
			Version: version,
		})
	}

	return result
}

func inputSecretsToActionModuleSecrets(secrets map[string]string) []*managementv3.ActionModuleSecretRequest {
	if len(secrets) == 0 {
		return nil
	}

	result := make([]*managementv3.ActionModuleSecretRequest, 0, len(secrets))
	for name, value := range secrets {
		result = append(result, &managementv3.ActionModuleSecretRequest{
			Name:  name,
			Value: value,
		})
	}

	return result
}

// currentActionModuleDependencies maps a module's existing dependencies into
// the name/version form used by the interactive prompts.
func currentActionModuleDependencies(module *managementv3.GetActionModuleResponseContent) map[string]string {
	dependencies := map[string]string{}
	for _, d := range module.GetDependencies() {
		dependencies[d.GetName()] = d.GetVersion()
	}
	return dependencies
}

// askActionModuleDependencies interactively collects npm dependencies as
// name/version pairs. It first asks a single gate question, then loops through
// one entry at a time and prints a recap of everything collected. Any entries
// in existing are shown first and carried forward, which lets the update flow
// safely append to the current set instead of replacing it.
func askActionModuleDependencies(existing map[string]string) (map[string]string, error) {
	dependencies := map[string]string{}
	maps.Copy(dependencies, existing)

	for name, version := range dependencies {
		fmt.Printf("  %s keeping %s\n", ansi.Faint("•"), ansi.Faint(name+"@"+version))
	}

	var add bool
	if err := prompt.AskBool("Do you want to add dependencies?", &add, false); err != nil {
		return nil, err
	}

	for add {
		var name string
		if err := prompt.AskOne(prompt.TextInput("", "Dependency name:", "The npm package name, e.g. lodash.", "", true), &name); err != nil {
			return nil, err
		}

		var version string
		if err := prompt.AskOne(prompt.TextInput("", "Dependency version:", "The npm package version, e.g. 4.17.21.", "", true), &version); err != nil {
			return nil, err
		}

		dependencies[name] = version
		fmt.Printf("  %s added %s\n", ansi.Green("✓"), ansi.Bold(name+"@"+version))

		if err := prompt.AskBool("Add another dependency?", &add, false); err != nil {
			return nil, err
		}
	}

	if len(dependencies) > 0 {
		fmt.Printf("%s %s\n", ansi.Green("✓"), summarizeActionModuleDependencies(dependencies))
	}

	return dependencies, nil
}

// summarizeActionModuleDependencies renders a one-line recap such as
// "2 dependencies: lodash@4.17.21, auth0@1.4.0".
func summarizeActionModuleDependencies(dependencies map[string]string) string {
	labels := make([]string, 0, len(dependencies))
	for name, version := range dependencies {
		labels = append(labels, name+"@"+version)
	}
	sort.Strings(labels)

	return fmt.Sprintf("%s: %s", pluralize(len(labels), "dependency", "dependencies"), strings.Join(labels, ", "))
}

// askActionModuleSecrets interactively collects secrets as name/value pairs. It
// asks a single gate question, then loops through one entry at a time and
// prints a recap listing only the names collected. Values are entered through a
// masked prompt so they never appear on screen or in shell history, and the
// recap never echoes them back.
func askActionModuleSecrets() (map[string]string, error) {
	secrets := map[string]string{}

	var add bool
	if err := prompt.AskBool("Do you want to add secrets?", &add, false); err != nil {
		return nil, err
	}

	for add {
		var name string
		if err := prompt.AskOne(prompt.TextInput("", "Secret name:", "The secret name, e.g. API_KEY.", "", true), &name); err != nil {
			return nil, err
		}

		var value string
		if err := prompt.AskOne(prompt.PasswordInput("", "Secret value:", true), &value); err != nil {
			return nil, err
		}

		secrets[name] = value
		fmt.Printf("  %s added %s\n", ansi.Green("✓"), ansi.Bold(name))

		if err := prompt.AskBool("Add another secret?", &add, false); err != nil {
			return nil, err
		}
	}

	if len(secrets) > 0 {
		names := make([]string, 0, len(secrets))
		for name := range secrets {
			names = append(names, name)
		}
		sort.Strings(names)
		fmt.Printf("%s %s: %s\n", ansi.Green("✓"), pluralize(len(names), "secret", "secrets"), strings.Join(names, ", "))
	}

	return secrets, nil
}

// pluralize returns "1 singular" or "n plural" with the count prefixed.
func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}
