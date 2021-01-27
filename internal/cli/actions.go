package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/auth0/auth0-cli/internal/validators"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

func actionsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "actions",
		Short: "manage resources for actions.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listActionsCmd(cli))
	cmd.AddCommand(testActionCmd(cli))
	cmd.AddCommand(createActionCmd(cli))
	cmd.AddCommand(deployActionCmd(cli))
	cmd.AddCommand(downloadActionCmd(cli))
	cmd.AddCommand(listActionVersionsCmd(cli))
	cmd.AddCommand(triggersCmd(cli))

	return cmd
}

func triggersCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "triggers",
		Short: "manage resources for action triggers.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(showTriggerCmd(cli))
	cmd.AddCommand(reorderTriggerCmd(cli))
	cmd.AddCommand(createTriggerCmd(cli))

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
	var actionId string
	var versionId string
	var payloadFile string
	var payload = make(management.Object)

	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test an action draft against a payload",
		Long:  `$ auth0 actions test --name <actionid> --file <payload.json>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := readJsonFile(payloadFile, &payload)
			if err != nil {
				return err
			}

			var result management.Object
			err = ansi.Spinner(fmt.Sprintf("Testing action: %s, version: %s", actionId, versionId), func() error {
				result, err = cli.api.ActionVersion.Test(actionId, versionId, payload)
				return err
			})

			if err != nil {
				return err
			}

			cli.renderer.ActionTest(result)
			return nil
		},
	}

	cmd.Flags().StringVar(&actionId, "name", "", "Action ID to to test")
	cmd.Flags().StringVarP(&payloadFile, "file", "f", "", "File containing the payload for the test")
	cmd.Flags().StringVarP(&versionId, "version", "v", "draft", "Version ID of the action to test")

	mustRequireFlags(cmd, "name", "file")

	return cmd
}

func deployActionCmd(cli *cli) *cobra.Command {
	var actionId string
	var versionId string

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploys the action version",
		Long:  `$ auth0 actions deploy --name <actionid> --version <versionid>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var version *management.ActionVersion
			err := ansi.Spinner(fmt.Sprintf("Deploying action: %s, version: %s", actionId, versionId), func() (err error) {
				version, err = cli.api.ActionVersion.Deploy(actionId, versionId)
				return err
			})

			if err != nil {
				return err
			}

			cli.renderer.ActionVersion(version)

			return nil
		},
	}

	cmd.Flags().StringVar(&actionId, "name", "", "Action ID to deploy")
	cmd.Flags().StringVarP(&versionId, "version", "v", "draft", "Version ID of the action to deploy")

	mustRequireFlags(cmd, "name")

	return cmd
}

