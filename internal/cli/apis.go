package cli

import (
	"github.com/auth0/auth0-cli/internal/ansi"
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
		name       string
		identifier string
	}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new API",
		Long: `Creates a new API:

auth0 apis create --name myapi --identifier http://my-api
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			api := &management.ResourceServer{
				Name:       &flags.name,
				Identifier: &flags.identifier,
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

	cmd.Flags().StringVarP(&flags.name, "name", "n", "", "Name of the API.")
	cmd.Flags().StringVarP(&flags.identifier, "identifier", "i", "", "Identifier of the API.")

	mustRequireFlags(cmd, "name", "identifier")

	return cmd
}

func updateApiCmd(cli *cli) *cobra.Command {
	var flags struct {
		id   string
		name string
	}

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update an API",
		Long: `Updates an API:

auth0 apis update --id id --name myapi
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			api := &management.ResourceServer{Name: &flags.name}
			manager := cli.api.ResourceServer

			err := ansi.Spinner("Updating API", func() error {
				return manager.Update(flags.id, api)
			})

			if err != nil {
				return err
			}

			cli.renderer.ApiUpdate(api)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.id, "id", "i", "", "ID of the API.")
	cmd.Flags().StringVarP(&flags.name, "name", "n", "", "Name of the API.")

	mustRequireFlags(cmd, "id", "name")

	return cmd
}

func deleteApiCmd(cli *cli) *cobra.Command {
	var flags struct {
		id string
	}

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an API",
		Long: `Deletes an API:

auth0 apis delete --id id
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := cli.api.ResourceServer

			err := ansi.Spinner("Deleting API", func() error {
				return manager.Delete(flags.id)
			})

			if err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.id, "id", "i", "", "ID of the API.")

	mustRequireFlags(cmd, "id")

	return cmd
}
