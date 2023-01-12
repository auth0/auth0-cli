package cli

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/auth0/go-auth0"
	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
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
		Name:         "Scopes",
		LongForm:     "scopes",
		ShortForm:    "s",
		Help:         "Comma-separated list of scopes (permissions).",
		AlwaysPrompt: true,
	}
	apiTokenLifetime = Flag{
		Name:         "Token Lifetime",
		LongForm:     "token-lifetime",
		ShortForm:    "l",
		Help:         "The amount of time in seconds that the token will be valid after being issued. Default value is 86400 seconds (1 day).",
		AlwaysPrompt: true,
	}
	apiOfflineAccess = Flag{
		Name:         "Allow Offline Access",
		LongForm:     "offline-access",
		ShortForm:    "o",
		Help:         "Whether Refresh Tokens can be issued for this API (true) or not (false).",
		AlwaysPrompt: true,
	}
	apiNumber = Flag{
		Name:      "Number",
		LongForm:  "number",
		ShortForm: "n",
		Help:      "Number of APIs, that match the search criteria, to retrieve. Maximum result number is 1000.",
	}
)

func apisCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apis",
		Short: "Manage resources for APIs",
		Long: "Manage resources for APIs. An API is an entity that represents an external resource, capable of " +
			"accepting and responding to protected resource requests made by applications. " +
			"In the OAuth2 specification, an API maps to the Resource Server.",
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
		Long:  "API Scopes define the specific actions applications can be allowed to do on a user's behalf.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listScopesCmd(cli))

	return cmd
}

func listApisCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Number int
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your APIs",
		Long:    "List your existing APIs. To create one, run: `auth0 apis create`.",
		Example: `  auth0 apis list
  auth0 apis ls
  auth0 apis ls --number 100
  auth0 apis ls -n 100 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Number < 1 || inputs.Number > 1000 {
				return fmt.Errorf("number flag invalid, please pass a number between 1 and 1000")
			}

			list, err := getWithPagination(
				cmd.Context(),
				inputs.Number,
				func(opts ...management.RequestOption) (result []interface{}, hasNext bool, err error) {
					apiList, err := cli.api.ResourceServer.List(opts...)
					if err != nil {
						return nil, false, err
					}

					for _, api := range apiList.ResourceServers {
						result = append(result, api)
					}

					return result, apiList.HasNext(), nil
				},
			)
			if err != nil {
				return fmt.Errorf("An unexpected error occurred while listing apis: %w", err)
			}

			var apis []*management.ResourceServer
			for _, item := range list {
				apis = append(apis, item.(*management.ResourceServer))
			}

			cli.renderer.ApiList(apis)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	apiNumber.RegisterInt(cmd, &inputs.Number, defaultPageSize)

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
		Long:  "Display the name, scopes, token lifetime, and other information about an API.",
		Example: `  auth0 apis show
  auth0 apis show <api-id|api-audience>
  auth0 apis show <api-id|api-audience> --json`,
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

			cli.renderer.ApiShow(api, cli.json)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

	return cmd
}

func createApiCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Name               string
		Identifier         string
		Scopes             []string
		TokenLifetime      int
		AllowOfflineAccess bool
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new API",
		Long: "Create a new API.\n\n" +
			"To create interactively, use `auth0 apis create` with no flags.\n\n" +
			"To create non-interactively, supply the name, identifier, scopes, " +
			"token lifetime and whether to allow offline access through the flags.",
		Example: `  auth0 apis create 
  auth0 apis create --name myapi
  auth0 apis create --name myapi --identifier http://my-api
  auth0 apis create --name myapi --identifier http://my-api --token-lifetime 6100
  auth0 apis create --name myapi --identifier http://my-api --token-lifetime 6100 --offline-access
  auth0 apis create --name myapi --identifier http://my-api --token-lifetime 6100 --offline-access false --scopes "letter:write,letter:read"
  auth0 apis create -n myapi -i http://my-api -t 6100 -o false -s "letter:write,letter:read" --json`,
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

			defaultTokenLifetime := strconv.Itoa(apiDefaultTokenLifetime())
			if err := apiTokenLifetime.Ask(cmd, &inputs.TokenLifetime, &defaultTokenLifetime); err != nil {
				return err
			}

			if err := apiOfflineAccess.AskBool(cmd, &inputs.AllowOfflineAccess, nil); err != nil {
				return err
			}

			api := &management.ResourceServer{
				Name:               &inputs.Name,
				Identifier:         &inputs.Identifier,
				AllowOfflineAccess: &inputs.AllowOfflineAccess,
				TokenLifetime:      &inputs.TokenLifetime,
			}

			if len(inputs.Scopes) > 0 {
				api.Scopes = apiScopesFor(inputs.Scopes)
			}

			// Set token lifetime
			if inputs.TokenLifetime <= 0 {
				api.TokenLifetime = auth0.Int(apiDefaultTokenLifetime())
			} else {
				api.TokenLifetime = auth0.Int(inputs.TokenLifetime)
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

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	apiName.RegisterString(cmd, &inputs.Name, "")
	apiIdentifier.RegisterString(cmd, &inputs.Identifier, "")
	apiScopes.RegisterStringSlice(cmd, &inputs.Scopes, nil)
	apiOfflineAccess.RegisterBool(cmd, &inputs.AllowOfflineAccess, false)
	apiTokenLifetime.RegisterInt(cmd, &inputs.TokenLifetime, 0)

	return cmd
}

func updateApiCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID                 string
		Name               string
		Scopes             []string
		TokenLifetime      int
		AllowOfflineAccess bool
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update an API",
		Long: "Update an API.\n\n" +
			"To update interactively, use `auth0 apis update` with no arguments.\n\n" +
			"To update non-interactively, supply the name, identifier, scopes, " +
			"token lifetime and whether to allow offline access through the flags.",
		Example: `  auth0 apis update 
  auth0 apis update <api-id|api-audience>
  auth0 apis update <api-id|api-audience> --name myapi
  auth0 apis update <api-id|api-audience> --name myapi --token-lifetime 6100
  auth0 apis update <api-id|api-audience> --name myapi --token-lifetime 6100 --offline-access false
  auth0 apis update <api-id|api-audience> --name myapi --token-lifetime 6100 --offline-access false --scopes "letter:write,letter:read"
  auth0 apis update <api-id|api-audience> -n myapi -t 6100 -o false -s "letter:write,letter:read" --json`,
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
				return fmt.Errorf("Unable to load API: %w", err)
			}

			if err := apiName.AskU(cmd, &inputs.Name, current.Name); err != nil {
				return err
			}

			if err := apiScopes.AskManyU(cmd, &inputs.Scopes, nil); err != nil {
				return err
			}

			currentTokenLifetime := strconv.Itoa(auth0.IntValue(current.TokenLifetime))
			if err := apiTokenLifetime.AskU(cmd, &inputs.TokenLifetime, &currentTokenLifetime); err != nil {
				return err
			}

			if !apiOfflineAccess.IsSet(cmd) {
				inputs.AllowOfflineAccess = auth0.BoolValue(current.AllowOfflineAccess)
			}

			if err := apiOfflineAccess.AskBoolU(cmd, &inputs.AllowOfflineAccess, current.AllowOfflineAccess); err != nil {
				return err
			}

			api := &management.ResourceServer{
				AllowOfflineAccess: &inputs.AllowOfflineAccess,
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

			if inputs.TokenLifetime == 0 {
				api.TokenLifetime = current.TokenLifetime
			} else {
				api.TokenLifetime = &inputs.TokenLifetime
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

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	apiName.RegisterStringU(cmd, &inputs.Name, "")
	apiScopes.RegisterStringSliceU(cmd, &inputs.Scopes, nil)
	apiOfflineAccess.RegisterBoolU(cmd, &inputs.AllowOfflineAccess, false)
	apiTokenLifetime.RegisterIntU(cmd, &inputs.TokenLifetime, 0)

	return cmd
}

func deleteApiCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "Delete an API",
		Long: "Delete an API.\n\n" +
			"To delete interactively, use `auth0 apis delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the API id and the `--force` flag to skip confirmation.",
		Example: `  auth0 apis delete 
  auth0 apis rm
  auth0 apis delete <api-id|api-audience>
  auth0 apis delete <api-id|api-audience> --force`,
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
					return fmt.Errorf("Unable to delete API: %w", err)
				}

				return cli.api.ResourceServer.Delete(url.PathEscape(inputs.ID))
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

func openApiCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "open",
		Args:  cobra.MaximumNArgs(1),
		Short: "Open the settings page of an API",
		Long:  "Open an APIs' settings page in the Auth0 Dashboard.",
		Example: `  auth0 apis open
  auth0 apis open <api-id|api-audience>`,
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
		Long:    "List the scopes of an API. To update scopes, run: `auth0 apis update <id|audience> -s <scopes>`.",
		Example: `  auth0 apis scopes list
  auth0 apis scopes ls <api-id|api-audience>
  auth0 apis scopes ls <api-id|api-audience> --json`,
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

			cli.renderer.ScopesList(api.GetName(), api.GetScopes())
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

	return cmd
}

func formatApiSettingsPath(id string) string {
	if len(id) == 0 {
		return ""
	}
	return fmt.Sprintf("apis/%s/settings", id)
}

func apiScopesFor(scopes []string) *[]management.ResourceServerScope {
	models := make([]management.ResourceServerScope, 0)

	for _, scope := range scopes {
		value := scope
		models = append(models, management.ResourceServerScope{Value: &value})
	}

	return &models
}

func apiDefaultTokenLifetime() int {
	return 86400
}

func (c *cli) apiPickerOptions() (pickerOptions, error) {
	return c.filteredAPIPickerOptions(func(r *management.ResourceServer) bool {
		return true
	})
}

func (c *cli) filteredAPIPickerOptions(include func(r *management.ResourceServer) bool) (pickerOptions, error) {
	list, err := c.api.ResourceServer.List()
	if err != nil {
		return nil, err
	}

	// NOTE: because client names are not unique, we'll just number these
	// labels.
	var opts pickerOptions
	for _, r := range list.ResourceServers {
		if !include(r) {
			continue
		}
		label := fmt.Sprintf("%s %s", r.GetName(), ansi.Faint("("+r.GetIdentifier()+")"))

		opts = append(opts, pickerOption{value: r.GetID(), label: label})
	}

	if len(opts) == 0 {
		return nil, errors.New("There are currently no applications.")
	}

	return opts, nil
}
