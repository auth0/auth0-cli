package cli

import (
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

const (
	apiID         = "id"
	apiName       = "name"
	apiIdentifier = "identifier"
)

func apisCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "apis",
		Short:   "Manage resources for APIs",
		Aliases: []string{"resource-servers"},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listApisCmd(cli))
	cmd.AddCommand(showApiCmd(cli))
	cmd.AddCommand(createApiCmd(cli))
	cmd.AddCommand(updateApiCmd(cli))
	cmd.AddCommand(deleteApiCmd(cli))

	return cmd
}

func listApisCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List your existing APIs",
		Long: `auth0 apis list
Lists your existing APIs. To create one try:

    auth0 apis create
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var list *management.ResourceServerList

			err := ansi.Spinner("Loading APIs", func() error {
				var err error
				list, err = cli.api.ResourceServer.List()
				return err
			})

			if err != nil {
				return err
			}

			cli.renderer.ApiList(list.ResourceServers)
			return nil
		},
	}

	return cmd
}

func showApiCmd(cli *cli) *cobra.Command {
	var flags struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show an API",
		Long: `Show an API:

auth0 apis show --id id
`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if shouldPrompt(cmd, apiID) {
				input := prompt.TextInput(apiID, "Id:", "Id of the API.", "", true)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			api := &management.ResourceServer{ID: &flags.ID}

			err := ansi.Spinner("Loading API", func() error {
				var err error
				api, err = cli.api.ResourceServer.Read(flags.ID)
				return err
			})

			if err != nil {
				return err
			}

			cli.renderer.ApiShow(api)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.ID, apiID, "i", "", "ID of the API.")
	mustRequireFlags(cmd, apiID)

	return cmd
}

func createApiCmd(cli *cli) *cobra.Command {
	var flags struct {
		Name       string
		Identifier string
	}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new API",
		Long: `Create a new API:

auth0 apis create --name myapi --identifier http://my-api
`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if shouldPrompt(cmd, apiName) {
				input := prompt.TextInput(
					apiName, "Name:",
					"Name of the API. You can change the API name later in the API settings.",
					"",
					true)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			if shouldPrompt(cmd, apiIdentifier) {
				input := prompt.TextInput(
					apiIdentifier, "Identifier:",
					"Identifier of the API. Cannot be changed once set.",
					"",
					true)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			api := &management.ResourceServer{
				Name:       &flags.Name,
				Identifier: &flags.Identifier,
			}

			err := ansi.Spinner("Creating API", func() error {
				return cli.api.ResourceServer.Create(api)
			})

			if err != nil {
				return err
			}

			cli.renderer.ApiCreate(api)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.Name, apiName, "n", "", "Name of the API.")
	cmd.Flags().StringVarP(&flags.Identifier, apiIdentifier, "i", "", "Identifier of the API.")
	mustRequireFlags(cmd, apiName, apiIdentifier)

	return cmd
}

func updateApiCmd(cli *cli) *cobra.Command {
	var flags struct {
		ID   string
		Name string
	}

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update an API",
		Long: `Update an API:

auth0 apis update --id id --name myapi
`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if shouldPrompt(cmd, apiID) {
				input := prompt.TextInput(apiID, "Id:", "Id of the API.", "", true)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			if shouldPrompt(cmd, apiName) {
				input := prompt.TextInput(apiName, "Name:", "Name of the API.", "", true)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			api := &management.ResourceServer{Name: &flags.Name}

			err := ansi.Spinner("Updating API", func() error {
				return cli.api.ResourceServer.Update(flags.ID, api)
			})

			if err != nil {
				return err
			}

			cli.renderer.ApiUpdate(api)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.ID, apiID, "i", "", "ID of the API.")
	cmd.Flags().StringVarP(&flags.Name, apiName, "n", "", "Name of the API.")
	mustRequireFlags(cmd, apiID, apiName)

	return cmd
}

func deleteApiCmd(cli *cli) *cobra.Command {
	var flags struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an API",
		Long: `Delete an API:

auth0 apis delete --id id
`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if shouldPrompt(cmd, apiID) {
				input := prompt.TextInput(apiID, "Id:", "Id of the API.", "", true)

				if err := prompt.AskOne(input, &flags); err != nil {
					return err
				}
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			err := ansi.Spinner("Deleting API", func() error {
				return cli.api.ResourceServer.Delete(flags.ID)
			})

			if err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.ID, apiID, "i", "", "ID of the API.")
	mustRequireFlags(cmd, apiID)

	return cmd
}
