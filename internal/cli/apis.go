package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/auth0/go-auth0"
	"github.com/auth0/go-auth0/management"
	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/display"
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
	apiEnforcePolicies = Flag{
		Name:     "Enforce Policies",
		LongForm: "enforce-policies",
		Help:     "If true, authorization policies will be enforced for this API.",
	}
	apiTokenDialect = Flag{
		Name:     "Token Dialect",
		LongForm: "token-dialect",
		Help:     "Dialect of access tokens for this API. Can be one of access_token, access_token_authz, rfc9068_profile, or rfc9068_profile_authz.",
	}
	apiSearchFilter = Flag{
		Name:     "Filter",
		LongForm: "filter",
		Help: "Filter expression, sent as the API's 'q' parameter. Lucene syntax by default, or SCIM with --parser scim. " +
			"Supported fields: id, identifier, name, updated_at. Maximum 5 filter operations.\n\n" +
			"For example: 'name:\"My API\"' or, with --parser scim, 'name co \"billing\" and updated_at gt \"2026-01-01\"'. " +
			"Equivalent to --query '{\"q\":\"<expression>\"}'.",
	}
	apiSearchParser = Flag{
		Name:     "Parser",
		LongForm: "parser",
		Help:     "Syntax of --filter: 'lucene' or 'scim'.",
	}
	apiSearchSort = Flag{
		Name:      "Sort",
		LongForm:  "sort",
		ShortForm: "s",
		Help: "Field to sort by, ascending only: 'name', 'identifier' or 'updated_at'. " +
			"The 'field:1' form is also accepted. Defaults to insertion order.",
	}
	apiSearchFields = Flag{
		Name:     "Fields",
		LongForm: "fields",
		Help: "Comma-separated list of fields to include in the response, e.g. 'id,name,identifier'. " +
			"'updated_at' can be filtered and sorted on but is never returned.",
	}
	apiSearchExcludeFields = Flag{
		Name:     "Exclude Fields",
		LongForm: "exclude-fields",
		Help:     "Exclude the --fields list from the response instead of returning only those fields.",
	}
)

const (
	apiSearchPageSize  = 100  // Max `take` per request.
	apiSearchMaxLength = 1000 // Max length of the `q` and `fields` params.
)

