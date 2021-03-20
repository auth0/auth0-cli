package cli

import (
	"fmt"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

var (
	apiID = Argument{
		Name:       "Id",
		Help:       "Id of the API.",
		IsRequired: true,
	}
	apiName = Flag{
		Name:       "Name",
		LongForm:   "name",
		ShortForm:  "n",
		Help:       "Name of the API.",
		IsRequired: true,
	}
	apiIdentifier = Flag{
		Name:       "Identifier",
		LongForm:   "identifier",
		ShortForm:  "i",
		Help:       "Identifier of the API. Cannot be changed once set.",
		IsRequired: true,
	}
	apiScopes = Flag{
		Name:       "Scopes",
		LongForm:   "scopes",
		ShortForm:  "s",
		Help:       "Comma-separated list of scopes.",
		IsRequired: true,
	}
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
	cmd.AddCommand(scopesCmd(cli))

	return cmd
}

func scopesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scopes",
		Short: "Manage resources for API scopes",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listScopesCmd(cli))

	return cmd
}

func listApisCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List your APIs",
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
				return fmt.Errorf("An unexpected error occurred: %w", err)
			}

			cli.renderer.ApiList(list.ResourceServers)
			return nil
		},
	}

	return cmd
}

func showApiCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show an API",
		Long: `Show an API:

auth0 apis show <id>
`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := apiID.Ask(cmd, &inputs.ID); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			api := &management.ResourceServer{ID: &inputs.ID}

			err := ansi.Spinner("Loading API", func() error {
				var err error
				api, err = cli.api.ResourceServer.Read(inputs.ID)
				return err
			})

			if err != nil {
				return fmt.Errorf("Unable to get an API with Id '%s': %w", inputs.ID, err)
			}

			cli.renderer.ApiShow(api)
			cli.renderer.Newline()
			cli.renderer.Infof("To see the full scope list, run %s", ansi.Faint("apis scopes list <api-id>"))
			return nil
		},
	}

	return cmd
}

func createApiCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Name         string
		Identifier   string
		Scopes       []string
		ScopesString string
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
			if err := apiName.Ask(cmd, &inputs.Name); err != nil {
				return err
			}

			if err := apiIdentifier.Ask(cmd, &inputs.Identifier); err != nil {
				return err
			}

			if err := apiScopes.Ask(cmd, &inputs.ScopesString); err != nil {
				return err
			}

			api := &management.ResourceServer{
				Name:       &inputs.Name,
				Identifier: &inputs.Identifier,
			}

			if len(inputs.ScopesString) > 0 {
				api.Scopes = apiScopesFor(commaSeparatedStringToSlice(inputs.ScopesString))
			} else if len(inputs.Scopes) > 0 {
				api.Scopes = apiScopesFor(inputs.Scopes)
			}

			err := ansi.Spinner("Creating API", func() error {
				return cli.api.ResourceServer.Create(api)
			})

			if err != nil {
				return fmt.Errorf("An unexpected error occurred while attempting to create an API with name '%s' and identifier '%s': %w", inputs.Name, inputs.Identifier, err)
			}

			cli.renderer.ApiCreate(api)
			return nil
		},
	}

	apiName.RegisterString(cmd, &inputs.Name, "")
	apiIdentifier.RegisterString(cmd, &inputs.Identifier, "")
	apiScopes.RegisterStringSlice(cmd, &inputs.Scopes, nil)

	return cmd
}

func updateApiCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID           string
		Name         string
		Scopes       []string
		ScopesString string
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update an API",
		Long: `Update an API:

auth0 apis update <id> --name myapi
`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := apiID.Ask(cmd, &inputs.ID); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			if err := apiName.AskU(cmd, &inputs.Name); err != nil {
				return err
			}

			if err := apiScopes.AskU(cmd, &inputs.ScopesString); err != nil {
				return err
			}

			api := &management.ResourceServer{}

			err := ansi.Spinner("Updating API", func() error {
				current, err := cli.api.ResourceServer.Read(inputs.ID)

				if err != nil {
					return fmt.Errorf("Unable to load API. The Id %v specified doesn't exist", inputs.ID)
				}

				if len(inputs.Name) == 0 {
					api.Name = current.Name
				} else {
					api.Name = &inputs.Name
				}

				if len(inputs.Scopes) == 0 {
					if len(inputs.ScopesString) == 0 {
						api.Scopes = current.Scopes
					} else {
						api.Scopes = apiScopesFor(commaSeparatedStringToSlice(inputs.ScopesString))
					}
				} else {
					api.Scopes = apiScopesFor(inputs.Scopes)
				}

				return cli.api.ResourceServer.Update(inputs.ID, api)
			})

			if err != nil {
				return fmt.Errorf("An unexpected error occurred while trying to update an API with Id '%s': %w", inputs.ID, err)
			}

			cli.renderer.ApiUpdate(api)
			return nil
		},
	}

	apiName.RegisterStringU(cmd, &inputs.Name, "")
	apiScopes.RegisterStringSliceU(cmd, &inputs.Scopes, nil)

	return cmd
}

func deleteApiCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "delete",
		Args:  cobra.MaximumNArgs(1),
		Short: "Delete an API",
		Long: `Delete an API:

auth0 apis delete <id>
`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := apiID.Ask(cmd, &inputs.ID); err != nil {
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

			return ansi.Spinner("Deleting API", func() error {
				err := cli.api.ResourceServer.Delete(inputs.ID)
				if err != nil {
					return fmt.Errorf("An unexpected error occurred while attempting to delete an API with Id '%s': %w", inputs.ID, err)
				}
				return nil
			})
		},
	}

	return cmd
}

func listScopesCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "list",
		Args:  cobra.MaximumNArgs(1),
		Short: "List the scopes of an API",
		Long: `List the scopes of an API:

auth0 apis scopes list <id>
`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := apiID.Ask(cmd, &inputs.ID); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			api := &management.ResourceServer{ID: &inputs.ID}

			err := ansi.Spinner("Loading scopes", func() error {
				var err error
				api, err = cli.api.ResourceServer.Read(inputs.ID)
				return err
			})

			if err != nil {
				return fmt.Errorf("An unexpected error occurred while getting scopes for an API with Id '%s': %w", inputs.ID, err)
			}

			cli.renderer.ScopesList(api.GetName(), api.Scopes)
			return nil
		},
	}

	return cmd
}

func apiScopesFor(scopes []string) []*management.ResourceServerScope {
	models := []*management.ResourceServerScope{}

	for _, scope := range scopes {
		value := scope
		models = append(models, &management.ResourceServerScope{Value: &value})
	}

	return models
}
