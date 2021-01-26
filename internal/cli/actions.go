package cli

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/validators"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5"
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
	cmd.AddCommand(triggersCmd(cli))

	return cmd
}

func triggersCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "triggers",
		Short: "manage resources for actions triggers.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(showTriggerCmd(cli))
	cmd.AddCommand(reorderTriggerCmd(cli))

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
		fmt.Println(err)
	}
	// defer the closing of our jsonFile so that we can parse it later on
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)

	err = json.Unmarshal([]byte(byteValue), &out)
	if err != nil {
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
				result, err = cli.api.ActionVersion.Test(actionId, "draft", payload)
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

func createActionCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Creates a new action",
		Long: `$ auth0 actions create
Creates a new action:

    $ auth0 actions create my-action --trigger post-login
`,
		Args: func(cmd *cobra.Command, args []string) error {
			if err := validators.ExactArgs("name")(cmd, args); err != nil {
				return err
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			trigger, err := cmd.LocalFlags().GetString("trigger")
			if err != nil {
				return err
			}

			if err := validators.TriggerID(trigger); err != nil {
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
				Name:              auth0.String(args[0]),
				SupportedTriggers: &triggers,
			}

			err = ansi.Spinner("Creating action", func() error {
				return cli.api.Action.Create(action)
			})

			if err != nil {
				return err
			}

			cli.renderer.ActionCreate(action)
			return nil
		},
	}

	cmd.LocalFlags().StringP("trigger", "t", string(management.PostLogin), "Trigger type for action.")

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
			err := ansi.Spinner("Loading actions", func() error {
				var err error
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

			err = ansi.Spinner("Loading actions", func() error {
				return cli.api.ActionBinding.Update(triggerID, list)
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