var (
	apiTokenDialectNone    = "<NONE>"
	apiTokenDialectOptions = []string{
		apiTokenDialectNone,
		"access_token",
		"access_token_authz",
		"rfc9068_profile",
		"rfc9068_profile_authz",
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
	cmd.AddCommand(searchApisCmd(cli))
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
		Schema bool
		Query  string
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your APIs",
		Long: `List your existing APIs. To create one, run: ` + "`auth0 apis create`" + `.

Use '--schema' to see available query parameters.
Use '--query' to filter results via a JSON object (any API-supported parameter works immediately).`,
		Example: `  auth0 apis list
  auth0 apis ls
  auth0 apis ls --number 100
  auth0 apis ls -n 100 --json
  auth0 apis ls -n 100 --json-compact
  auth0 apis ls --csv
  auth0 apis list --schema
  auth0 apis list --schema --json
  auth0 apis list --query '{"identifiers":["https://my-api"]}'
  auth0 apis list --query '{"identifiers":["https://my-api"]}' --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Schema {
				return printOperationSchema(cli, "GET", "/resource-servers")
			}

			if inputs.Query != "" {
				return runJSONQuery(cli, cmd, jsonQuerySpec{
					Path:      "resource-servers",
					SchemaCmd: "auth0 apis list",
				}, inputs.Query)
			}

			if inputs.Number < 1 || inputs.Number > 1000 {
				return validationError{err: fmt.Errorf("number flag invalid, please pass a number between 1 and 1000")}
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
	schemaFlag.RegisterBool(cmd, &inputs.Schema, false)
	listQueryFlag.RegisterString(cmd, &inputs.Query, "")

	return cmd
}

// apiSearchInputs holds the typed-mode flags of `apis search`.
type apiSearchInputs struct {
	Filter        string
	Parser        string
	Sort          string
	Fields        string
	ExcludeFields bool
	Number        int
}

func searchApisCmd(cli *cli) *cobra.Command {
	var inputs struct {
		apiSearchInputs
		Query  string
		Schema bool
	}

	cmd := &cobra.Command{
		Use:   "search",
		Args:  cobra.NoArgs,
		Short: "Search your APIs (Early Access)",
		Long: `[Early Access] Search your APIs using Lucene or SCIM filter syntax.
Results are eventually consistent; use ` + "`auth0 apis list`" + ` for up-to-date data.

Use '--schema' to see available query parameters.
Use '--query' to filter results via a JSON object (any API-supported parameter works immediately).`,
		Example: `  auth0 apis search
  auth0 apis search --filter 'name:"My API"'
  auth0 apis search --parser scim --filter 'identifier sw "https://internal"' --sort identifier
  auth0 apis search --parser scim --filter 'name co "billing"' -s name:1
  auth0 apis search --parser scim --filter 'updated_at gt "2026-01-01"' -n 200 --csv
  auth0 apis search --filter 'name:"My API"' --fields id,name,identifier --json
  auth0 apis search --schema
  auth0 apis search --query '{"q":"name co \"billing\"","parser":"scim","sort":"name"}'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if handled, err := runSearchSchemaOrQuery(cli, cmd, jsonQuerySpec{
				Path:      "resource-servers/search",
				SchemaCmd: "auth0 apis search",
			}, "APIs", inputs.Schema, inputs.Query); handled {
				return err
			}

			if inputs.Number < 1 || inputs.Number > 1000 {
				return validationError{err: fmt.Errorf("number flag invalid, please pass a number between 1 and 1000")}
			}

			request, err := buildResourceServerSearchRequest(inputs.apiSearchInputs)
			if err != nil {
				return err
			}

			fields := parseSearchFields(inputs.Fields)
			if cli.csv && len(display.APISearchColumns(fields, inputs.ExcludeFields)) == 0 {
				return validationError{err: fmt.Errorf("--csv needs at least one table column (id, name, identifier, scopes) left by --fields")}
			}

			results, err := collectV3Pages(cmd.Context(), inputs.Number,
				func(ctx context.Context) (*core.Page[*string, *managementv3.ResourceServerSearchResponse, *managementv3.SearchResourceServersResponseContent], error) {
					return cli.apiv3.ResourceServerV3.Search(ctx, request)
				})
			if err != nil {
				return searchError("APIs", err)
			}

			if len(results) == inputs.Number {
				hint := "Refine --filter or raise --number (max 1000)."
				if inputs.Number == 1000 {
					hint = "Refine --filter."
				}

				cli.renderer.Warnf("Results may be limited by --number; more may match. %s", hint)
			}

			cli.renderer.APISearchList(results, fields, inputs.ExcludeFields)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	apiSearchFilter.RegisterString(cmd, &inputs.Filter, "")
	apiSearchParser.RegisterString(cmd, &inputs.Parser, "lucene")
	apiSearchSort.RegisterString(cmd, &inputs.Sort, "")
	apiSearchFields.RegisterString(cmd, &inputs.Fields, "")
	apiSearchExcludeFields.RegisterBool(cmd, &inputs.ExcludeFields, false)
	apiNumber.RegisterInt(cmd, &inputs.Number, defaultPageSize)
	schemaFlag.RegisterBool(cmd, &inputs.Schema, false)
	searchQueryFlag.RegisterString(cmd, &inputs.Query, "")
	markQueryExclusive(cmd)

	return cmd
}

// buildResourceServerSearchRequest validates the flag values and maps them onto the SDK request.
func buildResourceServerSearchRequest(inputs apiSearchInputs) (*managementv3.SearchResourceServersRequestParameters, error) {
	fields := strings.Join(parseSearchFields(inputs.Fields), ",")

	// The API caps both at 1000 characters; reject early with a clearer error than its 400.
	if utf8.RuneCountInString(inputs.Filter) > apiSearchMaxLength {
		return nil, validationError{err: fmt.Errorf("--filter must be at most %d characters", apiSearchMaxLength)}
	}
	if utf8.RuneCountInString(fields) > apiSearchMaxLength {
		return nil, validationError{err: fmt.Errorf("--fields must be at most %d characters", apiSearchMaxLength)}
	}
	if inputs.ExcludeFields && fields == "" {
		return nil, validationError{err: fmt.Errorf("--exclude-fields requires --fields")}
	}

	request := &managementv3.SearchResourceServersRequestParameters{
		Take: auth0.Int(min(inputs.Number, apiSearchPageSize)),
	}

	switch inputs.Parser {
	case "", "lucene":
		request.Parser = managementv3.SearchParserEnumLucene.Ptr()
	case "scim":
		request.Parser = managementv3.SearchParserEnumSCIM.Ptr()
	default:
		return nil, validationError{err: fmt.Errorf("invalid --parser %q: must be 'lucene' or 'scim'", inputs.Parser)}
	}

	if inputs.Filter != "" {
		request.Q = auth0.String(inputs.Filter)
	}

	if fields != "" {
		request.Fields = auth0.String(fields)
	}

	if inputs.ExcludeFields {
		request.IncludeFields = auth0.Bool(false)
	}

	if inputs.Sort != "" {
		field, err := resourceServerSortField(inputs.Sort)
		if err != nil {
			return nil, err
		}
		request.Sort = field.Ptr()
	}

	return request, nil
}

// parseSearchFields splits a --fields value into trimmed, non-empty field names.
func parseSearchFields(fields string) []string {
	var names []string
	for name := range strings.SplitSeq(fields, ",") {
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}

	return names
}

// resourceServerSortField maps 'field' or 'field:1' onto the SDK enum. The
// endpoint only sorts ascending, so 'field:-1' is rejected.
func resourceServerSortField(s string) (managementv3.ResourceServerSortFieldEnum, error) {
	field, order, hasOrder := strings.Cut(s, ":")
	if hasOrder && order != "1" {
		return "", validationError{err: fmt.Errorf("invalid --sort %q: the search endpoint only sorts ascending, use 'field' or 'field:1'", s)}
	}

	switch field {
	case "name":
		return managementv3.ResourceServerSortFieldEnumName, nil
	case "identifier":
		return managementv3.ResourceServerSortFieldEnumIdentifier, nil
	case "updated_at":
		return managementv3.ResourceServerSortFieldEnumUpdatedAt, nil
	default:
		return "", validationError{err: fmt.Errorf("invalid --sort %q: must be 'name', 'identifier' or 'updated_at'", s)}
	}
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
		EnforcePolicies          bool
		TokenDialect             string
		Data                     string
		Schema                   bool
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new API",
		Long: `Create a new API.

To create interactively, use ` + "`auth0 apis create`" + ` with no flags.

To create non-interactively, supply the name, identifier, scopes, token lifetime and whether to allow offline access through the flags.

Use '--schema' to print the request payload schema and exit.
Use '--data' to supply the full JSON payload (validated against the schema before sending).`,
		Example: `  auth0 apis create
  auth0 apis create --name myapi
  auth0 apis create --name myapi --identifier http://my-api
  auth0 apis create --name myapi --identifier http://my-api --token-lifetime 6100
  auth0 apis create --name myapi --identifier http://my-api --token-lifetime 6100 --offline-access=true
  auth0 apis create --name myapi --identifier http://my-api --token-lifetime 6100 --offline-access=false --scopes "letter:write,letter:read"
  auth0 apis create --name myapi --identifier http://my-api --token-lifetime 6100 --offline-access=false --scopes "letter:write,letter:read" --signing-alg "RS256"
  auth0 apis create -n myapi -i http://my-api -t 6100 -o false -s "letter:write,letter:read" --signing-alg "RS256" --json
  auth0 apis create -n myapi -i http://my-api -t 6100 -o false -s "letter:write,letter:read" --signing-alg "RS256" --json-compact
  auth0 apis create --name myapi --identifier http://my-api --subject-type-authorization '{"user":{"policy":"allow_all"},"client":{"policy":"deny_all"}}'
  auth0 apis create --name myapi --identifier http://my-api --enforce-policies --token-dialect access_token_authz

  # Discover the payload schema
  auth0 apis create --schema
  auth0 apis create --schema --json

  # JSON input mode (for agents and automation)
  auth0 apis create --data '{"name":"myapi","identifier":"https://my-api"}'
  auth0 apis create --data @api.json
  cat api.json | auth0 apis create`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Schema {
				return printOperationSchema(cli, "POST", "/resource-servers")
			}

			payload, provided, err := ResolveData(cmd)
			if err != nil {
				return err
			}
			if provided {
				return createAPIFromJSON(cli, cmd, payload)
			}

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

			if err := apiEnforcePolicies.AskBool(cmd, &inputs.EnforcePolicies, nil); err != nil {
				return err
			}

			api := &management.ResourceServer{
				Name:               &inputs.Name,
				Identifier:         &inputs.Identifier,
				AllowOfflineAccess: &inputs.AllowOfflineAccess,
				TokenLifetime:      &inputs.TokenLifetime,
				SigningAlgorithm:   &inputs.SigningAlgorithm,
				EnforcePolicies:    &inputs.EnforcePolicies,
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

			if err := apiTokenDialect.Select(cmd, &inputs.TokenDialect, apiTokenDialectOptions, nil); err != nil {
				return err
			}

			if inputs.TokenDialect != "" && inputs.TokenDialect != apiTokenDialectNone {
				api.TokenDialect = &inputs.TokenDialect
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
	apiEnforcePolicies.RegisterBool(cmd, &inputs.EnforcePolicies, false)
	apiTokenDialect.RegisterString(cmd, &inputs.TokenDialect, "")
	dataFlag.RegisterString(cmd, &inputs.Data, "")
	schemaFlag.RegisterBool(cmd, &inputs.Schema, false)
	markDataExclusive(cmd)

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
		EnforcePolicies          bool
		TokenDialect             string
		Data                     string
		Schema                   bool
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update an API",
		Long: `Update an API.

To update interactively, use ` + "`auth0 apis update`" + ` with no arguments.

To update non-interactively, supply the name, identifier, scopes, token lifetime and whether to allow offline access through the flags.

Use '--schema' to print the request payload schema and exit.
Use '--data' to supply the full JSON payload (validated against the schema before sending).`,
		Example: `  auth0 apis update
  auth0 apis update <api-id|api-audience>
  auth0 apis update <api-id|api-audience> --name myapi
  auth0 apis update <api-id|api-audience> --name myapi --token-lifetime 6100
  auth0 apis update <api-id|api-audience> --name myapi --token-lifetime 6100 --offline-access=false
  auth0 apis update <api-id|api-audience> --name myapi --token-lifetime 6100 --offline-access=false --scopes "letter:write,letter:read" --signing-alg "RS256"
  auth0 apis update <api-id|api-audience> -n myapi -t 6100 -o false -s "letter:write,letter:read" --signing-alg "RS256" --json
  auth0 apis update <api-id|api-audience> -n myapi -t 6100 -o false -s "letter:write,letter:read" --signing-alg "RS256" --json-compact
  auth0 apis update <api-id|api-audience> --subject-type-authorization '{"user":{"policy":"require_client_grant"},"client":{"policy":"deny_all"}}'
  auth0 apis update <api-id|api-audience> --enforce-policies=false --token-dialect rfc9068_profile_authz

  # Discover the payload schema
  auth0 apis update --schema
  auth0 apis update --schema --json

  # JSON input mode (for agents and automation)
  auth0 apis update <api-id|api-audience> --data '{"token_lifetime":7200}'
  auth0 apis update <api-id|api-audience> --data @api.json
  cat api.json | auth0 apis update <api-id|api-audience>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Schema {
				return printOperationSchema(cli, "PATCH", "/resource-servers/{id}")
			}

			if len(args) == 0 {
				if err := apiID.Pick(cmd, &inputs.ID, cli.apiPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			payload, provided, err := ResolveData(cmd)
			if err != nil {
				return err
			}
			if provided {
				return updateAPIFromJSON(cli, cmd, inputs.ID, payload)
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

			if !apiEnforcePolicies.IsSet(cmd) {
				inputs.EnforcePolicies = current.GetEnforcePolicies()
			}

			if err := apiEnforcePolicies.AskBoolU(cmd, &inputs.EnforcePolicies, current.EnforcePolicies); err != nil {
				return err
			}

			if err := apiSigningAlgorithm.AskU(cmd, &inputs.SigningAlgorithm, current.SigningAlgorithm); err != nil {
				return err
			}

			if err := apiTokenDialect.SelectU(cmd, &inputs.TokenDialect, apiTokenDialectOptions, current.TokenDialect); err != nil {
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
				EnforcePolicies:    &inputs.EnforcePolicies,
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

			if inputs.TokenDialect != "" && inputs.TokenDialect != apiTokenDialectNone {
				api.TokenDialect = &inputs.TokenDialect
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
	apiEnforcePolicies.RegisterBoolU(cmd, &inputs.EnforcePolicies, false)
	apiTokenDialect.RegisterStringU(cmd, &inputs.TokenDialect, "")
	dataFlag.RegisterString(cmd, &inputs.Data, "")
	schemaFlag.RegisterBool(cmd, &inputs.Schema, false)
	markDataExclusive(cmd)

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

			if !cli.force && cli.agentMode {
				return errDestructiveNoConfirm
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

			return openManageURL(cli, cli.tenant, formatAPISettingsPath(inputs.ID))
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

func createAPIFromJSON(cli *cli, cmd *cobra.Command, dataStr string) error {
	api, err := runJSONWrite[management.ResourceServer](cli, cmd, jsonWriteSpec{
		Method:     http.MethodPost,
		SchemaPath: "/resource-servers",
		URI:        cli.api.HTTPClient.URI("resource-servers"),
		Data:       dataStr,
		SchemaCmd:  "auth0 apis create",
	})
	if err != nil {
		return fmt.Errorf("failed to create API: %w", err)
	}
	cli.renderer.APICreate(api)
	return nil
}

func updateAPIFromJSON(cli *cli, cmd *cobra.Command, id, dataStr string) error {
	api, err := runJSONWrite[management.ResourceServer](cli, cmd, jsonWriteSpec{
		Method:     http.MethodPatch,
		SchemaPath: "/resource-servers/{id}",
		URI:        cli.api.HTTPClient.URI("resource-servers", id),
		Data:       dataStr,
		SchemaCmd:  "auth0 apis update",
	})
	if err != nil {
		return fmt.Errorf("failed to update API with ID %q: %w", id, err)
	}
	cli.renderer.APIUpdate(api)
	return nil
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
