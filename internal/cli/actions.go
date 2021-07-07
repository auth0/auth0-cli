package cli

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
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
		Help:      "Third party npm module, and it version, that the action depends on.",
	}

	actionSecret = Flag{
		Name:      "Secret",
		LongForm:  "secret",
		ShortForm: "s",
		Help:      "Secret to be used in the action.",
	}

	actionTemplates = map[string]string{
		"post-login":             actionTemplatePostLogin,
		"credentials-exchange":   actionTemplateCredentialsEchange,
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
		Long:  "Manage resources for actions.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listActionsCmd(cli))
	cmd.AddCommand(createActionCmd(cli))
	cmd.AddCommand(showActionCmd(cli))
	cmd.AddCommand(updateActionCmd(cli))
	cmd.AddCommand(deleteActionCmd(cli))
	cmd.AddCommand(openActionCmd(cli))

	return cmd
}

func listActionsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your actions",
		Long: `List your existing actions. To create one try:
auth0 actions create`,
		Example: `auth0 actions list
auth0 actions ls`,
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
		Long:  "Show an action.",
		Example: `auth0 actions show 
auth0 actions show <id>`,
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
				action, err = cli.api.Action.Read(url.PathEscape(inputs.ID))
				return err
			}); err != nil {
				return fmt.Errorf("Unable to get an action with Id '%s': %w", inputs.ID, err)
			}

			cli.renderer.ActionShow(action)
			return nil
		},
	}

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
		Long:  "Create a new action.",
		Example: `auth0 actions create 
auth0 actions create --name myaction
auth0 actions create --n myaction --trigger post-login
auth0 actions create --n myaction -t post-login -d "lodash=4.0.0" -d "uuid=8.0.0"
auth0 actions create --n myaction -t post-login -d "lodash=4.0.0" -s "API_KEY=value" -s "SECRET=value`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := actionName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			triggers, version, err := latestActionTriggers(cli)
			if err != nil {
				return err
			}

			if err := actionTrigger.Select(cmd, &inputs.Trigger, triggers, nil); err != nil {
				return err
			}

			// TODO(cyx): we can re-think this once we have
			// `--stdin` based commands. For now we don't have
			// those yet, so keeping this simple.
			if err := actionCode.EditorPrompt(
				cmd,
				&inputs.Code,
				actionTemplate(inputs.Trigger),
				inputs.Name+".*.js",
				cli.actionEditorHint,
			); err != nil {
				return err
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
				Dependencies: apiActionDependenciesFor(inputs.Dependencies),
				Secrets:      apiActionSecretsFor(inputs.Secrets),
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
		Trigger      string
		Code         string
		Dependencies map[string]string
		Secrets      map[string]string
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update an action",
		Long:  "Update an action.",
		Example: `auth0 actions update <id> 
auth0 actions update <id> --name myaction
auth0 actions update <id> --n myaction --trigger post-login
auth0 actions update <id> --n myaction -t post-login -d "lodash=4.0.0" -d "uuid=8.0.0"
auth0 actions update <id> --n myaction -t post-login -d "lodash=4.0.0" -s "API_KEY=value" -s "SECRET=value`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				err := actionID.Pick(cmd, &inputs.ID, cli.actionPickerOptions)
				if err != nil {
					return err
				}
			}

			var current *management.Action
			err := ansi.Waiting(func() error {
				var err error
				current, err = cli.api.Action.Read(inputs.ID)
				return err
			})
			if err != nil {
				return fmt.Errorf("Failed to fetch action with ID: %s %v", inputs.ID, err)
			}

			if err := actionName.AskU(cmd, &inputs.Name, current.Name); err != nil {
				return err
			}

			triggers, version, err := latestActionTriggers(cli)
			if err != nil {
				return err
			}

			var currentTriggerId = ""
			if len(current.SupportedTriggers) > 0 {
				currentTriggerId = current.SupportedTriggers[0].GetID()
			}

			if err := actionTrigger.SelectU(cmd, &inputs.Trigger, triggers, &currentTriggerId); err != nil {
				return err
			}

			// TODO(cyx): we can re-think this once we have
			// `--stdin` based commands. For now we don't have
			// those yet, so keeping this simple.
			if err := actionCode.EditorPromptU(
				cmd,
				&inputs.Code,
				current.GetCode(),
				inputs.Name+".*.js",
				cli.actionEditorHint,
			); err != nil {
				return err
			}
			if err != nil {
				return fmt.Errorf("Failed to capture input from the editor: %w", err)
			}

			if inputs.Name == "" {
				inputs.Name = current.GetName()
			}

			if inputs.Trigger == "" && currentTriggerId != "" {
				inputs.Trigger = currentTriggerId
			}

			// Prepare action payload for update. This will also be
			// re-hydrated by the SDK, which we'll use below during
			// display.
			action := &management.Action{
				Name: &inputs.Name,
				SupportedTriggers: []management.ActionTrigger{
					{
						ID:      &inputs.Trigger,
						Version: &version,
					},
				},
				Code: &inputs.Code,
			}

			if len(inputs.Dependencies) == 0 {
				action.Dependencies = current.Dependencies
			} else {
				action.Dependencies = apiActionDependenciesFor(inputs.Dependencies)
			}

			if len(inputs.Secrets) == 0 {
				action.Secrets = current.Secrets
			} else {
				action.Secrets = apiActionSecretsFor(inputs.Secrets)
			}

			if err = ansi.Waiting(func() error {
				return cli.api.Action.Update(inputs.ID, action)
			}); err != nil {
				return err
			}

			cli.renderer.ActionUpdate(current)
			return nil
		},
	}

	actionName.RegisterStringU(cmd, &inputs.Name, "")
	actionTrigger.RegisterStringU(cmd, &inputs.Trigger, "")
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
		Use:   "delete",
		Args:  cobra.MaximumNArgs(1),
		Short: "Delete an action",
		Long:  "Delete an action.",
		Example: `auth0 actions delete 
auth0 actions delete <id>`,
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
				_, err := cli.api.Action.Read(url.PathEscape(inputs.ID))

				if err != nil {
					return fmt.Errorf("Unable to delete action: %w", err)
				}

				return cli.api.Action.Delete(url.PathEscape(inputs.ID))
			})
		},
	}

	return cmd
}

func openActionCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:     "open",
		Args:    cobra.MaximumNArgs(1),
		Short:   "Open action details page in the Auth0 Dashboard",
		Long:    "Open action details page in the Auth0 Dashboard.",
		Example: "auth0 actions open <id>",
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
	c.renderer.Infof("%s once you close the editor, the action will be saved. To cancel, CTRL+C.", ansi.Faint("Hint:"))
}

func formatActionDetailsPath(id string) string {
	if len(id) == 0 {
		return ""
	}
	return fmt.Sprintf("actions/library/details/%s", id)
}

func latestActionTriggerVersion(list []*management.ActionTrigger) string {
	latestVersion := "v1"
	for _, t := range list {
		if t.GetVersion() > latestVersion {
			latestVersion = t.GetVersion()
		}
	}
	return latestVersion
}

func filterActionTriggersByVersion(list []*management.ActionTrigger, version string) []*management.ActionTrigger {
	res := []*management.ActionTrigger{}
	for _, t := range list {
		if t.GetVersion() == version && t.GetStatus() == "CURRENT" {
			res = append(res, t)
		}
	}
	return res
}

func latestActionTriggers(cli *cli) ([]string, string, error) {
	var triggers []*management.ActionTrigger
	if err := ansi.Waiting(func() error {
		list, err := cli.api.Action.ListTriggers()
		if err != nil {
			return err
		}
		triggers = list.Triggers
		return nil
	}); err != nil {
		return nil, "", err
	}

	latestTriggerVersion := latestActionTriggerVersion(triggers)
	triggers = filterActionTriggersByVersion(triggers, latestTriggerVersion)
	var triggerIds []string

	for _, t := range triggers {
		triggerIds = append(triggerIds, t.GetID())
	}
	return triggerIds, latestTriggerVersion, nil
}

func actionTemplate(key string) string {
	t, exists := actionTemplates[key]
	if exists {
		return t
	}
	return actionTemplateEmpty
}

func apiActionDependenciesFor(dependencies map[string]string) []management.ActionDependency {
	var res []management.ActionDependency
	for k, v := range dependencies {
		key := k
		value := v
		res = append(res, management.ActionDependency{
			Name:    &key,
			Version: &value,
		})
	}
	return res
}

func apiActionSecretsFor(secrets map[string]string) []management.ActionSecret {
	var res []management.ActionSecret
	for k, v := range secrets {
		key := k
		value := v
		res = append(res, management.ActionSecret{
			Name:  &key,
			Value: &value,
		})
	}
	return res
}
