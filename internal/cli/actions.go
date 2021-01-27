package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/auth0/auth0-cli/internal/validators"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

const (
	actionID         = "id"
	actionName       = "name"
	actionVersion    = "version"
	actionFile       = "file"
	actionScript     = "script"
	actionDependency = "dependencies"
	actionPath       = "path"
	actionTrigger    = "trigger"
)

func actionsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "actions",
		Short: "Manage resources for actions",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listActionsCmd(cli))
	cmd.AddCommand(testActionCmd(cli))
	cmd.AddCommand(createActionCmd(cli))
	cmd.AddCommand(updateActionCmd(cli))
	cmd.AddCommand(deployActionCmd(cli))
	cmd.AddCommand(downloadActionCmd(cli))
	cmd.AddCommand(listActionVersionsCmd(cli))
	cmd.AddCommand(bindActionCmd(cli))

	cmd.AddCommand(flowsCmd(cli))

	return cmd
}

func flowsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "flows",
		Short: "Manages resources for action flows",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(showFlowCmd(cli))
	cmd.AddCommand(updateFlowCmd(cli))

	return cmd
}

func listActionsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List existing actions",
		Long: `auth0 actions list
List existing actions. To create one try:

    auth0 actions create
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

func readJsonFile(filePath string, out interface{}) error {
	// Open our jsonFile
	jsonFile, err := os.Open(filePath)
	// if we os.Open returns an error then handle it
	if err != nil {
		return err
	}
	// defer the closing of our jsonFile so that we can parse it later on
	defer jsonFile.Close()

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(byteValue, out); err != nil {
		return err
	}

	return nil
}

func testActionCmd(cli *cli) *cobra.Command {
	var flags struct {
		ID      string
		File    string
		Version string
	}

	var payload = make(management.Object)

	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test an action draft against a payload",
		Long:  `auth0 actions test --id <actionid> --file <payload.json>`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if shouldPrompt(cmd, actionID) {
				input := prompt.TextInput(actionID, "Id:", "Action Id to test.", true)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			if shouldPrompt(cmd, actionFile) {
				input := prompt.TextInput(actionFile, "File:", "File containing the payload for the test.", true)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			if shouldPrompt(cmd, actionVersion) {
				input := prompt.TextInputDefault(actionVersion, "Version Id:", "Version ID of the action to test. Default: draft", "draft", false)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			err := readJsonFile(flags.File, &payload)
			if err != nil {
				return err
			}

			var result management.Object
			err = ansi.Spinner(fmt.Sprintf("Testing action: %s, version: %s", flags.ID, flags.Version), func() error {
				result, err = cli.api.ActionVersion.Test(flags.ID, flags.Version, payload)
				return err
			})

			if err != nil {
				return err
			}

			cli.renderer.ActionTest(result)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.ID, actionID, "i", "", "Action Id to test.")
	cmd.Flags().StringVarP(&flags.File, actionFile, "f", "", "File containing the payload for the test.")
	cmd.Flags().StringVarP(&flags.Version, actionVersion, "v", "draft", "Version Id of the action to test.")
	mustRequireFlags(cmd, actionID, actionFile)

	return cmd
}

func deployActionCmd(cli *cli) *cobra.Command {
	var flags struct {
		ID      string
		Version string
	}

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy the action version",
		Long:  `auth0 actions deploy --id <actionid> --version <versionid>`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if shouldPrompt(cmd, actionID) {
				input := prompt.TextInput(actionID, "Id:", "Action Id to deploy.", true)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			if shouldPrompt(cmd, actionVersion) {
				version, err := askVersion(cli, flags.ID)
				if err != nil {
					return err
				}
				flags.Version = version
			}

			var version *management.ActionVersion
			err := ansi.Spinner(fmt.Sprintf("Deploying action: %s, version: %s", flags.ID, flags.Version), func() (err error) {
				version, err = cli.api.ActionVersion.Deploy(flags.ID, flags.Version)
				return err
			})

			if err != nil {
				return err
			}

			cli.renderer.ActionVersion(version)

			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.ID, actionID, "i", "", "Action Id to deploy.")
	cmd.Flags().StringVarP(&flags.Version, actionVersion, "v", "draft", "Version Id of the action to deploy.")
	mustRequireFlags(cmd, actionID)

	return cmd
}

func renderVersionOptionText(v *management.ActionVersion) string {
	deployed := ""

	if v.Deployed {
		deployed = "[DEPLOYED]"
	}

	return fmt.Sprintf("#%d %s %s", v.Number, v.ID, deployed)
}

func askVersion(cli *cli, actionId string) (string, error) {
	// var versionId string
	versions, err := cli.api.ActionVersion.List(actionId)
	if err != nil {
		return "", err
	}

	optChoices := make(map[string]string)
	options := make([]string, 0)
	options = append(options, "Draft")
	optChoices["Draft"] = "draft"

	for _, v := range versions.Versions {
		optText := renderVersionOptionText(v)
		optChoices[optText] = v.ID
		options = append(options, optText)
	}

	var versionLabel string
	if err = prompt.AskOne(prompt.SelectInput("Actions version", "Choose a version", "Select the version number you want to choose for this action", options, true), &versionLabel); err != nil {
		return "", err
	}

	return optChoices[versionLabel], nil
}

func downloadActionCmd(cli *cli) *cobra.Command {
	var flags struct {
		ID      string
		Version string
		Path    string
	}

	cmd := &cobra.Command{
		Use:   "download",
		Short: "Download the action version",
		Long:  `auth0 actions download --id <actionid> --version <versionid | draft>`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if shouldPrompt(cmd, actionID) {
				input := prompt.TextInput(actionID, "Id:", "Action Id to download.", true)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			if shouldPrompt(cmd, actionVersion) {
				version, err := askVersion(cli, flags.ID)
				if err != nil {
					return err
				}
				flags.Version = version
			}

			if shouldPrompt(cmd, actionPath) {
				input := prompt.TextInputDefault(actionPath, "Path:", "Path to save the action content.", "./", false)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			cli.renderer.Infof("It will overwrite files in %s", flags.Path)
			if confirmed := prompt.Confirm("Do you wish to proceed?"); !confirmed {
				return nil
			}

			var version *management.ActionVersion
			err := ansi.Spinner(fmt.Sprintf("Downloading action: %s, version: %s", flags.ID, flags.Version), func() (err error) {
				if version, err = cli.api.ActionVersion.Read(flags.ID, flags.Version); err != nil {
					return err
				}

				if version.ID == "" {
					version.ID = "draft"
				}
				return nil
			})

			if err != nil {
				return err
			}

			cli.renderer.Infof("Code downloaded to %s/code.js", flags.Path)

			if err := ioutil.WriteFile(flags.Path+"/code.js", []byte(version.Code), 0644); err != nil {
				return err
			}

			version.Code = ""
			metadata, err := json.MarshalIndent(version, "", "    ")
			if err != nil {
				return err
			}

			if err := ioutil.WriteFile(flags.Path+"/metadata.json", metadata, 0644); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.ID, actionID, "i", "", "Action ID to download.")
	cmd.Flags().StringVarP(&flags.Version, actionVersion, "v", "", "Version ID of the action to deploy or draft. Default: draft")
	cmd.Flags().StringVarP(&flags.Path, actionPath, "p", "./", "Path to save the action content.")
	mustRequireFlags(cmd, actionID)

	return cmd
}

func listActionVersionsCmd(cli *cli) *cobra.Command {
	var flags struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "versions",
		Short: "List the action versions",
		Long:  `auth0 actions versions --id <actionid>`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if shouldPrompt(cmd, actionID) {
				input := prompt.TextInput(actionID, "Id:", "Action Id to show versions.", true)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			var list *management.ActionVersionList
			err := ansi.Spinner(fmt.Sprintf("Loading versions for action: %s", flags.ID), func() (err error) {
				list, err = cli.api.ActionVersion.List(flags.ID)
				return err
			})

			if err != nil {
				return err
			}

			cli.renderer.ActionVersionList(list.Versions)

			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.ID, actionID, "i", "", "Action Id to show versions.")
	mustRequireFlags(cmd, actionID)

	return cmd
}

func createActionCmd(cli *cli) *cobra.Command {
	var flags struct {
		Name          string
		Trigger       string
		File          string
		Script        string
		Dependency    []string
		CreateVersion bool
	}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new action",
		Long: `auth0 actions create
