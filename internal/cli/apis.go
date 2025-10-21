package cli

import (
	"context"
	"encoding/json"
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

const apiDefaultTokenLifetime = 86400

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
	apiSigningAlgorithm = Flag{
		Name:     "Signing Algorithm",
		LongForm: "signing-alg",
		Help:     "Algorithm used to sign JWTs. Can be HS256 or RS256. PS256 available via addon.",
	}
	apiNumber = Flag{
		Name:      "Number",
		LongForm:  "number",
		ShortForm: "n",
		Help:      "Number of APIs to retrieve. Minimum 1, maximum 1000.",
	}
	apiSubjectTypeAuthorization = Flag{
		Name:     "Subject Type Authorization",
		LongForm: "subject-type-authorization",
		Help:     "JSON object defining access policies for user and client flows. Example: '{\"user\":{\"policy\":\"require_client_grant\"},\"client\":{\"policy\":\"deny_all\"}}'",
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
	cmd.AddCommand(createAPICmd(cli))
	cmd.AddCommand(showAPICmd(cli))
	cmd.AddCommand(updateAPICmd(cli))
	cmd.AddCommand(deleteAPICmd(cli))
	cmd.AddCommand(openAPICmd(cli))
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
  auth0 apis ls -n 100 --json
  auth0 apis ls -n 100 --json-compact
  auth0 apis ls --csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Number < 1 || inputs.Number > 1000 {
				return fmt.Errorf("number flag invalid, please pass a number between 1 and 1000")
			}

			list, err := getWithPagination(
				inputs.Number,
				func(opts ...management.RequestOption) (result []interface{}, hasNext bool, err error) {
					apiList, err := cli.api.ResourceServer.List(cmd.Context(), opts...)
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
				return fmt.Errorf("failed to list APIs: %w", err)
			}

			var apis []*management.ResourceServer
			for _, item := range list {
				apis = append(apis, item.(*management.ResourceServer))
			}

			cli.renderer.APIList(apis)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	apiNumber.RegisterInt(cmd, &inputs.Number, defaultPageSize)

	return cmd
}

func showAPICmd(cli *cli) *cobra.Command {
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
  auth0 apis show <api-id|api-audience> --json
  auth0 apis show <api-id|api-audience> --json-compact`,
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
				api, err = cli.api.ResourceServer.Read(cmd.Context(), url.PathEscape(inputs.ID))
				return err
			}); err != nil {
				return fmt.Errorf("failed to read API with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.APIShow(api, cli.json)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func createAPICmd(cli *cli) *cobra.Command {
	var inputs struct {
		Name                     string
		Identifier               string
		Scopes                   []string
		TokenLifetime            int
		AllowOfflineAccess       bool
		SigningAlgorithm         string
		SubjectTypeAuthorization string
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
  auth0 apis create --name myapi --identifier http://my-api --token-lifetime 6100 --offline-access=true
  auth0 apis create --name myapi --identifier http://my-api --token-lifetime 6100 --offline-access=false --scopes "letter:write,letter:read"
  auth0 apis create --name myapi --identifier http://my-api --token-lifetime 6100 --offline-access=false --scopes "letter:write,letter:read" --signing-alg "RS256"
  auth0 apis create -n myapi -i http://my-api -t 6100 -o false -s "letter:write,letter:read" --signing-alg "RS256" --json
  auth0 apis create -n myapi -i http://my-api -t 6100 -o false -s "letter:write,letter:read" --signing-alg "RS256" --json-compact
  auth0 apis create --name myapi --identifier http://my-api --subject-type-authorization '{"user":{"policy":"allow_all"},"client":{"policy":"deny_all"}}'`,
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

			defaultTokenLifetime := strconv.Itoa(apiDefaultTokenLifetime)
			if err := apiTokenLifetime.Ask(cmd, &inputs.TokenLifetime, &defaultTokenLifetime); err != nil {
				return err
			}

			if err := apiOfflineAccess.AskBool(cmd, &inputs.AllowOfflineAccess, nil); err != nil {
				return err
			}

			if err := apiSigningAlgorithm.Ask(cmd, &inputs.SigningAlgorithm, auth0.String("RS256")); err != nil {
				return err
			}

			if err := apiSubjectTypeAuthorization.Ask(cmd, &inputs.SubjectTypeAuthorization, nil); err != nil {
				return err
			}

			api := &management.ResourceServer{
				Name:               &inputs.Name,
				Identifier:         &inputs.Identifier,
				AllowOfflineAccess: &inputs.AllowOfflineAccess,
				TokenLifetime:      &inputs.TokenLifetime,
				SigningAlgorithm:   &inputs.SigningAlgorithm,
			}

			if len(inputs.Scopes) > 0 {
				api.Scopes = apiScopesFor(inputs.Scopes)
			}

			if inputs.TokenLifetime <= 0 {
				api.TokenLifetime = auth0.Int(apiDefaultTokenLifetime)
			} else {
				api.TokenLifetime = auth0.Int(inputs.TokenLifetime)
			}

			if inputs.SubjectTypeAuthorization != "{}" && inputs.SubjectTypeAuthorization != "" {
				var subjectTypeAuth management.ResourceServerSubjectTypeAuthorization
				if err := json.Unmarshal([]byte(inputs.SubjectTypeAuthorization), &subjectTypeAuth); err != nil {
					return fmt.Errorf("invalid JSON for subject-type-authorization: %w", err)
				}
				api.SubjectTypeAuthorization = &subjectTypeAuth
			}

			if err := ansi.Waiting(func() error {
				return cli.api.ResourceServer.Create(cmd.Context(), api)
			}); err != nil {
				return fmt.Errorf(
					"failed to create API with name %q and identifier %q: %w",
					inputs.Name,
					inputs.Identifier,
					err,
				)
			}

			cli.renderer.APICreate(api)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	apiName.RegisterString(cmd, &inputs.Name, "")
	apiIdentifier.RegisterString(cmd, &inputs.Identifier, "")
	apiScopes.RegisterStringSlice(cmd, &inputs.Scopes, nil)
	apiOfflineAccess.RegisterBool(cmd, &inputs.AllowOfflineAccess, false)
	apiTokenLifetime.RegisterInt(cmd, &inputs.TokenLifetime, 0)
	apiSigningAlgorithm.RegisterString(cmd, &inputs.SigningAlgorithm, "RS256")
	apiSubjectTypeAuthorization.RegisterString(cmd, &inputs.SubjectTypeAuthorization, "{}")

	return cmd
}

func updateAPICmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID                       string
		Name                     string
		Scopes                   []string
		TokenLifetime            int
		AllowOfflineAccess       bool
		SigningAlgorithm         string
		SubjectTypeAuthorization string
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
  auth0 apis update <api-id|api-audience> --name myapi --token-lifetime 6100 --offline-access=false
  auth0 apis update <api-id|api-audience> --name myapi --token-lifetime 6100 --offline-access=false --scopes "letter:write,letter:read" --signing-alg "RS256"
  auth0 apis update <api-id|api-audience> -n myapi -t 6100 -o false -s "letter:write,letter:read" --signing-alg "RS256" --json
  auth0 apis update <api-id|api-audience> -n myapi -t 6100 -o false -s "letter:write,letter:read" --signing-alg "RS256" --json-compact
  auth0 apis update <api-id|api-audience> --subject-type-authorization '{"user":{"policy":"require_client_grant"},"client":{"policy":"deny_all"}}'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := apiID.Pick(cmd, &inputs.ID, cli.apiPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var current *management.ResourceServer
			if err := ansi.Waiting(func() (err error) {
				current, err = cli.api.ResourceServer.Read(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to find API with ID %q: %w", inputs.ID, err)
			}

			if err := apiName.AskU(cmd, &inputs.Name, current.Name); err != nil {
				return err
			}

			if err := apiScopes.AskManyU(cmd, &inputs.Scopes, nil); err != nil {
				return err
			}

			currentTokenLifetime := strconv.Itoa(current.GetTokenLifetime())
			if err := apiTokenLifetime.AskIntU(cmd, &inputs.TokenLifetime, &currentTokenLifetime); err != nil {
				return err
			}

			if !apiOfflineAccess.IsSet(cmd) {
				inputs.AllowOfflineAccess = current.GetAllowOfflineAccess()
			}

			if err := apiOfflineAccess.AskBoolU(cmd, &inputs.AllowOfflineAccess, current.AllowOfflineAccess); err != nil {
				return err
			}

			if err := apiSigningAlgorithm.AskU(cmd, &inputs.SigningAlgorithm, current.SigningAlgorithm); err != nil {
				return err
			}

			// Current subject type authorization value for display.
			var currentSubjectTypeJSON string
			if current.SubjectTypeAuthorization != nil {
				if jsonBytes, err := json.Marshal(current.SubjectTypeAuthorization); err == nil {
					currentSubjectTypeJSON = string(jsonBytes)
				}
			}

			if err := apiSubjectTypeAuthorization.AskU(cmd, &inputs.SubjectTypeAuthorization, &currentSubjectTypeJSON); err != nil {
				return err
			}

			api := &management.ResourceServer{
				AllowOfflineAccess: &inputs.AllowOfflineAccess,
			}

			api.Name = current.Name
			if len(inputs.Name) != 0 {
				api.Name = &inputs.Name
			}

			api.Scopes = current.Scopes
			if len(inputs.Scopes) != 0 {
				api.Scopes = apiScopesFor(inputs.Scopes)
			}

			api.TokenLifetime = current.TokenLifetime
			if inputs.TokenLifetime != 0 {
				api.TokenLifetime = &inputs.TokenLifetime
			}

			api.SigningAlgorithm = current.SigningAlgorithm
			if inputs.SigningAlgorithm != "" {
				api.SigningAlgorithm = &inputs.SigningAlgorithm
			}

			api.SubjectTypeAuthorization = current.SubjectTypeAuthorization
			if inputs.SubjectTypeAuthorization != "{}" {
				var subjectTypeAuth management.ResourceServerSubjectTypeAuthorization
				if err := json.Unmarshal([]byte(inputs.SubjectTypeAuthorization), &subjectTypeAuth); err != nil {
					return fmt.Errorf("invalid JSON for subject-type-authorization: %w", err)
				}
				api.SubjectTypeAuthorization = &subjectTypeAuth
			}

			if err := ansi.Waiting(func() error {
				return cli.api.ResourceServer.Update(cmd.Context(), current.GetID(), api)
			}); err != nil {
				return fmt.Errorf("failed to update API with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.APIUpdate(api)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	apiName.RegisterStringU(cmd, &inputs.Name, "")
	apiScopes.RegisterStringSliceU(cmd, &inputs.Scopes, nil)
	apiOfflineAccess.RegisterBoolU(cmd, &inputs.AllowOfflineAccess, false)
	apiTokenLifetime.RegisterIntU(cmd, &inputs.TokenLifetime, 0)
	apiSigningAlgorithm.RegisterStringU(cmd, &inputs.SigningAlgorithm, "RS256")
	apiSubjectTypeAuthorization.RegisterStringU(cmd, &inputs.SubjectTypeAuthorization, "{}")

	return cmd
}

func deleteAPICmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Short:   "Delete an API",
		Long: "Delete an API.\n\n" +
			"To delete interactively, use `auth0 apis delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the API id and the `--force` flag to skip confirmation.",
		Example: `  auth0 apis delete 
  auth0 apis rm
  auth0 apis delete <api-id|api-audience>
  auth0 apis delete <api-id|api-audience> --force
  auth0 apis delete <api-id|api-audience> <api-id2> <api-idn>
  auth0 apis delete <api-id|api-audience> <api-id2> <api-idn> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var ids []string
			if len(args) == 0 {
				if err := apiID.PickMany(cmd, &ids, cli.apiPickerOptions); err != nil {
					return err
				}
			} else {
				ids = append(ids, args...)
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			return ansi.ProgressBar("Deleting API(s)", ids, func(_ int, id string) error {
				if _, err := cli.api.ResourceServer.Read(cmd.Context(), id); err != nil {
					return fmt.Errorf("failed to delete API with ID %q: %w", id, err)
				}

				if err := cli.api.ResourceServer.Delete(cmd.Context(), id); err != nil {
					return fmt.Errorf("failed to delete API with ID %q: %w", id, err)
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

func openAPICmd(cli *cli) *cobra.Command {
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
			// But it should cover the vast majority of users.
			if _, err := url.ParseRequestURI(inputs.ID); err == nil || len(inputs.ID) != 24 {
				if err := ansi.Waiting(func() error {
					api, err := cli.api.ResourceServer.Read(cmd.Context(), inputs.ID)
					if err != nil {
						return err
					}

					inputs.ID = api.GetID()

					return nil
				}); err != nil {
					return fmt.Errorf("failed to read API with ID %q: %w", inputs.ID, err)
				}
			}

			openManageURL(cli, cli.Config.DefaultTenant, formatAPISettingsPath(inputs.ID))

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
  auth0 apis scopes ls <api-id|api-audience> --json
  auth0 apis scopes ls <api-id|api-audience> --json-compact
  auth0 apis scopes ls <api-id|api-audience> --csv`,
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
				api, err = cli.api.ResourceServer.Read(cmd.Context(), url.PathEscape(inputs.ID))
				return err
			}); err != nil {
				return fmt.Errorf("failed to read scopes for API with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.ScopesList(api.GetName(), api.GetScopes())

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	return cmd
}

func formatAPISettingsPath(id string) string {
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

func (c *cli) apiPickerOptions(ctx context.Context) (pickerOptions, error) {
	return c.filteredAPIPickerOptions(ctx, func(r *management.ResourceServer) bool {
		return true
	})
}

func (c *cli) filteredAPIPickerOptions(ctx context.Context, include func(r *management.ResourceServer) bool) (pickerOptions, error) {
	list, err := c.api.ResourceServer.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list APIs: %w", err)
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
		return nil, errors.New("there are currently no APIs to choose from. Create one by running: `auth0 apis create`")
	}

	return opts, nil
}
