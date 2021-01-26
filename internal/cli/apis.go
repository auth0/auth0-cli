package cli

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

func apisCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apis",
		Short: "manage resources for APIs.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listApisCmd(cli))
	cmd.AddCommand(createApiCmd(cli))
	cmd.AddCommand(updateApiCmd(cli))
	cmd.AddCommand(deleteApiCmd(cli))

	return cmd
}

func listApisCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lists your existing APIs",
		Long: `$ auth0 apis list
Lists your existing APIs. To create one try:

    $ auth0 apis create
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var list *management.ResourceServerList

			err := ansi.Spinner("Getting APIs", func() error {
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

func createApiCmd(cli *cli) *cobra.Command {
	var flags struct {
		Name       string
		Identifier string
	}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new API",
		Long: `Creates a new API:

auth0 apis create --name myapi --identifier http://my-api
`,
		PreRun: func(cmd *cobra.Command, args []string) {
			checkFlags(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !hasFlags(cmd) {
				name := prompt.TextInput("name", "Name:", "Name of the API. You can change the API name later in the API settings.", true)
				identifier := prompt.TextInput("identifier", "Identifier:", "Identifier of the API. Cannot be changed once set.", true)

				if err := prompt.Ask([]*survey.Question {name, identifier}, &flags); err != nil {
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

	cmd.Flags().StringVarP(&flags.Name, "name", "n", "", "Name of the API.")
	cmd.Flags().StringVarP(&flags.Identifier, "identifier", "i", "", "Identifier of the API.")

	mustRequireFlags(cmd, "name", "identifier")

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
		Long: `Updates an API:

auth0 apis update --id id --name myapi
`,
		PreRun: func(cmd *cobra.Command, args []string) {
			checkFlags(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !hasFlags(cmd) {
				id := prompt.TextInput("id", "Id:", "Id of the API.", true)
				name := prompt.TextInput("name", "Name:", "Name of the API.", true)

				if err := prompt.Ask([]*survey.Question {id, name}, &flags); err != nil {
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

	cmd.Flags().StringVarP(&flags.ID, "id", "i", "", "ID of the API.")
	cmd.Flags().StringVarP(&flags.Name, "name", "n", "", "Name of the API.")

	mustRequireFlags(cmd, "id", "name")

	return cmd
}

func deleteApiCmd(cli *cli) *cobra.Command {
	var flags struct {
		id    string
		force bool
	}

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an API",
		Long: `Deletes an API:

auth0 apis delete --id id --force
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !flags.force {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}
		
			err := ansi.Spinner("Deleting API", func() error {
				return cli.api.ResourceServer.Delete(flags.id)
			})

			if err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.id, "id", "i", "", "ID of the API.")
	cmd.Flags().BoolVarP(&flags.force, "force", "f", false, "Do not ask for confirmation.")

	return cmd
}