Create a new action:

    auth0 actions create --name my-action --trigger post-login --file action.js --dependency lodash@4.17.19
`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if shouldPrompt(cmd, actionName) {
				input := prompt.TextInput(actionName, "Name:", "Action name to create.", true)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			if shouldPrompt(cmd, actionTrigger) {
				input := prompt.SelectInput(
					actionTrigger,
					"Trigger:",
					"Trigger type for action.",
					validators.ValidTriggerIDs,
					false)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			if shouldPrompt(cmd, actionFile) && shouldPrompt(cmd, actionScript) {
				input := prompt.TextInput(actionFile, "Action File:", "File containing the action source code.", false)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			// TODO: Add prompt for script, dependency and version

			if err := validators.TriggerID(flags.Trigger); err != nil {
				return err
			}

			source, err := sourceFromFileOrScript(flags.File, flags.Script)
			if err != nil {
				return err
			}

			dependencies, err := validators.Dependencies(flags.Dependency)
			if err != nil {
				return err
			}

			triggerID := management.TriggerID(flags.Trigger)
			triggers := []management.Trigger{
				{
					ID:      &triggerID,
					Version: auth0.String("v1"),
				},
			}

			action := &management.Action{
				Name:              auth0.String(flags.Name),
				SupportedTriggers: &triggers,
			}

			version := &management.ActionVersion{
				Code:         source,
				Dependencies: dependencies,
				Runtime:      "node12",
			}

			err = ansi.Spinner("Creating action", func() error {
				if err := cli.api.Action.Create(action); err != nil {
					return err
				}

				created, err := createActionVersion(cli.api, auth0.StringValue(action.ID), !flags.CreateVersion, version)
				if err != nil {
					return err
				}

				version = created

				return nil
			})

			if err != nil {
				return err
			}

			cli.renderer.ActionVersion(version)

			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.Name, actionName, "n", "", "Action name to create.")
	cmd.Flags().StringVarP(&flags.Trigger, actionTrigger, "t", string(management.PostLogin), "Trigger type for action.")
	cmd.Flags().StringVarP(&flags.File, actionFile, "f", "", "File containing the action source code.")
	cmd.Flags().StringVarP(&flags.Script, actionScript, "s", "", "Raw source code for the action.")
	cmd.Flags().StringSliceVarP(&flags.Dependency, actionDependency, "d", nil, "Dependency for the source code (<name>@<semver>).")
	// TODO: This name is kind of overloaded since it could also refer to the version of the trigger (though there's only v1's at this time)
	cmd.Flags().BoolVarP(&flags.CreateVersion, actionVersion, "v", false, "Create an explicit action version from the source code instead of a draft.")

	mustRequireFlags(cmd, actionName)
	if err := cmd.MarkFlagFilename(actionFile); err != nil {
		panic(err)
	}

	return cmd
}

func updateActionCmd(cli *cli) *cobra.Command {
	var flags struct {
		ID            string
		File          string
		Script        string
		Dependency    []string
		CreateVersion bool
	}

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Updates an existing action",
		Long: `$ auth0 actions update