func downloadActionCmd(cli *cli) *cobra.Command {
	var actionId string
	var versionId string
	var path string

	cmd := &cobra.Command{
		Use:   "download",
		Short: "Download the action version",
		Long:  `$ auth0 actions download --name <actionid> --version <versionid | draft>`,
		RunE: func(cmd *cobra.Command, args []string) error {

			versions, err := cli.api.ActionVersion.List(actionId)
			var options []string
			options = append(options, "draft")

			for _, v := range versions.Versions {
				options = append(options, fmt.Sprint(v.Number))
			}

			var versionNumber string
			prompt.AskOne(prompt.SelectInput("Actions version", "Choose a version to download", options, "draft"), &versionNumber)
			if versionNumber == "draft" {
				versionId = "draft"
			} else {
				i, err := strconv.Atoi(versionNumber)
				if err != nil {
					return err
				}

				versionId = versions.Versions[len(versions.Versions)-i].ID
			}

			cli.renderer.Infof("It will overwrite files in %s", path)
			if confirmed := prompt.Confirm("Do you wish to proceed?"); !confirmed {
				return nil
			}

			var version *management.ActionVersion
			err = ansi.Spinner(fmt.Sprintf("Downloading action: %s, version: %s", actionId, versionId), func() (err error) {
				if version, err = cli.api.ActionVersion.Read(actionId, versionId); err != nil {
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

			cli.renderer.Infof("Code downloaded to %s/code.js", path)

			if err := ioutil.WriteFile(path+"/code.js", []byte(version.Code), 0644); err != nil {
				return err
			}

			version.Code = ""
			metadata, err := json.MarshalIndent(version, "", "    ")
			if err != nil {
				return err
			}

			if err := ioutil.WriteFile(path+"/metadata.json", metadata, 0644); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&actionId, "name", "", "Action ID to deploy")
	cmd.Flags().StringVarP(&versionId, "version", "v", "draft", "Version ID of the action to deploy or draft, default: draft")
	cmd.Flags().StringVarP(&path, "path", "p", "./", "Path to save the action content")

	mustRequireFlags(cmd, "name")

	return cmd
}

func listActionVersionsCmd(cli *cli) *cobra.Command {
	var actionId string

	cmd := &cobra.Command{
		Use:   "versions",
		Short: "Lists the action versions",
		Long:  `$ auth0 actions versions --name <actionid>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var list *management.ActionVersionList
			err := ansi.Spinner(fmt.Sprintf("Loading versions for action: %s", actionId), func() (err error) {
				list, err = cli.api.ActionVersion.List(actionId)
				return err
			})

			if err != nil {
				return err
			}

			cli.renderer.ActionVersionList(list.Versions)

			return nil
		},
	}

	cmd.Flags().StringVar(&actionId, "name", "", "Action ID to show versions")

	mustRequireFlags(cmd, "name")

	return cmd
}

func createActionCmd(cli *cli) *cobra.Command {
	var (
		name          string
		trigger       string
		file          string
		script        string
		dependency    []string
		createVersion bool
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Creates a new action",
		Long: `$ auth0 actions create
Creates a new action:

    $ auth0 actions create --name my-action --trigger post-login --file action.js --dependency lodash@4.17.19
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validators.TriggerID(trigger); err != nil {
				return err
			}

			source, err := sourceFromFileOrScript(file, script)
			if err != nil {
				return err
			}

			dependencies, err := validators.Dependencies(dependency)
			if err != nil {
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
				Name:              auth0.String(name),
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

				if createVersion {
					if err := cli.api.ActionVersion.Create(auth0.StringValue(action.ID), version); err != nil {
						return err
					}

					// TODO(iamjem): this is a hack since the SDK won't decode 202 responses
					list, err := cli.api.ActionVersion.List(auth0.StringValue(action.ID))
					if err != nil {
						return err
					}

					if len(list.Versions) > 0 {
						version = list.Versions[0]
					}
				} else {
					if err := cli.api.ActionVersion.UpsertDraft(auth0.StringValue(action.ID), version); err != nil {
						return err
					}

					// TODO(iamjem): this is a hack since the SDK won't decode 202 responses
					draft, err := cli.api.ActionVersion.ReadDraft(auth0.StringValue(action.ID))
					if err != nil {
						return err
					}
					version = draft
				}

				return nil
			})

			if err != nil {
				return err
			}

			cli.renderer.ActionVersion(version)

			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Unique name for the action.")
	cmd.Flags().StringVarP(&trigger, "trigger", "t", string(management.PostLogin), "Trigger type for action.")
	cmd.Flags().StringVarP(&file, "file", "f", "", "File containing the action source code.")
	cmd.Flags().StringVarP(&script, "script", "s", "", "Raw source code for the action.")
	cmd.Flags().StringSliceVarP(&dependency, "dependency", "d", nil, "Dependency for the source code (<name>@<semver>).")
	// TODO: This name is kind of overloaded since it could also refer to the version of the trigger (though there's only v1's at this time)
	cmd.Flags().BoolVarP(&createVersion, "version", "v", false, "Create an explicit action version from the source code instead of a draft.")

	mustRequireFlags(cmd, "name")
	if err := cmd.MarkFlagFilename("file"); err != nil {
		panic(err)
	}

	return cmd
}

func showTriggerCmd(cli *cli) *cobra.Command {
	var trigger string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show actions by trigger",
		Long:  `$ auth0 actions triggers show --trigger post-login`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validators.TriggerID(trigger); err != nil {
				return err
			}

			triggerID := management.TriggerID(trigger)

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

	cmd.Flags().StringVarP(&trigger, "trigger", "t", string(management.PostLogin), "Trigger type for action.")

	return cmd
}

func reorderTriggerCmd(cli *cli) *cobra.Command {
	var trigger string
	var bindingsFile string

	cmd := &cobra.Command{
		Use:   "reorder",
		Short: "Reorders actions by trigger",
		Long:  `$ auth0 actions triggers reorder --trigger <post-login> --file <bindings.json>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validators.TriggerID(trigger); err != nil {
				return err
			}

			triggerID := management.TriggerID(trigger)

			var list *management.ActionBindingList
			err := readJsonFile(bindingsFile, &list)
			if err != nil {
				return err
			}

			err = ansi.Spinner("Reordoring actions", func() (err error) {
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

	cmd.Flags().StringVarP(&trigger, "trigger", "t", string(management.PostLogin), "Trigger type for action.")
	cmd.Flags().StringVarP(&bindingsFile, "file", "f", "", "File containing the bindings")

	return cmd
}

func createTriggerCmd(cli *cli) *cobra.Command {
	var trigger string
	var actionId string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Bind an action to a trigger",
		Long:  `$ auth0 actions triggers create --trigger <post-login> --name <action_id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validators.TriggerID(trigger); err != nil {
				return err
			}

			triggerID := management.TriggerID(trigger)

			var binding *management.ActionBinding
			var list *management.ActionBindingList

			err := ansi.Spinner("Adding action", func() (err error) {
				var action *management.Action
				if action, err = cli.api.Action.Read(actionId); err != nil {
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

	cmd.Flags().StringVarP(&trigger, "trigger", "t", string(management.PostLogin), "Trigger type for action.")
	cmd.Flags().StringVar(&actionId, "name", "", "Action ID to to test")

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
