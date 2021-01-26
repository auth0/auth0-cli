package cli

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/auth0/auth0-cli/internal/ansi"
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
			// Open our jsonFile
			jsonFile, err := os.Open(payloadFile)
			// if we os.Open returns an error then handle it
			if err != nil {
				fmt.Println(err)
			}
			// defer the closing of our jsonFile so that we can parse it later on
			defer jsonFile.Close()

			byteValue, _ := ioutil.ReadAll(jsonFile)

			json.Unmarshal([]byte(byteValue), &payload)

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
	cmd.MarkFlagRequired("name")
	cmd.Flags().StringVarP(&payloadFile, "file", "f", "", "File containing the payload for the test")
	cmd.MarkFlagRequired("file")
	cmd.Flags().StringVarP(&versionId, "version", "v", "draft", "Version ID of the action to test")

	return cmd
}