Updates an existing action:

    $ auth0 actions update --id <actionid> --file action.js --dependency lodash@4.17.19
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if shouldPrompt(cmd, actionFile) && shouldPrompt(cmd, actionScript) {
				input := prompt.TextInput(actionFile, "Action File:", "File containing the action source code.", false)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			source, err := sourceFromFileOrScript(flags.File, flags.Script)
			if err != nil {
				return err
			}

			dependencies, err := validators.Dependencies(flags.Dependency)
			if err != nil {
				return err
			}

			version := &management.ActionVersion{
				Code:         source,
				Dependencies: dependencies,
				Runtime:      "node12",
			}

			err = ansi.Spinner("Updating action", func() error {
				created, err := createActionVersion(cli.api, flags.ID, !flags.CreateVersion, version)
				if err != nil {
					return err
				}

				version = created
				return nil
			})

			if err != nil {
				return err
			}

			cli.renderer.ActionVersion(version)

			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.ID, actionName, "n", "", "Action ID to update.")
	cmd.Flags().StringVarP(&flags.File, actionFile, "f", "", "File containing the action source code.")
	cmd.Flags().StringVarP(&flags.Script, actionScript, "s", "", "Raw source code for the action.")
	cmd.Flags().StringSliceVarP(&flags.Dependency, actionDependency, "d", nil, "Dependency for the source code (<name>@<semver>).")
	// TODO: This name is kind of overloaded since it could also refer to the version of the trigger (though there's only v1's at this time)
	cmd.Flags().BoolVarP(&flags.CreateVersion, actionVersion, "v", false, "Create an explicit action version from the source code instead of a draft.")

	mustRequireFlags(cmd, actionName)
	if err := cmd.MarkFlagFilename(actionFile); err != nil {
		panic(err)
	}

	return cmd
}

