package cli

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/auth0/go-auth0/management"
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

	actionTemplates = map[string]string{
		"post-login":             actionTemplatePostLogin,
		"credentials-exchange":   actionTemplateCredentialsExchange,
		"pre-user-registration":  actionTemplatePreUserRegistration,
		"post-user-registration": actionTemplatePostUserRegistration,
		"post-change-password":   actionTemplatePostChangePassword,
		"send-phone-message":     actionTemplateSendPhoneMessage,
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
  auth0 actions ls --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var list *management.ActionList

			if err := ansi.Waiting(func() error {
				var err error
				list, err = cli.api.Action.List()
				return err
			}); err != nil {
				return fmt.Errorf("An unexpected error occurred: %w", err)
			}

			cli.renderer.ActionList(list.Actions)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

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
  auth0 actions show <action-id> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := actionID.Pick(cmd, &inputs.ID, cli.actionPickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var action *management.Action

			if err := ansi.Waiting(func() error {
				var err error
				action, err = cli.api.Action.Read(inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("Unable to get an action with ID '%s': %w", inputs.ID, err)
			}

			cli.renderer.ActionShow(action)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

	return cmd
}

func createActionCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Name         string
		Trigger      string
		Code         string
		Dependencies map[string]string
		Secrets      map[string]string
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
  auth0 actions create --name myaction --trigger post-login --code "$(cat path/to/code.js)"
  auth0 actions create --name myaction --trigger post-login --code "$(cat path/to/code.js)" --dependency "lodash=4.0.0"
  auth0 actions create --name myaction --trigger post-login --code "$(cat path/to/code.js)" --dependency "lodash=4.0.0" --secret "SECRET=value"
  auth0 actions create --name myaction --trigger post-login --code "$(cat path/to/code.js)" --dependency "lodash=4.0.0" --dependency "uuid=9.0.0" --secret "API_KEY=value" --secret "SECRET=value"
  auth0 actions create -n myaction -t post-login -c "$(cat path/to/code.js)" -d "lodash=4.0.0" -d "uuid=9.0.0" -s "API_KEY=value" -s "SECRET=value" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := actionName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			triggers, err := getCurrentTriggers(cli)
			if err != nil {
				return err
			}

			triggerIds := make([]string, 0)
			for _, t := range triggers {
				triggerIds = append(triggerIds, t.GetID())
			}

			if err := actionTrigger.Select(cmd, &inputs.Trigger, triggerIds, nil); err != nil {
				return err
			}

			// TODO(cyx): we can re-think this once we have
			// `--stdin` based commands. For now we don't have
			// those yet, so keeping this simple.
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
				Code:         &inputs.Code,
				Dependencies: inputDependenciesToActionDependencies(inputs.Dependencies),
				Secrets:      inputSecretsToActionSecrets(inputs.Secrets),
			}

			if err := ansi.Waiting(func() error {
				return cli.api.Action.Create(action)
			}); err != nil {
				return fmt.Errorf("An unexpected error occurred while attempting to create an action with name '%s': %w", inputs.Name, err)
			}

			cli.renderer.ActionCreate(action)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	actionName.RegisterString(cmd, &inputs.Name, "")
	actionTrigger.RegisterString(cmd, &inputs.Trigger, "")
	actionCode.RegisterString(cmd, &inputs.Code, "")
	actionDependency.RegisterStringMap(cmd, &inputs.Dependencies, nil)
	actionSecret.RegisterStringMap(cmd, &inputs.Secrets, nil)

	return cmd
}

func updateActionCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID           string
		Name         string
		Code         string
		Dependencies map[string]string
		Secrets      map[string]string
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
  auth0 actions update <action-id> --name myaction
  auth0 actions update <action-id> --name myaction --code "$(cat path/to/code.js)"
  auth0 actions update <action-id> --name myaction --code "$(cat path/to/code.js)" --dependency "lodash=4.0.0"
  auth0 actions update <action-id> --name myaction --code "$(cat path/to/code.js)" --dependency "lodash=4.0.0" --secret "SECRET=value"
  auth0 actions update <action-id> --name myaction --code "$(cat path/to/code.js)" --dependency "lodash=4.0.0" --dependency "uuid=9.0.0" --secret "API_KEY=value" --secret "SECRET=value"
  auth0 actions update <action-id> -n myaction -t post-login -c "$(cat path/to/code.js)" -d "lodash=4.0.0" -d "uuid=9.0.0" -s "API_KEY=value" -s "SECRET=value" --json`,
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
				oldAction, err = cli.api.Action.Read(inputs.ID)
				return err
			})
			if err != nil {
				return fmt.Errorf("failed to fetch action with ID %s: %w", inputs.ID, err)
			}

			if err := actionName.AskU(cmd, &inputs.Name, oldAction.Name); err != nil {
				return err
			}

			if err := actionCode.OpenEditorU(
				cmd,
				&inputs.Code,
				oldAction.GetCode(),
				inputs.Name+".*.js",
				cli.actionEditorHint,
			); err != nil {
				return fmt.Errorf("failed to capture input from the editor: %w", err)
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

			if err = ansi.Waiting(func() error {
				return cli.api.Action.Update(oldAction.GetID(), updatedAction)
			}); err != nil {
				return fmt.Errorf("failed to update action with ID %s: %w", oldAction.GetID(), err)
			}

			cli.renderer.ActionUpdate(updatedAction)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	actionName.RegisterStringU(cmd, &inputs.Name, "")
	actionCode.RegisterStringU(cmd, &inputs.Code, "")
	actionDependency.RegisterStringMapU(cmd, &inputs.Dependencies, nil)
	actionSecret.RegisterStringMapU(cmd, &inputs.Secrets, nil)

	return cmd
}

func deleteActionCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "Delete an action",
		Long: "Delete an action.\n\n" +
			"To delete interactively, use `auth0 actions delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the action id and the `--force` flag to skip confirmation.",
		Example: `  auth0 actions delete
  auth0 actions rm
  auth0 actions delete <action-id>
  auth0 actions delete <action-id> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := actionID.Pick(cmd, &inputs.ID, cli.actionPickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			return ansi.Spinner("Deleting action", func() error {
				_, err := cli.api.Action.Read(inputs.ID)
				if err != nil {
					return fmt.Errorf("Unable to delete action: %w", err)
				}

				return cli.api.Action.Delete(inputs.ID)
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
  auth0 actions deploy <action-id> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := actionID.Pick(cmd, &inputs.ID, cli.actionPickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var action *management.Action

			if err := ansi.Waiting(func() error {
				var err error
				if _, err = cli.api.Action.Deploy(inputs.ID); err != nil {
					return fmt.Errorf("Unable to deploy an action with Id '%s': %w", inputs.ID, err)
				}
				if action, err = cli.api.Action.Read(inputs.ID); err != nil {
					return fmt.Errorf("Unable to get deployed action with Id '%s': %w", inputs.ID, err)
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
				err := actionID.Pick(cmd, &inputs.ID, cli.actionPickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			openManageURL(cli, cli.config.DefaultTenant, formatActionDetailsPath(url.PathEscape(inputs.ID)))
			return nil
		},
	}

	return cmd
}

func (c *cli) actionPickerOptions() (pickerOptions, error) {
	list, err := c.api.Action.List()
	if err != nil {
		return nil, err
	}

	var opts pickerOptions
	for _, r := range list.Actions {
		label := fmt.Sprintf("%s %s", r.GetName(), ansi.Faint("("+r.GetID()+")"))

		opts = append(opts, pickerOption{value: r.GetID(), label: label})
	}

	if len(opts) == 0 {
		return nil, errors.New("There are currently no actions.")
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

func filterDeprecatedActionTriggers(list []*management.ActionTrigger) []*management.ActionTrigger {
	res := []*management.ActionTrigger{}
	for _, t := range list {
		if t.GetStatus() == "CURRENT" {
			res = append(res, t)
		}
	}
	return res
}

func getCurrentTriggers(cli *cli) ([]*management.ActionTrigger, error) {
	var triggers []*management.ActionTrigger
	if err := ansi.Waiting(func() error {
		list, err := cli.api.Action.Triggers()
		if err != nil {
			return err
		}
		triggers = list.Triggers
		return nil
	}); err != nil {
		return nil, err
	}

	return filterDeprecatedActionTriggers(triggers), nil
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
			Name:    &name,
			Version: &version,
		})
	}

	return &actionDependencyList
}

func inputSecretsToActionSecrets(secrets map[string]string) *[]management.ActionSecret {
	actionSecrets := make([]management.ActionSecret, 0)

	for name, value := range secrets {
		actionSecrets = append(actionSecrets, management.ActionSecret{
			Name:  &name,
			Value: &value,
		})
	}

	return &actionSecrets
}
