package cli

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5"
	"gopkg.in/auth0.v5/management"
)

var (
	apiID = Argument{
		Name: "Id",
		Help: "Id of the API.",
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
	apiTokenLifetime = Flag{
		Name:       "Token Expiration",
		LongForm:   "token-expiration",
		ShortForm:  "e",
		Help:       "The amount of time in seconds that the token will be valid after being issued",
		IsRequired: false,
	}
	apiAllowOfflineAccess = Flag{
		Name:       "Offline Access",
		LongForm:   "offline-access",
		ShortForm:  "o",
		Help:       "Allows issuance of refresh tokens for this API",
		AlwaysPrompt: true,
	}
)

func apisCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "apis",
		Short:   "Manage resources for APIs",
		Long:    "Manage resources for APIs.",
		Aliases: []string{"resource-servers"},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listApisCmd(cli))
	cmd.AddCommand(createApiCmd(cli))
	cmd.AddCommand(showApiCmd(cli))
	cmd.AddCommand(updateApiCmd(cli))
	cmd.AddCommand(deleteApiCmd(cli))
	cmd.AddCommand(openApiCmd(cli))
	cmd.AddCommand(scopesCmd(cli))

	return cmd
}

func scopesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scopes",
		Short: "Manage resources for API scopes",
		Long:  "Manage resources for API scopes.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listScopesCmd(cli))

	return cmd
}

func listApisCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your APIs",
		Long: `List your existing APIs. To create one try:
auth0 apis create`,
		Example: `auth0 apis list
auth0 apis ls`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var list *management.ResourceServerList

			if err := ansi.Waiting(func() error {
				var err error
				list, err = cli.api.ResourceServer.List()
				return err
			}); err != nil {
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
		Long:  "Show an API.",
		Example: `auth0 apis show 
auth0 apis show <id|audience>`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := apiID.Pick(cmd, &inputs.ID, cli.apiPickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var api *management.ResourceServer

			if err := ansi.Waiting(func() error {
				var err error
				api, err = cli.api.ResourceServer.Read(url.PathEscape(inputs.ID))
				return err
			}); err != nil {
				return fmt.Errorf("Unable to get an API with Id '%s': %w", inputs.ID, err)
			}

			cli.renderer.ApiShow(api)
			return nil
		},
	}

	return cmd
}

func createApiCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Name       string
		Identifier string
		Scopes     []string
		TokenLifetime int
		AllowOfflineAccess bool
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new API",
		Long:  "Create a new API.",
		Example: `auth0 apis create 
auth0 apis create --name myapi
auth0 apis create -n myapi --identifier http://my-api
auth0 apis create -n myapi -i http://my-api --offline-access=true --token-expiration 6100`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := apiName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			if err := apiIdentifier.Ask(cmd, &inputs.Identifier, nil); err != nil {
				return err
			}

			if err := apiScopes.AskMany(cmd, &inputs.Scopes, nil); err != nil {
				return err
			}

			if err := apiTokenLifetime.Ask(cmd, &inputs.TokenLifetime, nil); err != nil {
				return err
			}

			apiAllowOfflineAccess.AskBool(cmd, &inputs.AllowOfflineAccess, nil)

			api := &management.ResourceServer{
				Name:       &inputs.Name,
				Identifier: &inputs.Identifier,
				AllowOfflineAccess: &inputs.AllowOfflineAccess,
				TokenLifetime: &inputs.TokenLifetime,
			}

			if len(inputs.Scopes) > 0 {
				api.Scopes = apiScopesFor(inputs.Scopes)
			}

			if err := ansi.Waiting(func() error {
				return cli.api.ResourceServer.Create(api)
			}); err != nil {
				return fmt.Errorf("An unexpected error occurred while attempting to create an API with name '%s' and identifier '%s': %w", inputs.Name, inputs.Identifier, err)
			}

			cli.renderer.ApiCreate(api)
			return nil
		},
	}

	apiName.RegisterString(cmd, &inputs.Name, "")
	apiIdentifier.RegisterString(cmd, &inputs.Identifier, "")
	apiScopes.RegisterStringSlice(cmd, &inputs.Scopes, nil)
	apiAllowOfflineAccess.RegisterBool(cmd, &inputs.AllowOfflineAccess, false)
	apiTokenLifetime.RegisterInt(cmd, &inputs.TokenLifetime, 86400)

	return cmd
}

func updateApiCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID     string
		Name   string
		Scopes []string
		TokenLifetime int
		AllowOfflineAccess bool
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update an API",
		Long:  "Update an API.",
		Example: `auth0 apis update 
auth0 apis update <id|audience> 
auth0 apis update <id|audience> --name myapi
auth0 apis create -n myapi --token-expiration 6100
auth0 apis create -n myapi -t 6100 --offline-access=true`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var current *management.ResourceServer

			if len(args) == 0 {
				err := apiID.Pick(cmd, &inputs.ID, cli.apiPickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			if err := ansi.Waiting(func() error {
				var err error
				current, err = cli.api.ResourceServer.Read(url.PathEscape(inputs.ID))
				return err
			}); err != nil {
				return fmt.Errorf("Unable to load API. The Id %v specified doesn't exist", inputs.ID)
			}

			if err := apiName.AskU(cmd, &inputs.Name, current.Name); err != nil {
				return err
			}

			if err := apiScopes.AskManyU(cmd, &inputs.Scopes, nil); err != nil {
				return err
			}

			if err := apiTokenLifetime.AskU(cmd, &inputs.TokenLifetime, nil); err != nil {
				return err
			}

			apiAllowOfflineAccess.AskBoolU(cmd, &inputs.AllowOfflineAccess, nil)

			api := &management.ResourceServer{
				AllowOfflineAccess: &inputs.AllowOfflineAccess,
				TokenLifetime: &inputs.TokenLifetime,
			}

			if len(inputs.Name) == 0 {
				api.Name = current.Name
			} else {
				api.Name = &inputs.Name
			}

			if len(inputs.Scopes) == 0 {
				api.Scopes = current.Scopes
			} else {
				api.Scopes = apiScopesFor(inputs.Scopes)
			}

			if len(strconv.Itoa(inputs.TokenLifetime)) == 0 {
				api.TokenLifetime = current.TokenLifetime
			} else {
				api.TokenLifetime = &inputs.TokenLifetime
			}

			if len(strconv.FormatBool(inputs.AllowOfflineAccess)) == 0 {
				api.AllowOfflineAccess = current.AllowOfflineAccess
			} else {
				api.AllowOfflineAccess = &inputs.AllowOfflineAccess
			}

			if err := ansi.Waiting(func() error {
				return cli.api.ResourceServer.Update(current.GetID(), api)
			}); err != nil {
				return fmt.Errorf("An unexpected error occurred while trying to update an API with Id '%s': %w", inputs.ID, err)
			}

			cli.renderer.ApiUpdate(api)
			return nil
		},
	}

	apiName.RegisterStringU(cmd, &inputs.Name, "")
	apiScopes.RegisterStringSliceU(cmd, &inputs.Scopes, nil)
	apiAllowOfflineAccess.RegisterBoolU(cmd, &inputs.AllowOfflineAccess, true)
	apiTokenLifetime.RegisterIntU(cmd, &inputs.TokenLifetime, 86400)

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
		Long:  "Delete an API.",
		Example: `auth0 apis delete 
auth0 apis delete <id|audience>`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := apiID.Pick(cmd, &inputs.ID, cli.apiPickerOptions)
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

			return ansi.Spinner("Deleting API", func() error {
				_, err := cli.api.ResourceServer.Read(url.PathEscape(inputs.ID))

				if err != nil {
					return fmt.Errorf("Unable to delete API. The specified Id: %v doesn't exist", inputs.ID)
				}

				return cli.api.ResourceServer.Delete(url.PathEscape(inputs.ID))
			})
		},
	}

	return cmd
}

func openApiCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:     "open",
		Args:    cobra.MaximumNArgs(1),
		Short:   "Open API settings page in Auth0 Manage",
		Long:    "Open API settings page in Auth0 Manage.",
		Example: `auth0 apis open
auth0 apis open <id|audience>`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := apiID.Pick(cmd, &inputs.ID, cli.apiPickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			// Heuristics to determine if this a valid ID, or an audience value
			// Audiences are usually URLs, but not necessarily. Whereas IDs have a length of 24
			// So here if the value is not a URL, we then check if has the length of an ID
			// If the length check fails, we know it's a non-URL audience value
			// This will fail for non-URL audience values with the same length as the ID
			// But it should cover the vast majority of users
			if _, err := url.ParseRequestURI(inputs.ID); err == nil || len(inputs.ID) != 24 {
				if err := ansi.Waiting(func() error {
					api, err := cli.api.ResourceServer.Read(url.PathEscape(inputs.ID))
					if err != nil {
						return err
					}
					inputs.ID = auth0.StringValue(api.ID)
					return nil
				}); err != nil {
					return fmt.Errorf("An unexpected error occurred while trying to get the API Id for '%s': %w", inputs.ID, err)
				}
			}

			openManageURL(cli, cli.config.DefaultTenant, formatApiSettingsPath(inputs.ID))
			return nil
		},
	}

	return cmd
}

func listScopesCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "List the scopes of an API",
		Long:    "List the scopes of an API.",
		Example: `auth0 apis scopes list 
auth0 apis scopes ls <id|audience>`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := apiID.Pick(cmd, &inputs.ID, cli.apiPickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			api := &management.ResourceServer{ID: &inputs.ID}

			if err := ansi.Waiting(func() error {
				var err error
				api, err = cli.api.ResourceServer.Read(url.PathEscape(inputs.ID))
				return err
			}); err != nil {
				return fmt.Errorf("An unexpected error occurred while getting scopes for an API with Id '%s': %w", inputs.ID, err)
			}

			cli.renderer.ScopesList(api.GetName(), api.Scopes)
			return nil
		},
	}

	return cmd
}

func formatApiSettingsPath(id string) string {
	if len(id) == 0 {
		return ""
	}
	return fmt.Sprintf("apis/%s/settings", id)
}

func apiScopesFor(scopes []string) []*management.ResourceServerScope {
	models := []*management.ResourceServerScope{}

	for _, scope := range scopes {
		value := scope
		models = append(models, &management.ResourceServerScope{Value: &value})
	}

	return models
}

func (c *cli) apiPickerOptions() (pickerOptions, error) {
	list, err := c.api.ResourceServer.List()
	if err != nil {
		return nil, err
	}

	// NOTE: because client names are not unique, we'll just number these
	// labels.
	var opts pickerOptions
	for _, r := range list.ResourceServers {
		label := fmt.Sprintf("%s %s", r.GetName(), ansi.Faint("("+r.GetIdentifier()+")"))

		opts = append(opts, pickerOption{value: r.GetID(), label: label})
	}

	if len(opts) == 0 {
		return nil, errors.New("There are currently no applications.")
	}

	return opts, nil
}