func showFlowCmd(cli *cli) *cobra.Command {
	var flags struct {
		Trigger string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Shows actions by flow",
		Long:  `auth0 actions flows --trigger post-login`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if shouldPrompt(cmd, actionTrigger) {
				input := prompt.SelectInput(
					actionTrigger,
					"Trigger:",
					"Trigger type for action.",
					validators.ValidTriggerIDs,
					false)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			if err := validators.TriggerID(flags.Trigger); err != nil {
				return err
			}

			triggerID := management.TriggerID(flags.Trigger)

			var list *management.ActionBindingList
			err := ansi.Spinner("Loading actions", func() (err error) {
				list, err = cli.api.ActionBinding.List(triggerID)
				return err
			})

			if err != nil {
				return err
			}

			cli.renderer.ActionTriggersList(list.Bindings)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.Trigger, actionTrigger, "t", string(management.PostLogin), "Trigger type for action.")

	return cmd
}

func updateFlowCmd(cli *cli) *cobra.Command {
	var flags struct {
		File    string
		Trigger string
	}

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update actions by flow",
		Long:  `auth0 actions flows update --trigger <post-login> --file <bindings.json>`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if shouldPrompt(cmd, actionFile) {
				input := prompt.TextInput(actionFile, "File:", "File containing the bindings.", true)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			if shouldPrompt(cmd, actionTrigger) {
				input := prompt.SelectInput(
					actionTrigger,
					"Trigger:",
					"Trigger type for action.",
					validators.ValidTriggerIDs,
					false)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			if err := validators.TriggerID(flags.Trigger); err != nil {
				return err
			}

			triggerID := management.TriggerID(flags.Trigger)

			var list *management.ActionBindingList
			err := readJsonFile(flags.File, &list)
			if err != nil {
				return err
			}

			err = ansi.Spinner("Updating actions", func() (err error) {
				if _, err = cli.api.ActionBinding.Update(triggerID, list.Bindings); err != nil {
					return err
				}

				list, err = cli.api.ActionBinding.List(triggerID)

				return err
			})

			if err != nil {
				return err
			}

			cli.renderer.ActionTriggersList(list.Bindings)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.File, actionFile, "f", "", "File containing the bindings.")
	cmd.Flags().StringVarP(&flags.Trigger, actionTrigger, "t", string(management.PostLogin), "Trigger type for action.")
	mustRequireFlags(cmd, actionFile)

	return cmd
}

func bindActionCmd(cli *cli) *cobra.Command {
	var flags struct {
		Action  string
		Trigger string
	}

	cmd := &cobra.Command{
		Use:   "bind",
		Short: "Bind an action to a flow",
		Long:  `auth0 actions bind --trigger <post-login> --action <action_id>`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if shouldPrompt(cmd, actionTrigger) {
				input := prompt.SelectInput(
					actionTrigger,
					"Trigger:",
					"Trigger type for action.",
					validators.ValidTriggerIDs,
					false)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			if shouldPrompt(cmd, "action") {
				input := prompt.TextInput("action", "Action Id:", "Action Id to bind.", false)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			if err := validators.TriggerID(flags.Trigger); err != nil {
				return err
			}

			triggerID := management.TriggerID(flags.Trigger)

			var binding *management.ActionBinding
			var list *management.ActionBindingList

			err := ansi.Spinner("Adding action", func() (err error) {
				var action *management.Action
				if action, err = cli.api.Action.Read(flags.Action); err != nil {
					return err
				}

				if binding, err = cli.api.ActionBinding.Create(triggerID, action); err != nil {
					return err
				}

				if list, err = cli.api.ActionBinding.List(triggerID); err != nil {
					return err
				}

				bindings := append(list.Bindings, binding)

				if _, err = cli.api.ActionBinding.Update(triggerID, bindings); err != nil {
					return err
				}

				list, err = cli.api.ActionBinding.List(triggerID)

				return err
			})

			if err != nil {
				return err
			}

			cli.renderer.ActionTriggersList(list.Bindings)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.Trigger, actionTrigger, "t", string(management.PostLogin), "Trigger type for action.")
	cmd.Flags().StringVarP(&flags.Action, "action", "a", "", "Action Id to bind.")

	return cmd
}

var errNoSource = errors.New("please provide source code via --file or --script")

func sourceFromFileOrScript(file, script string) (string, error) {
	if script != "" {
		return script, nil
	}

	if file != "" {
		f, err := os.Open(file)
		if err != nil {
			return "", err
		}
		defer f.Close()

		contents, err := ioutil.ReadAll(f)
		if err != nil {
			return "", err
		}

		if len(contents) > 0 {
			return string(contents), nil
		}
	}

	return "", errNoSource
}

func createActionVersion(api *auth0.API, actionID string, draft bool, version *management.ActionVersion) (*management.ActionVersion, error) {
	var v *management.ActionVersion
	if draft {
		if err := api.ActionVersion.UpsertDraft(actionID, version); err != nil {
			return nil, err
		}

		// TODO(iamjem): this is a hack since the SDK won't decode 202 responses
		draft, err := api.ActionVersion.ReadDraft(actionID)
		if err != nil {
			return nil, err
		}
		v = draft
	} else {
		if err := api.ActionVersion.Create(actionID, version); err != nil {
			return nil, err
		}

		// TODO(iamjem): this is a hack since the SDK won't decode 202 responses
		list, err := api.ActionVersion.List(actionID)
		if err != nil {
			return nil, err
		}

		if len(list.Versions) > 0 {
			v = list.Versions[0]
		}
	}

	return v, nil
}
