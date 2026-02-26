package cli

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/AlecAivazis/survey/v2"
	"github.com/auth0/go-auth0"
	"github.com/auth0/go-auth0/management"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
)

var (
	actionID = Argument{
		Name: "Id",
		Help: "Id of the action.",
	}

	actionName = Flag{
		Name:       "Name",
		LongForm:   "name",
		ShortForm:  "n",
		Help:       "Name of the action.",
		IsRequired: true,
	}

	actionTrigger = Flag{
		Name:       "Trigger",
		LongForm:   "trigger",
		ShortForm:  "t",
		Help:       "Trigger of the action. At this time, an action can only target a single trigger at a time.",
		IsRequired: true,
	}

	actionCode = Flag{
		Name:       "Code",
		LongForm:   "code",
		ShortForm:  "c",
		Help:       "Code content for the action.",
		IsRequired: true,
	}

	actionDependency = Flag{
		Name:      "Dependency",
		LongForm:  "dependency",
		ShortForm: "d",
		Help:      "Third party npm module, and its version, that the action depends on.",
	}

	actionSecret = Flag{
		Name:      "Secret",
		LongForm:  "secret",
		ShortForm: "s",
		Help:      "Secrets to be used in the action.",
	}

	actionRuntime = Flag{
		Name:      "Runtime",
		LongForm:  "runtime",
		ShortForm: "r",
		Help:      "Runtime to be used in the action.  Possible values are: node22(recommended), node18, node16, node12",
	}

	actionTemplates = map[string]string{
		"post-login":             actionTemplatePostLogin,
		"credentials-exchange":   actionTemplateCredentialsExchange,
		"pre-user-registration":  actionTemplatePreUserRegistration,
		"post-user-registration": actionTemplatePostUserRegistration,
		"post-change-password":   actionTemplatePostChangePassword,
		"send-phone-message":     actionTemplateSendPhoneMessage,
		"custom-email-provider":  actionTemplateCustomEmailProvider,
		"custom-phone-provider":  actionTemplateCustomPhoneProvider,
	}
)

func actionsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "actions",
		Short: "Manage resources for actions",
		Long: "Actions are secure, tenant-specific, versioned functions written in Node.js that execute " +
			"at certain points within the Auth0 platform. Actions are used to customize and extend Auth0's " +
			"capabilities with custom logic.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listActionsCmd(cli))
	cmd.AddCommand(createActionCmd(cli))
	cmd.AddCommand(showActionCmd(cli))
	cmd.AddCommand(updateActionCmd(cli))
	cmd.AddCommand(deleteActionCmd(cli))
	cmd.AddCommand(deployActionCmd(cli))
	cmd.AddCommand(openActionCmd(cli))
	cmd.AddCommand(diffActionCmd(cli))

	return cmd
}

func listActionsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your actions",
		Long:    "List your existing actions. To create one, run: `auth0 actions create`.",
		Example: `  auth0 actions list
  auth0 actions ls
  auth0 actions ls --json
  auth0 actions ls --json-compact
  auth0 actions ls --csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var list *management.ActionList

			if err := ansi.Waiting(func() (err error) {
				list, err = cli.api.Action.List(cmd.Context(), management.PerPage(defaultPageSize))
				return err
			}); err != nil {
				return fmt.Errorf("failed to list actions: %w", err)
			}

			cli.renderer.ActionList(list.Actions)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	return cmd
}

func showActionCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show an action",
		Long:  "Display the name, type, status, code and other information about an action.",
		Example: `  auth0 actions show
  auth0 actions show <action-id>
  auth0 actions show <action-id> --json
  auth0 actions show <action-id> --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := actionID.Pick(cmd, &inputs.ID, cli.actionPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var action *management.Action

			if err := ansi.Waiting(func() (err error) {
				action, err = cli.api.Action.Read(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to read action with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.ActionShow(action)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func createActionCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Name         string
		Trigger      string
		Code         string
		Dependencies map[string]string
		Secrets      map[string]string
		Runtime      string
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new action",
		Long: "Create a new action.\n\n" +
			"To create interactively, use `auth0 actions create` with no flags.\n\n" +
			"To create non-interactively, supply the action name, trigger, code, secrets and dependencies through the flags.",
		Example: `  auth0 actions create
  auth0 actions create --name myaction
  auth0 actions create --name myaction --trigger post-login
  auth0 actions create --name myaction --trigger post-login --code "$(cat path/to/code.js) --runtime node18
  auth0 actions create --name myaction --trigger post-login --code "$(cat path/to/code.js)" --dependency "lodash=4.0.0"
  auth0 actions create --name myaction --trigger post-login --code "$(cat path/to/code.js)" --dependency "lodash=4.0.0" --secret "SECRET=value"
  auth0 actions create --name myaction --trigger post-login --code "$(cat path/to/code.js)" --dependency "lodash=4.0.0" --dependency "uuid=9.0.0" --secret "API_KEY=value" --secret "SECRET=value"
  auth0 actions create -n myaction -t post-login -c "$(cat path/to/code.js)" -r node18 -d "lodash=4.0.0" -d "uuid=9.0.0" -s "API_KEY=value" -s "SECRET=value" --json
  auth0 actions create -n myaction -t post-login -c "$(cat path/to/code.js)" -r node18 -d "lodash=4.0.0" -d "uuid=9.0.0" -s "API_KEY=value" -s "SECRET=value" --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := actionName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			triggers, err := getCurrentTriggers(cmd.Context(), cli)
			if err != nil {
				return fmt.Errorf("failed to retrieve available triggers: %w", err)
			}

			triggerIDs := make([]string, 0)
			for _, t := range triggers {
				triggerIDs = append(triggerIDs, t.GetID())
			}

			if err := actionTrigger.Select(cmd, &inputs.Trigger, triggerIDs, nil); err != nil {
				return err
			}

			if err := actionCode.OpenEditor(
				cmd,
				&inputs.Code,
				actionTemplate(inputs.Trigger),
				inputs.Name+".*.js",
				cli.actionEditorHint,
			); err != nil {
				return err
			}

			var version string
			for _, t := range triggers {
				if t.GetID() == inputs.Trigger {
					version = t.GetVersion()
					break
				}
			}

			action := &management.Action{
				Name: &inputs.Name,
				SupportedTriggers: []management.ActionTrigger{
					{
						ID:      &inputs.Trigger,
						Version: &version,
					},
				},
				Runtime:      &inputs.Runtime,
				Code:         &inputs.Code,
				Dependencies: inputDependenciesToActionDependencies(inputs.Dependencies),
				Secrets:      inputSecretsToActionSecrets(inputs.Secrets),
			}

			if err := ansi.Waiting(func() error {
				return cli.api.Action.Create(cmd.Context(), action)
			}); err != nil {
				return fmt.Errorf("failed to create action: %w", err)
			}

			cli.renderer.ActionCreate(action)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	actionName.RegisterString(cmd, &inputs.Name, "")
	actionTrigger.RegisterString(cmd, &inputs.Trigger, "")
	actionCode.RegisterString(cmd, &inputs.Code, "")
	actionDependency.RegisterStringMap(cmd, &inputs.Dependencies, nil)
	actionSecret.RegisterStringMap(cmd, &inputs.Secrets, nil)
	actionRuntime.RegisterString(cmd, &inputs.Runtime, "")

	return cmd
}

func updateActionCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID           string
		Name         string
		Code         string
		Dependencies map[string]string
		Secrets      map[string]string
		Runtime      string
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update an action",
		Long: "Update an action.\n\n" +
			"To update interactively, use `auth0 actions update` with no arguments.\n\n" +
			"To update non-interactively, supply the action id, name, code, secrets and " +
			"dependencies through the flags.",
		Example: `  auth0 actions update <action-id>
  auth0 actions update <action-id> --runtime node18
  auth0 actions update <action-id> --name myaction --runtime node18
  auth0 actions update <action-id> --name myaction --code "$(cat path/to/code.js) --r node18"
  auth0 actions update <action-id> --name myaction --code "$(cat path/to/code.js)" --dependency "lodash=4.0.0"
  auth0 actions update <action-id> --name myaction --code "$(cat path/to/code.js)" --dependency "lodash=4.0.0" --secret "SECRET=value"
  auth0 actions update <action-id> --name myaction --code "$(cat path/to/code.js)" --dependency "lodash=4.0.0" --dependency "uuid=9.0.0" --secret "API_KEY=value" --secret "SECRET=value"
  auth0 actions update <action-id> -n myaction -c "$(cat path/to/code.js)" -r node18 -d "lodash=4.0.0" -d "uuid=9.0.0" -s "API_KEY=value" -s "SECRET=value" --json
  auth0 actions update <action-id> -n myaction -c "$(cat path/to/code.js)" -r node18 -d "lodash=4.0.0" -d "uuid=9.0.0" -s "API_KEY=value" -s "SECRET=value" --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				if err := actionID.Pick(cmd, &inputs.ID, cli.actionPickerOptions); err != nil {
					return err
				}
			}

			var oldAction *management.Action
			err := ansi.Waiting(func() (err error) {
				oldAction, err = cli.api.Action.Read(cmd.Context(), inputs.ID)
				return err
			})
			if err != nil {
				return fmt.Errorf("failed to read action with ID %q: %w", inputs.ID, err)
			}

			if err := actionName.AskU(cmd, &inputs.Name, oldAction.Name); err != nil {
				return err
			}

			if err := actionCode.OpenEditorU(
				cmd,
				&inputs.Code,
				oldAction.GetCode(),
				inputs.Name+".*.js",
			); err != nil {
				return fmt.Errorf("failed to capture input from the editor: %w", err)
			}

			if !cli.force && canPrompt(cmd) {
				var confirmed bool
				if err := prompt.AskBool("Do you want to save the action code?", &confirmed, true); err != nil {
					return fmt.Errorf("failed to capture prompt input: %w", err)
				}
				if !confirmed {
					return nil
				}
			}

			updatedAction := &management.Action{
				SupportedTriggers: oldAction.SupportedTriggers,
			}
			if inputs.Name != "" {
				updatedAction.Name = &inputs.Name
			}
			if inputs.Code != "" {
				updatedAction.Code = &inputs.Code
			}
			if len(inputs.Dependencies) != 0 {
				updatedAction.Dependencies = inputDependenciesToActionDependencies(inputs.Dependencies)
			}
			if len(inputs.Secrets) != 0 {
				updatedAction.Secrets = inputSecretsToActionSecrets(inputs.Secrets)
			}

			if inputs.Runtime != "" {
				updatedAction.Runtime = &inputs.Runtime
			}

			if err = ansi.Waiting(func() error {
				return cli.api.Action.Update(cmd.Context(), oldAction.GetID(), updatedAction)
			}); err != nil {
				return fmt.Errorf("failed to update action with ID %q: %w", oldAction.GetID(), err)
			}

			cli.renderer.ActionUpdate(updatedAction)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")
	actionName.RegisterStringU(cmd, &inputs.Name, "")
	actionCode.RegisterStringU(cmd, &inputs.Code, "")
	actionDependency.RegisterStringMapU(cmd, &inputs.Dependencies, nil)
	actionSecret.RegisterStringMapU(cmd, &inputs.Secrets, nil)
	actionRuntime.RegisterStringU(cmd, &inputs.Runtime, "")

	return cmd
}

func deleteActionCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Short:   "Delete an action",
		Long: "Delete an action.\n\n" +
			"To delete interactively, use `auth0 actions delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the action id and the `--force` flag to skip confirmation.",
		Example: `  auth0 actions delete
  auth0 actions rm
  auth0 actions delete <action-id>
  auth0 actions delete <action-id> --force
  auth0 actions delete <action-id> <action-id2> <action-idn>
  auth0 actions delete <action-id> <action-id2> <action-idn> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var ids []string
			if len(args) == 0 {
				if err := actionID.PickMany(cmd, &ids, cli.actionPickerOptions); err != nil {
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

			return ansi.ProgressBar("Deleting action(s)", ids, func(i int, id string) error {
				if id != "" {
					if err := cli.api.Action.Delete(cmd.Context(), id); err != nil {
						return fmt.Errorf("failed to delete Action with ID %q: %w", id, err)
					}
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

func deployActionCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "deploy",
		Args:  cobra.MaximumNArgs(1),
		Short: "Deploy an action",
		Long: "Before an action can be bound to a flow, the action must be deployed.\n\n" +
			"The selected action will be deployed and added to the collection of available actions for flows. " +
			"Additionally, a new draft version of the deployed action will be created for future editing. " +
			"Because secrets and dependencies are tied to versions, any saved secrets or dependencies will " +
			"be available to the new draft.",
		Example: `  auth0 actions deploy
  auth0 actions deploy <action-id>
  auth0 actions deploy <action-id> --json
  auth0 actions deploy <action-id> --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := actionID.Pick(cmd, &inputs.ID, cli.unDeployedActionPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var action *management.Action

			if err := ansi.Waiting(func() (err error) {
				if _, err = cli.api.Action.Deploy(cmd.Context(), inputs.ID); err != nil {
					return fmt.Errorf("failed to deploy action with ID %q: %w", inputs.ID, err)
				}

				if action, err = cli.api.Action.Read(cmd.Context(), inputs.ID); err != nil {
					return fmt.Errorf("failed to read deployed action with ID %q: %w", inputs.ID, err)
				}

				return nil
			}); err != nil {
				return err
			}

			cli.renderer.ActionDeploy(action)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func openActionCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "open",
		Args:  cobra.MaximumNArgs(1),
		Short: "Open the settings page of an action",
		Long:  "Open an action's settings page in the Auth0 Dashboard.",
		Example: `  auth0 actions open
  auth0 actions open <action-id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := actionID.Pick(cmd, &inputs.ID, cli.actionPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			openManageURL(cli, cli.Config.DefaultTenant, formatActionDetailsPath(url.PathEscape(inputs.ID)))

			return nil
		},
	}

	return cmd
}

func diffActionCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID       string
		version1 int
		version2 int
	}

	cmd := &cobra.Command{
		Use:   "diff [action-id]",
		Short: "Show diff between two versions of an Actions",
		Args:  cobra.MaximumNArgs(1),
		Long:  "Show code difference between two versions of an Actions",
		Example: `auth0 actions diff
  auth0 actions diff <action-id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if len(args) == 0 {
				if err := actionID.Pick(cmd, &inputs.ID, cli.actionPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			// Fetch all versions (paginate).
			var allVersions []*management.ActionVersion
			page := 0
			perPage := 50
			for {
				queryParams := []management.RequestOption{
					management.Parameter("page", fmt.Sprintf("%d", page)),
					management.Parameter("per_page", fmt.Sprintf("%d", perPage)),
				}

				versionsResp, err := cli.api.Action.Versions(ctx, inputs.ID, queryParams...)
				if err != nil {
					return err
				}

				allVersions = append(allVersions, versionsResp.Versions...)

				// Check if we've retrieved all available versions using the total count.
				if len(allVersions) >= versionsResp.Total {
					break
				}
				page++
			}

			var err error
			inputs.version1, inputs.version2, err = pickTwoVersions(allVersions)
			if err != nil {
				return err
			}

			var code1, code2 string
			for _, v := range allVersions {
				if v.Number == inputs.version1 {
					code1 = v.GetCode()
				}
				if v.Number == inputs.version2 {
					code2 = v.GetCode()
				}
			}

			if code1 == "" || code2 == "" {
				return fmt.Errorf("There are %d versions for the action. "+
					"\nCould not find one of the versions: %d or %d", len(allVersions), inputs.version1, inputs.version2)
			}

			// Check if the code is identical between versions.
			if code1 == code2 {
				fmt.Printf("No differences found between v%d and v%d - the code is identical\n", inputs.version1, inputs.version2)
				return nil
			}

			printColorDiff(code1, code2, inputs.version1, inputs.version2)
			return nil
		},
	}
	return cmd
}

func (c *cli) actionPickerOptions(ctx context.Context) (pickerOptions, error) {
	list, err := c.api.Action.List(ctx)
	if err != nil {
		return nil, err
	}

	var opts pickerOptions
	for _, r := range list.Actions {
		label := fmt.Sprintf("%s %s", r.GetName(), ansi.Faint("("+r.GetID()+")"))

		opts = append(opts, pickerOption{value: r.GetID(), label: label})
	}

	if len(opts) == 0 {
		return nil, errors.New("there are currently no actions to choose from. Create one by running: `auth0 actions create`")
	}

	return opts, nil
}

func (c *cli) unDeployedActionPickerOptions(ctx context.Context) (pickerOptions, error) {
	list, err := c.api.Action.List(ctx, management.Parameter("deployed", "false"))
	if err != nil {
		return nil, fmt.Errorf("failed to list actions: %w", err)
	}

	var opts pickerOptions
	for _, r := range list.Actions {
		label := fmt.Sprintf("%s %s", r.GetName(), ansi.Faint("("+r.GetID()+")"))

		opts = append(opts, pickerOption{value: r.GetID(), label: label})
	}

	if len(opts) == 0 {
		return nil, errors.New("there are currently no actions to deploy")
	}

	return opts, nil
}

func (c *cli) actionEditorHint() {
	c.renderer.Infof("%s Once you close the editor, the action will be saved. To cancel, press CTRL+C.", ansi.Faint("Hint:"))
}

func formatActionDetailsPath(id string) string {
	if len(id) == 0 {
		return ""
	}
	return fmt.Sprintf("actions/library/details/%s", id)
}

func filterOutDeprecatedActionTriggers(list []*management.ActionTrigger) []*management.ActionTrigger {
	res := []*management.ActionTrigger{}
	for _, t := range list {
		if t.GetStatus() == "CURRENT" {
			res = append(res, t)
		}
	}
	return res
}

func getCurrentTriggers(ctx context.Context, cli *cli) ([]*management.ActionTrigger, error) {
	var triggers []*management.ActionTrigger

	if err := ansi.Waiting(func() error {
		list, err := cli.api.Action.Triggers(ctx)
		if err != nil {
			return err
		}

		triggers = list.Triggers

		return nil
	}); err != nil {
		return nil, err
	}

	return filterOutDeprecatedActionTriggers(triggers), nil
}

func actionTemplate(key string) string {
	t, exists := actionTemplates[key]
	if exists {
		return t
	}
	return actionTemplateEmpty
}

func inputDependenciesToActionDependencies(dependencies map[string]string) *[]management.ActionDependency {
	actionDependencyList := make([]management.ActionDependency, 0)

	for name, version := range dependencies {
		actionDependencyList = append(actionDependencyList, management.ActionDependency{
			Name:    auth0.String(name),
			Version: auth0.String(version),
		})
	}

	return &actionDependencyList
}

func inputSecretsToActionSecrets(secrets map[string]string) *[]management.ActionSecret {
	actionSecrets := make([]management.ActionSecret, 0)

	for name, value := range secrets {
		actionSecrets = append(actionSecrets, management.ActionSecret{
			Name:  auth0.String(name),
			Value: auth0.String(value),
		})
	}

	return &actionSecrets
}

func printColorDiff(code1, code2 string, fromVersion, toVersion int) {
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(code1),
		B:        difflib.SplitLines(code2),
		FromFile: fmt.Sprintf("v%d", fromVersion),
		ToFile:   fmt.Sprintf("v%d", toVersion),
		Context:  3,
	}
	text, _ := difflib.GetUnifiedDiffString(diff)

	if text == "" {
		fmt.Printf("No differences found between v%d and v%d - the code is identical\n", fromVersion, toVersion)
		return
	}

	fmt.Printf("Comparing v%d → v%d:\n\n", fromVersion, toVersion)

	for _, line := range difflib.SplitLines(text) {
		switch {
		case len(line) > 0 && line[0] == '+':
			fmt.Print(ansi.Green(line)) // Green for additions.
		case len(line) > 0 && line[0] == '-':
			fmt.Print(ansi.Red(line)) // Red for deletions.
		case len(line) > 0 && line[0] == '@':
			fmt.Print(ansi.Cyan(line)) // Cyan for hunk headers.
		default:
			fmt.Println(line) // Default color.
		}
	}
}

func pickTwoVersions(versions []*management.ActionVersion) (int, int, error) {
	if len(versions) < 2 {
		return 0, 0, fmt.Errorf("need at least 2 versions to compare, found only %d version. Create more versions to enable comparison", len(versions))
	}

	// Build version options with clear labels showing version numbers, IDs, and deployment status.
	versionOptions := make([]string, len(versions))
	versionNumbers := make([]int, len(versions))

	for i, v := range versions {
		versionNumbers[i] = v.Number
		deployedStatus := ""
		if v.Deployed {
			deployedStatus = " [DEPLOYED]"
		} else {
			deployedStatus = " [DRAFT]"
		}

		versionID := *v.ID

		versionOptions[i] = fmt.Sprintf("v%d (%s)%s", v.Number, versionID, deployedStatus)
	}

	var fromVersionStr, toVersionStr string

	// Ask for first version (baseline).
	if err := prompt.AskOne(&survey.Question{
		Name: "fromVersion",
		Prompt: &survey.Select{
			Message: "Select the first version (baseline):",
			Options: versionOptions,
		},
		Validate: survey.Required,
	}, &fromVersionStr); err != nil {
		return 0, 0, handleInputError(err)
	}

	// Find the selected "from" version number.
	var selectedFromVersion int
	for i, option := range versionOptions {
		if option == fromVersionStr {
			selectedFromVersion = versionNumbers[i]
			break
		}
	}

	// Build filtered options for "to" version (excluding the already selected "from" version).
	var filteredToOptions []string
	var filteredToNumbers []int
	for i, option := range versionOptions {
		if versionNumbers[i] != selectedFromVersion {
			filteredToOptions = append(filteredToOptions, option)
			filteredToNumbers = append(filteredToNumbers, versionNumbers[i])
		}
	}

	// Ask for second version (comparison target) - only show remaining options.
	if err := prompt.AskOne(&survey.Question{
		Name: "toVersion",
		Prompt: &survey.Select{
			Message: "Select the second version (to compare against):",
			Options: filteredToOptions,
		},
		Validate: survey.Required,
	}, &toVersionStr); err != nil {
		return 0, 0, handleInputError(err)
	}

	// Find the selected "to" version number from filtered options.
	var selectedToVersion int
	for i, option := range filteredToOptions {
		if option == toVersionStr {
			selectedToVersion = filteredToNumbers[i]
			break
		}
	}

	return selectedFromVersion, selectedToVersion, nil
}
