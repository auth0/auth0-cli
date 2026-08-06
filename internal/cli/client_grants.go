package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/auth0/go-auth0/management"
	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/prompt"
)

var clientGrantOrganizationUsageOptions = []string{"deny", "allow", "require"}

var (
	clientGrantID = Argument{
		Name: "Id",
		Help: "Id of the client grant.",
	}
	clientGrantClientID = Flag{
		Name:      "Client ID",
		LongForm:  "client-id",
		ShortForm: "c",
		Help:      "Client ID of the application to authorize. Cannot be changed once set. Mutually exclusive with --default-for.",
	}
	clientGrantDefaultFor = Flag{
		Name:     "Default For",
		LongForm: "default-for",
		Help:     "Make this the default grant for a group of clients instead of authorizing a specific client. Mutually exclusive with --client-id. Possible value: third_party_clients.",
	}
	clientGrantAuthorizationDetailsTypes = Flag{
		Name:         "Authorization Details Types",
		LongForm:     "authorization-details-types",
		Help:         "Comma-separated list of authorization_details types allowed for this grant (Rich Authorization Requests).",
		AlwaysPrompt: true,
	}
	clientGrantAudience = Flag{
		Name:       "Audience",
		LongForm:   "audience",
		ShortForm:  "a",
		Help:       "Audience (API identifier) of the client grant. Cannot be changed once set.",
		IsRequired: true,
	}
	clientGrantScopes = Flag{
		Name:         "Scopes",
		LongForm:     "scopes",
		ShortForm:    "s",
		Help:         "Comma-separated list of scopes (permissions) to grant.",
		AlwaysPrompt: true,
	}
	clientGrantAllowAllScopes = Flag{
		Name:     "Allow All Scopes",
		LongForm: "allow-all-scopes",
		Help:     "Grant every scope configured on the API. Mutually exclusive with --scopes.",
	}
	clientGrantNoScopes = Flag{
		Name:     "No Scopes",
		LongForm: "no-scopes",
		Help:     "Clear all scopes on the grant, authorizing a token with no permissions. Mutually exclusive with --scopes and --allow-all-scopes.",
	}
	clientGrantOrganizationUsage = Flag{
		Name:         "Organization Usage",
		LongForm:     "organization-usage",
		ShortForm:    "o",
		Help:         "Whether organizations can be used with this grant. Possible values: " + strings.Join(clientGrantOrganizationUsageOptions, ", ") + ".",
		AlwaysPrompt: true,
	}
	clientGrantAllowAnyOrganization = Flag{
		Name:         "Allow Any Organization",
		LongForm:     "allow-any-organization",
		Help:         "Whether any organization can be used with this grant (true) or only explicitly assigned organizations (false).",
		AlwaysPrompt: true,
	}
	clientGrantSubjectType = Flag{
		Name:     "Subject Type",
		LongForm: "subject-type",
		Help:     "Subject type of the grant. Cannot be changed once set. Possible values: " + strings.Join(clientGrantSubjectTypeOptions, ", ") + ".",
	}
	clientGrantNumber = Flag{
		Name:      "Number",
		LongForm:  "number",
		ShortForm: "n",
		Help:      "Number of client grants to retrieve. Minimum 1, maximum 1000.",
	}

	clientGrantFilterClientID = Flag{
		Name:      "Client ID",
		LongForm:  "client-id",
		ShortForm: "c",
		Help:      "Filter by client ID. Mutually exclusive with --default-for.",
	}
	clientGrantFilterAudience = Flag{
		Name:      "Audience",
		LongForm:  "audience",
		ShortForm: "a",
		Help:      "Filter by audience (API identifier).",
	}
	clientGrantFilterSubjectType = Flag{
		Name:     "Subject Type",
		LongForm: "subject-type",
		Help:     "Filter by subject type. Possible values: " + strings.Join(clientGrantSubjectTypeOptions, ", ") + ".",
	}
	clientGrantFilterDefaultFor = Flag{
		Name:     "Default For",
		LongForm: "default-for",
		Help:     "Filter by the group this grant is the default for. Possible value: third_party_clients. Mutually exclusive with --client-id.",
	}
	clientGrantFilterAllowAnyOrganization = Flag{
		Name:     "Allow Any Organization",
		LongForm: "allow-any-organization",
		Help:     "Filter by whether any organization can be used with the grant (true) or only explicitly assigned organizations (false).",
	}
)

var clientGrantSubjectTypeOptions = []string{"client", "user", "anonymous_user"}

var clientGrantDefaultForOptions = []string{"third_party_clients"}

// managementAPIUserScopesNote explains that, for a user subject type against the
// Auth0 Management API, the scopes are a fixed current_user set that the API
// does not expose for dynamic discovery, so they have to be passed inline with
// --scopes rather than picked interactively.
const managementAPIUserScopesNote = "Note: for the Auth0 Management API with `--subject-type user`, scopes must be a " +
	"subset of the fixed current_user set and cannot be listed dynamically, so pass them inline, " +
	"for example: `--scopes \"read:current_user,update:current_user_metadata,delete:current_user_metadata," +
	"create:current_user_metadata,create:current_user_device_credentials,delete:current_user_device_credentials," +
	"update:current_user_identities\"`."

func clientGrantsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "client-grants",
		Short:   "Manage client grants",
		Long:    "Manage client grants. A client grant authorizes an application (client) to request access tokens for an API (audience), optionally scoped to specific permissions or organizations.",
		Aliases: []string{"grants"},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listClientGrantsCmd(cli))
	cmd.AddCommand(createClientGrantCmd(cli))
	cmd.AddCommand(showClientGrantCmd(cli))
	cmd.AddCommand(updateClientGrantCmd(cli))
	cmd.AddCommand(deleteClientGrantCmd(cli))
	cmd.AddCommand(organizationsClientGrantCmd(cli))

	return cmd
}

func listClientGrantsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Number               int
		ClientID             string
		Audience             string
		SubjectType          string
		DefaultFor           string
		AllowAnyOrganization bool
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your client grants",
		Long: "List your existing client grants. To create one, run: `auth0 client-grants create`.\n\n" +
			"Use the filter flags to narrow the results server-side by client, audience, subject type, " +
			"default group or organization usage.",
		Example: `  auth0 client-grants list
  auth0 client-grants ls
  auth0 client-grants ls --number 100
  auth0 client-grants ls --audience <api-identifier>
  auth0 client-grants ls --client-id <client-id> --subject-type client
  auth0 client-grants ls --default-for third_party_clients
  auth0 client-grants ls --allow-any-organization=true
  auth0 client-grants ls -n 100 --json
  auth0 client-grants ls --csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Number < 1 || inputs.Number > 1000 {
				return fmt.Errorf("number flag invalid, please pass a number between 1 and 1000")
			}

			// The API requires a client_id, audience or default_for filter
			// alongside subject_type; catch it early with a clearer message.
			if inputs.SubjectType != "" && inputs.ClientID == "" && inputs.Audience == "" && inputs.DefaultFor == "" {
				return fmt.Errorf("--subject-type must be combined with --client-id, --audience or --default-for")
			}

			request := &managementv3.ListClientGrantsRequestParameters{}
			if inputs.ClientID != "" {
				request.ClientID = &inputs.ClientID
			}
			if inputs.Audience != "" {
				request.Audience = &inputs.Audience
			}
			if inputs.SubjectType != "" {
				subjectType, err := managementv3.NewClientGrantSubjectTypeEnumFromString(inputs.SubjectType)
				if err != nil {
					return err
				}
				request.SubjectType = &subjectType
			}
			if inputs.DefaultFor != "" {
				defaultFor, err := managementv3.NewClientGrantDefaultForEnumFromString(inputs.DefaultFor)
				if err != nil {
					return err
				}
				request.DefaultFor = &defaultFor
			}
			if clientGrantFilterAllowAnyOrganization.IsSet(cmd) {
				request.AllowAnyOrganization = &inputs.AllowAnyOrganization
			}

			var grants []*managementv3.ClientGrantResponseContent
			if err := ansi.Waiting(func() error {
				page, err := cli.apiv3.ClientGrant.List(cmd.Context(), request)
				if err != nil {
					return err
				}

				iter := page.Iterator()
				for iter.Next(cmd.Context()) {
					grants = append(grants, iter.Current())
					if len(grants) >= inputs.Number {
						break
					}
				}
				return iter.Err()
			}); err != nil {
				return fmt.Errorf("failed to list client grants: %w", err)
			}

			cli.renderer.ClientGrantList(grants)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	clientGrantNumber.RegisterInt(cmd, &inputs.Number, defaultPageSize)
	clientGrantFilterClientID.RegisterString(cmd, &inputs.ClientID, "")
	clientGrantFilterAudience.RegisterString(cmd, &inputs.Audience, "")
	clientGrantFilterSubjectType.RegisterString(cmd, &inputs.SubjectType, "")
	clientGrantFilterDefaultFor.RegisterString(cmd, &inputs.DefaultFor, "")
	clientGrantFilterAllowAnyOrganization.RegisterBool(cmd, &inputs.AllowAnyOrganization, false)

	// The API rejects client_id and default_for together.
	cmd.MarkFlagsMutuallyExclusive("client-id", "default-for")

	return cmd
}

func showClientGrantCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show a client grant",
		Long:  "Display the client, audience, scopes, and other information about a client grant.",
		Example: `  auth0 client-grants show
  auth0 client-grants show <client-grant-id>
  auth0 client-grants show <client-grant-id> --json
  auth0 client-grants show <client-grant-id> --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := clientGrantID.Pick(cmd, &inputs.ID, cli.clientGrantPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var grant *managementv3.GetClientGrantResponseContent
			if err := ansi.Waiting(func() (err error) {
				grant, err = cli.apiv3.ClientGrant.Get(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to read client grant with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.ClientGrantShow(grant)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	return cmd
}

// Target-selection modes offered before the client-id or default-for prompt.
const (
	clientGrantTargetClient  = "A specific client"
	clientGrantTargetDefault = "Default for a group of clients"
)

func createClientGrantCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ClientID                  string
		DefaultFor                string
		Audience                  string
		Scopes                    []string
		AllowAllScopes            bool
		OrganizationUsage         string
		AllowAnyOrganization      bool
		SubjectType               string
		AuthorizationDetailsTypes []string
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new client grant",
		Long: "Create a new client grant.\n\n" +
			"To create interactively, use `auth0 client-grants create` with no flags.\n\n" +
			"To create non-interactively, supply the audience and either a client id (`--client-id`) " +
			"or a default group (`--default-for`), which are mutually exclusive, along with any optional " +
			"scopes or organization settings through the flags. A grant can authorize specific " +
			"scopes (`--scopes`), every scope on the API (`--allow-all-scopes`), or no scopes at all.\n\n" +
			managementAPIUserScopesNote,
		Example: `  auth0 client-grants create
  auth0 client-grants create --client-id <client-id> --audience <api-identifier>
  auth0 client-grants create --default-for third_party_clients --audience <api-identifier>
  auth0 client-grants create --client-id <client-id> --audience <api-identifier> --scopes "read:users,update:users"
  auth0 client-grants create --client-id <client-id> --audience <api-identifier> --allow-all-scopes
  auth0 client-grants create --client-id <client-id> --audience <api-identifier> --authorization-details-types "payment,transfer"
  auth0 client-grants create -c <client-id> -a <api-identifier> -s "read:users" -o require --allow-any-organization=false
  auth0 client-grants create -c <client-id> -a <api-identifier> --subject-type user
  auth0 client-grants create -c <client-id> -a <api-identifier> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// A grant authorizes either a specific client or a default group,
			// never both. When neither flag was passed and we can prompt, ask
			// which the grant should target, then prompt for that target. When a
			// flag was passed we skip the prompts and honor it directly.
			if !clientGrantClientID.IsSet(cmd) && !clientGrantDefaultFor.IsSet(cmd) && canPrompt(cmd) {
				if err := cli.pickClientGrantTarget(cmd, &inputs.ClientID, &inputs.DefaultFor); err != nil {
					return err
				}
			}

			if inputs.ClientID == "" && inputs.DefaultFor == "" {
				return errors.New("one of --client-id or --default-for must be set")
			}

			// A default grant is a template for a group of clients rather than an
			// authorization for a specific client, so subject type and organization
			// settings do not apply to it. The API rejects them, so skip those
			// prompts and never send those fields for a default grant.
			isDefaultGrant := inputs.DefaultFor != ""

			// Auth0 rejects a default grant against a system API, so hide system
			// APIs from the audience picker for a default grant.
			audiencePicker := cli.apiIdentifierPickerOptions
			if isDefaultGrant {
				audiencePicker = cli.nonSystemAPIIdentifierPickerOptions
			}
			if err := clientGrantAudience.Pick(cmd, &inputs.Audience, audiencePicker); err != nil {
				return err
			}

			// Reject subject type or organization flags passed for a default grant
			// (matching the API) rather than silently dropping them.
			if isDefaultGrant {
				for _, f := range []*Flag{&clientGrantSubjectType, &clientGrantOrganizationUsage, &clientGrantAllowAnyOrganization} {
					if f.IsSet(cmd) {
						return fmt.Errorf("--%s cannot be set with --default-for", f.LongForm)
					}
				}
			}

			if !isDefaultGrant {
				defaultSubjectType := clientGrantSubjectTypeOptions[0]
				if err := clientGrantSubjectType.Select(cmd, &inputs.SubjectType, clientGrantSubjectTypeOptions, &defaultSubjectType); err != nil {
					return err
				}
			}

			// The scope and authorization_details pickers both read the audience
			// API, so read it once here and share it between them rather than
			// hitting the API twice. The same read tells us whether the audience
			// is a system API, which cannot carry organization settings.
			askScopes := !clientGrantAllowAllScopes.IsSet(cmd) && shouldAsk(cmd, &clientGrantScopes, false)
			askAuthDetailsTypes := shouldAsk(cmd, &clientGrantAuthorizationDetailsTypes, false)
			// Scopes passed by flag skip the picker, so validate them against the
			// audience API here (reading it if the pickers above did not already).
			validateScopes := clientGrantScopes.IsSet(cmd)
			var audienceIsSystemAPI bool
			if askScopes || askAuthDetailsTypes || validateScopes {
				audienceAPI, err := cli.readClientGrantAudienceAPI(cmd.Context(), inputs.Audience)
				if err != nil {
					return err
				}
				audienceIsSystemAPI = audienceAPI.GetIsSystem()

				// When neither scope flag was passed, ask how to grant scopes
				// (all of them, a specific set, or none) and, for a specific set,
				// show a multi-select scoped to the chosen audience so the user
				// only picks from scopes that API actually defines.
				if askScopes {
					if err := cli.pickClientGrantScopes(audienceAPI, &inputs.Scopes, &inputs.AllowAllScopes, nil, nil, false, true); err != nil {
						return err
					}
				} else if validateScopes {
					if err := validateClientGrantScopes(audienceAPI, inputs.Scopes, nil, inputs.SubjectType); err != nil {
						return err
					}
				}

				// The authorization_details types are defined on the audience API,
				// so offer a multi-select of them (skipping silently when the API
				// has none) rather than making the user recall the exact strings.
				if askAuthDetailsTypes {
					if err := cli.pickClientGrantAuthorizationDetailsTypes(audienceAPI, &inputs.AuthorizationDetailsTypes, nil); err != nil {
						return err
					}
				}
			}

			// Organizations cannot be used with a default grant, with the user or
			// anonymous_user subject types, or against a system API (which rejects
			// any organization settings), so skip the organization prompts for them.
			if !isDefaultGrant && !audienceIsSystemAPI && clientGrantSubjectTypeAllowsOrganizations(inputs.SubjectType) {
				if err := clientGrantOrganizationUsage.Select(cmd, &inputs.OrganizationUsage, clientGrantOrganizationUsageOptions, nil); err != nil {
					return err
				}

				// Allowing any organization only applies when organizations can be
				// used with the grant, so only ask for it when organization usage
				// is allow or require. On deny (the default) it must stay false.
				if clientGrantOrganizationAllowsAny(inputs.OrganizationUsage) {
					if err := clientGrantAllowAnyOrganization.AskBool(cmd, &inputs.AllowAnyOrganization, nil); err != nil {
						return err
					}
				}
			}

			if err := validateClientGrantSubjectType(inputs.SubjectType, inputs.OrganizationUsage, inputs.AllowAnyOrganization); err != nil {
				return err
			}

			if err := validateClientGrantOrganization(inputs.OrganizationUsage, inputs.AllowAnyOrganization); err != nil {
				return err
			}

			grant := &managementv3.CreateClientGrantRequestContent{
				Audience: inputs.Audience,
			}

			// A grant targets either a specific client or a default group, so
			// only send the one that was provided.
			if inputs.ClientID != "" {
				grant.ClientID = &inputs.ClientID
			}
			if inputs.DefaultFor != "" {
				defaultFor, err := managementv3.NewClientGrantDefaultForEnumFromString(inputs.DefaultFor)
				if err != nil {
					return err
				}
				grant.DefaultFor = &defaultFor
			}

			if len(inputs.AuthorizationDetailsTypes) > 0 {
				grant.AuthorizationDetailsTypes = inputs.AuthorizationDetailsTypes
			}

			if inputs.AllowAllScopes {
				grant.AllowAllScopes = auth0.Bool(true)
			} else {
				// Send the scope explicitly, even when empty, so a grant with no
				// scopes serializes as "scope": [] rather than being omitted or
				// sent as null, both of which the API rejects. A nil slice
				// marshals to null, so normalize it to a non-nil empty slice.
				scopes := inputs.Scopes
				if scopes == nil {
					scopes = []string{}
				}
				grant.SetScope(scopes)
			}

			if inputs.SubjectType != "" {
				subjectType, err := managementv3.NewClientGrantSubjectTypeEnumFromString(inputs.SubjectType)
				if err != nil {
					return err
				}
				grant.SubjectType = &subjectType
			}

			// Organization settings cannot be sent for a default grant, for the
			// user or anonymous_user subject types, or against a system API, so
			// only attach them when the grant targets a specific client with a
			// subject type that allows it.
			if !isDefaultGrant && !audienceIsSystemAPI && clientGrantSubjectTypeAllowsOrganizations(inputs.SubjectType) {
				if inputs.OrganizationUsage != "" {
					organizationUsage, err := managementv3.NewClientGrantOrganizationUsageEnumFromString(inputs.OrganizationUsage)
					if err != nil {
						return err
					}
					grant.OrganizationUsage = &organizationUsage
				}

				// Send allow_any_organization only when the user engaged with
				// organization settings, either by passing the flag or by choosing
				// an organization usage (which is what the interactive prompt sets,
				// so the answer is not dropped). A grant that never touches
				// organizations must not carry a stray false, which the API rejects
				// for reserved-identifier audiences.
				if inputs.OrganizationUsage != "" || clientGrantAllowAnyOrganization.IsSet(cmd) {
					grant.AllowAnyOrganization = &inputs.AllowAnyOrganization
				}
			}

			// Describe the grant by whichever target it authorizes, so the error
			// reads sensibly for both a specific client and a default group.
			target := fmt.Sprintf("client %q", inputs.ClientID)
			if inputs.ClientID == "" {
				target = fmt.Sprintf("default group %q", inputs.DefaultFor)
			}

			var created *managementv3.CreateClientGrantResponseContent
			if err := ansi.Waiting(func() (err error) {
				created, err = cli.apiv3.ClientGrant.Create(cmd.Context(), grant)
				return err
			}); err != nil {
				return fmt.Errorf(
					"failed to create client grant for %s and audience %q: %w",
					target,
					inputs.Audience,
					err,
				)
			}

			cli.renderer.ClientGrantCreate(created)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")
	clientGrantClientID.RegisterString(cmd, &inputs.ClientID, "")
	clientGrantDefaultFor.RegisterString(cmd, &inputs.DefaultFor, "")
	clientGrantAudience.RegisterString(cmd, &inputs.Audience, "")
	clientGrantScopes.RegisterStringSlice(cmd, &inputs.Scopes, nil)
	clientGrantAllowAllScopes.RegisterBool(cmd, &inputs.AllowAllScopes, false)
	clientGrantOrganizationUsage.RegisterString(cmd, &inputs.OrganizationUsage, "")
	clientGrantAllowAnyOrganization.RegisterBool(cmd, &inputs.AllowAnyOrganization, false)
	clientGrantSubjectType.RegisterString(cmd, &inputs.SubjectType, "")
	clientGrantAuthorizationDetailsTypes.RegisterStringSlice(cmd, &inputs.AuthorizationDetailsTypes, nil)

	// A grant authorizes either specific scopes or all of them, never both.
	cmd.MarkFlagsMutuallyExclusive("scopes", "allow-all-scopes")

	// A grant targets either a specific client or a default group, never both.
	cmd.MarkFlagsMutuallyExclusive("client-id", "default-for")

	return cmd
}

func updateClientGrantCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID                        string
		Scopes                    []string
		AllowAllScopes            bool
		NoScopes                  bool
		OrganizationUsage         string
		AllowAnyOrganization      bool
		AuthorizationDetailsTypes []string
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update a client grant",
		Long: "Update a client grant.\n\n" +
			"To update interactively, use `auth0 client-grants update` with no arguments.\n\n" +
			"The client id and audience of a grant cannot be changed. To update non-interactively, " +
			"supply the scopes or organization settings through the flags. Pass `--allow-all-scopes` " +
			"to grant every scope on the API instead of a specific list, or `--no-scopes` to clear all " +
			"scopes and authorize a token with no permissions.\n\n" +
			managementAPIUserScopesNote,
		Example: `  auth0 client-grants update
  auth0 client-grants update <client-grant-id>
  auth0 client-grants update <client-grant-id> --scopes "read:users,update:users"
  auth0 client-grants update <client-grant-id> --allow-all-scopes
  auth0 client-grants update <client-grant-id> --no-scopes
  auth0 client-grants update <client-grant-id> --authorization-details-types "payment,transfer"
  auth0 client-grants update <client-grant-id> -s "read:users" -o require --allow-any-organization=false
  auth0 client-grants update <client-grant-id> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := clientGrantID.Pick(cmd, &inputs.ID, cli.mutableClientGrantPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var current *managementv3.GetClientGrantResponseContent
			if err := ansi.Waiting(func() (err error) {
				current, err = cli.apiv3.ClientGrant.Get(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to find client grant with ID %q: %w", inputs.ID, err)
			}

			// Auth0 rejects updating a system grant, so fail before running the
			// interactive flow rather than after the user has clicked through it.
			if current.GetIsSystem() {
				return fmt.Errorf("client grant with ID %q is a system grant and cannot be updated", inputs.ID)
			}

			// Audience is immutable, so the scope and authorization_details
			// pickers both read the grant's existing audience API. Read it once
			// here and share it between them rather than hitting the API twice.
			// The same read tells us whether the audience is a system API, which
			// cannot carry organization settings.
			askScopes := !clientGrantAllowAllScopes.IsSet(cmd) && shouldAsk(cmd, &clientGrantScopes, true)
			askAuthDetailsTypes := shouldAsk(cmd, &clientGrantAuthorizationDetailsTypes, true)
			// Scopes passed by flag skip the picker, so validate them against the
			// audience API here (reading it if the pickers above did not already).
			validateScopes := clientGrantScopes.IsSet(cmd)
			subjectType := string(current.GetSubjectType())
			var audienceIsSystemAPI bool
			if askScopes || askAuthDetailsTypes || validateScopes {
				audienceAPI, err := cli.readClientGrantAudienceAPI(cmd.Context(), current.GetAudience())
				if err != nil {
					return err
				}
				audienceIsSystemAPI = audienceAPI.GetIsSystem()

				// Default the scopes mode and selection to the grant's current
				// state, keeping the flow in sync with create.
				if askScopes {
					if err := cli.pickClientGrantScopes(audienceAPI, &inputs.Scopes, &inputs.AllowAllScopes, &inputs.NoScopes, current.GetScope(), current.GetAllowAllScopes(), true); err != nil {
						return err
					}
				} else if validateScopes {
					if err := validateClientGrantScopes(audienceAPI, inputs.Scopes, current.GetScope(), subjectType); err != nil {
						return err
					}
				}

				// Offer the authorization_details types defined on the API
				// (skipping silently when it has none), pre-selecting the grant's
				// current types.
				if askAuthDetailsTypes {
					if err := cli.pickClientGrantAuthorizationDetailsTypes(audienceAPI, &inputs.AuthorizationDetailsTypes, current.GetAuthorizationDetailsTypes()); err != nil {
						return err
					}
				}
			}

			// Organizations cannot be used with the user or anonymous_user subject
			// types, or against a system API (which rejects any organization
			// settings), so skip the organization prompts entirely for them. The
			// subject type is immutable, so it comes from the existing grant.
			if !audienceIsSystemAPI && clientGrantSubjectTypeAllowsOrganizations(subjectType) {
				if err := clientGrantOrganizationUsage.SelectU(cmd, &inputs.OrganizationUsage, clientGrantOrganizationUsageOptions, stringPtr(current.OrganizationUsage)); err != nil {
					return err
				}

				if !clientGrantAllowAnyOrganization.IsSet(cmd) {
					inputs.AllowAnyOrganization = current.GetAllowAnyOrganization()
				}

				// The effective organization usage is the new value when supplied,
				// otherwise whatever the grant already has (which we leave untouched).
				effectiveOrganizationUsage := inputs.OrganizationUsage
				if effectiveOrganizationUsage == "" {
					effectiveOrganizationUsage = string(current.GetOrganizationUsage())
				}

				// Allowing any organization only applies when organizations can be
				// used with the grant, so only ask for it when organization usage
				// is allow or require. On deny it must stay false.
				if clientGrantOrganizationAllowsAny(effectiveOrganizationUsage) {
					if err := clientGrantAllowAnyOrganization.AskBoolU(cmd, &inputs.AllowAnyOrganization, current.AllowAnyOrganization); err != nil {
						return err
					}
				}

				if err := validateClientGrantOrganization(effectiveOrganizationUsage, inputs.AllowAnyOrganization); err != nil {
					return err
				}
			}

			// Catch organization flags passed for a subject type that cannot use
			// organizations (matching create), turning the API 400 into a clear
			// message on the non-interactive path.
			if err := validateClientGrantSubjectType(subjectType, inputs.OrganizationUsage, inputs.AllowAnyOrganization); err != nil {
				return err
			}

			grant := &managementv3.UpdateClientGrantRequestContent{}

			if inputs.NoScopes {
				// The user explicitly cleared the scopes. Send them with SetScope
				// so an empty list serializes as "scope": [] rather than being
				// omitted (which would leave the existing scopes untouched). A nil
				// slice marshals to null, so normalize it to a non-nil empty slice.
				grant.SetScope([]string{})
				if current.GetAllowAllScopes() {
					grant.AllowAllScopes = auth0.Bool(false)
				}
			} else {
				grant.Scope, grant.AllowAllScopes = resolveUpdateClientGrantScopes(
					inputs.Scopes,
					inputs.AllowAllScopes,
					current.GetScope(),
					current.GetAllowAllScopes(),
				)
			}

			// Organization settings cannot be sent for the user or anonymous_user
			// subject types, or against a system API, so only attach them when the
			// subject type allows it and the audience is not a system API.
			if !audienceIsSystemAPI && clientGrantSubjectTypeAllowsOrganizations(subjectType) {
				grant.AllowAnyOrganization = &inputs.AllowAnyOrganization

				if inputs.OrganizationUsage != "" {
					organizationUsage, err := managementv3.NewClientGrantOrganizationNullableUsageEnumFromString(inputs.OrganizationUsage)
					if err != nil {
						return err
					}
					grant.OrganizationUsage = &organizationUsage
				}
			}

			if len(inputs.AuthorizationDetailsTypes) > 0 {
				grant.AuthorizationDetailsTypes = inputs.AuthorizationDetailsTypes
			}

			var updated *managementv3.UpdateClientGrantResponseContent
			if err := ansi.Waiting(func() (err error) {
				updated, err = cli.apiv3.ClientGrant.Update(cmd.Context(), inputs.ID, grant)
				return err
			}); err != nil {
				return fmt.Errorf("failed to update client grant with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.ClientGrantUpdate(updated)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")
	clientGrantScopes.RegisterStringSliceU(cmd, &inputs.Scopes, nil)
	clientGrantAllowAllScopes.RegisterBoolU(cmd, &inputs.AllowAllScopes, false)
	clientGrantNoScopes.RegisterBoolU(cmd, &inputs.NoScopes, false)
	clientGrantOrganizationUsage.RegisterStringU(cmd, &inputs.OrganizationUsage, "")
	clientGrantAllowAnyOrganization.RegisterBoolU(cmd, &inputs.AllowAnyOrganization, false)
	clientGrantAuthorizationDetailsTypes.RegisterStringSliceU(cmd, &inputs.AuthorizationDetailsTypes, nil)

	// A grant authorizes specific scopes, all of them, or none, never a mix.
	cmd.MarkFlagsMutuallyExclusive("scopes", "allow-all-scopes", "no-scopes")

	return cmd
}

func deleteClientGrantCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Short:   "Delete a client grant",
		Long: "Delete a client grant.\n\n" +
			"To delete interactively, use `auth0 client-grants delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the client grant id and the `--force` flag to skip confirmation.",
		Example: `  auth0 client-grants delete
  auth0 client-grants rm
  auth0 client-grants delete <client-grant-id>
  auth0 client-grants delete <client-grant-id> --force
  auth0 client-grants delete <client-grant-id> <client-grant-id2> <client-grant-idn>
  auth0 client-grants delete <client-grant-id> <client-grant-id2> <client-grant-idn> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var ids []string
			if len(args) == 0 {
				if err := clientGrantID.PickMany(cmd, &ids, cli.mutableClientGrantPickerOptions); err != nil {
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

			return ansi.ProgressBar("Deleting client grant(s)", ids, func(_ int, id string) error {
				current, err := cli.apiv3.ClientGrant.Get(cmd.Context(), id)
				if err != nil {
					return fmt.Errorf("failed to delete client grant with ID %q: %w", id, err)
				}

				// Auth0 rejects deleting a system grant, so surface a clear
				// message rather than the raw API error.
				if current.GetIsSystem() {
					return fmt.Errorf("client grant with ID %q is a system grant and cannot be deleted", id)
				}

				if err := cli.apiv3.ClientGrant.Delete(cmd.Context(), id); err != nil {
					return fmt.Errorf("failed to delete client grant with ID %q: %w", id, err)
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

func (c *cli) clientGrantPickerOptions(ctx context.Context) (pickerOptions, error) {
	return c.clientGrantPickerOptionsFiltered(ctx, false)
}

// mutableClientGrantPickerOptions lists only the grants that can actually be
// changed, dropping system grants. Auth0 rejects updating or deleting a system
// grant, so offering them in the update/delete pickers would only lead to a
// late API error on a grant the user can never modify.
func (c *cli) mutableClientGrantPickerOptions(ctx context.Context) (pickerOptions, error) {
	return c.clientGrantPickerOptionsFiltered(ctx, true)
}

func (c *cli) clientGrantPickerOptionsFiltered(ctx context.Context, excludeSystem bool) (pickerOptions, error) {
	// Fetch a single page of up to 100 grants. The API defaults to 50 per page,
	// which silently drops grants past that on busy tenants; taking 100 keeps the
	// common case selectable while still opening fast. If a grant is not on this
	// page, the user can pass its id directly.
	page, err := c.apiv3.ClientGrant.List(ctx, &managementv3.ListClientGrantsRequestParameters{
		Take: auth0.Int(100),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list client grants: %w", err)
	}

	var opts pickerOptions
	for _, grant := range page.Results {
		if excludeSystem && grant.GetIsSystem() {
			continue
		}

		identifier := grant.GetClientID()
		if identifier == "" {
			identifier = string(grant.GetDefaultFor())
		}

		label := fmt.Sprintf("%s %s", grant.GetID(), ansi.Faint("("+identifier+", "+grant.GetAudience()+")"))
		opts = append(opts, pickerOption{value: grant.GetID(), label: label})
	}

	if len(opts) == 0 {
		return nil, errors.New("there are currently no client grants to choose from. Create one by running: `auth0 client-grants create`")
	}

	return opts, nil
}

// clientGrantSubjectTypeAllowsOrganizations reports whether organizations can be
// used with the given subject type. Only client grants (the default) support
// organization settings; the API rejects them for the user and anonymous_user
// subject types.
func clientGrantSubjectTypeAllowsOrganizations(subjectType string) bool {
	return subjectType != "user" && subjectType != "anonymous_user"
}

// validateClientGrantSubjectType catches the API rule that organizations cannot
// be used with the user or anonymous_user subject types, turning the raw 400
// into a clear, actionable message for the non-interactive path.
func validateClientGrantSubjectType(subjectType, organizationUsage string, allowAnyOrganization bool) error {
	if !clientGrantSubjectTypeAllowsOrganizations(subjectType) && (organizationUsage != "" || allowAnyOrganization) {
		return fmt.Errorf("--organization-usage and --allow-any-organization cannot be set when --subject-type is %q", subjectType)
	}
	return nil
}

// clientGrantOrganizationAllowsAny reports whether allow_any_organization is
// meaningful for the given organization usage. The API only accepts a true
// allow_any_organization when organization usage is allow or require.
func clientGrantOrganizationAllowsAny(organizationUsage string) bool {
	return organizationUsage == "allow" || organizationUsage == "require"
}

// validateClientGrantOrganization catches the API rule that allow_any_organization
// may only be true when organization_usage is allow or require, turning the raw
// 400 into a clear, actionable message.
func validateClientGrantOrganization(organizationUsage string, allowAnyOrganization bool) error {
	if allowAnyOrganization && !clientGrantOrganizationAllowsAny(organizationUsage) {
		return errors.New("--allow-any-organization can only be enabled when --organization-usage is 'allow' or 'require'")
	}
	return nil
}

// validateClientGrantScopes checks scopes passed via --scopes against the
// audience API, turning a typo or an unsupported grant into a clear error rather
// than a raw API 400.
//
// When the API defines scopes, every passed scope must be one of them (or, for an
// update, already on the grant). When the API exposes no scopes at all, the
// meaning depends on the subject type: the user and anonymous_user subject types
// against the Auth0 Management API carry a fixed current_user scope set the API
// does not expose for dynamic discovery, so those cannot be validated and are
// left for the API to decide; any other subject type has nothing to grant, so
// setting --scopes is rejected outright.
func validateClientGrantScopes(resourceServer *management.ResourceServer, scopes, currentScopes []string, subjectType string) error {
	defined := make(map[string]bool)
	for _, scope := range resourceServer.GetScopes() {
		defined[scope.GetValue()] = true
	}

	if len(defined) == 0 {
		if len(scopes) > 0 && clientGrantSubjectTypeAllowsOrganizations(subjectType) {
			return fmt.Errorf(
				"the API %q does not define any scopes, so --scopes cannot be set; use --allow-all-scopes or grant no scopes instead",
				resourceServer.GetIdentifier(),
			)
		}
		return nil
	}

	// Scopes already on the grant are always valid, so an update that re-sends
	// or trims them never trips on a scope the API no longer advertises.
	for _, scope := range currentScopes {
		defined[scope] = true
	}

	var unknown []string
	for _, scope := range scopes {
		if !defined[scope] {
			unknown = append(unknown, scope)
		}
	}

	if len(unknown) > 0 {
		return fmt.Errorf(
			"the following scopes are not defined on the API %q: %s",
			resourceServer.GetIdentifier(),
			strings.Join(unknown, ", "),
		)
	}

	return nil
}

// resolveUpdateClientGrantScopes computes the scope and allow_all_scopes fields
// for a client-grant update. Choosing specific scopes and allowing every scope
// are mutually exclusive, so new scopes win, then an explicit allow-all,
// otherwise the grant keeps whatever it already authorizes (so a scope-only
// edit never drops an existing allow_all_scopes grant). When moving to specific
// scopes it also clears allow_all_scopes, because the API rejects scope while
// allow_all_scopes is still true. A nil allowAllScopes means the field is left
// unset on the request.
func resolveUpdateClientGrantScopes(newScopes []string, newAllowAll bool, currentScopes []string, currentAllowAll bool) (scope []string, allowAllScopes *bool) {
	switch {
	case len(newScopes) != 0:
		if currentAllowAll {
			return newScopes, auth0.Bool(false)
		}
		return newScopes, nil
	case newAllowAll, currentAllowAll:
		return nil, auth0.Bool(true)
	default:
		return currentScopes, nil
	}
}

// apiIdentifierPickerOptions lists the tenant APIs for the audience picker. A
// client grant's audience is the API identifier, so the picker value is the
// identifier rather than the API id used by the apis command's own picker.
func (c *cli) apiIdentifierPickerOptions(ctx context.Context) (pickerOptions, error) {
	return c.apiIdentifierPickerOptionsFiltered(ctx, false)
}

// nonSystemAPIIdentifierPickerOptions lists only non-system tenant APIs for the
// audience picker. Auth0 rejects a default client grant that targets a system
// API, so offering one in the default-grant flow would only lead to a late API
// error on an audience that can never be used for a default grant.
func (c *cli) nonSystemAPIIdentifierPickerOptions(ctx context.Context) (pickerOptions, error) {
	return c.apiIdentifierPickerOptionsFiltered(ctx, true)
}

func (c *cli) apiIdentifierPickerOptionsFiltered(ctx context.Context, excludeSystem bool) (pickerOptions, error) {
	list, err := c.api.ResourceServer.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list APIs: %w", err)
	}

	var opts pickerOptions
	for _, r := range list.ResourceServers {
		if excludeSystem && r.GetIsSystem() {
			continue
		}

		// Some APIs have no name, so fall back to a placeholder so the row
		// keeps the same "name (identifier)" shape as every other option.
		name := r.GetName()
		if name == "" {
			name = "custom API"
		}
		label := fmt.Sprintf("%s %s", name, ansi.Faint("("+r.GetIdentifier()+")"))
		opts = append(opts, pickerOption{value: r.GetIdentifier(), label: label})
	}

	if len(opts) == 0 {
		return nil, errors.New("there are currently no APIs to choose from. Create one by running: `auth0 apis create`")
	}

	return opts, nil
}

// pickClientGrantTarget drives the interactive choice of what a new grant
// authorizes: a specific client or a default group of clients. It first asks
// which of the two to target and then prompts for that target, writing the
// answer into clientID or defaultFor. The default-group values come from
// clientGrantDefaultForOptions, so new groups become selectable here without
// touching this flow.
func (c *cli) pickClientGrantTarget(cmd *cobra.Command, clientID, defaultFor *string) error {
	var target string
	targetPrompt := &survey.Select{
		Message: "What should this grant authorize?",
		Options: []string{clientGrantTargetClient, clientGrantTargetDefault},
		Default: clientGrantTargetClient,
	}
	if err := survey.AskOne(targetPrompt, &target); err != nil {
		return err
	}

	if target == clientGrantTargetDefault {
		defaultDefaultFor := clientGrantDefaultForOptions[0]
		return clientGrantDefaultFor.Select(cmd, defaultFor, clientGrantDefaultForOptions, &defaultDefaultFor)
	}

	return clientGrantClientID.Ask(cmd, clientID, nil)
}

// Scope-selection modes offered before the scopes multi-select.
const (
	clientGrantScopesModeSpecific = "Select specific scopes"
	clientGrantScopesModeAll      = "Always grant all permissions"
	clientGrantScopesModeNone     = "No scopes (grant a token with no permissions)"
)

// readClientGrantAudienceAPI reads the API (resource server) a grant's audience
// points at. The scope and authorization_details pickers both draw their options
// from this same API, so the caller reads it once and passes it to both, keeping
// the interactive flow to a single API read instead of one per picker.
func (c *cli) readClientGrantAudienceAPI(ctx context.Context, audience string) (*management.ResourceServer, error) {
	var resourceServer *management.ResourceServer
	if err := ansi.Waiting(func() (err error) {
		resourceServer, err = c.api.ResourceServer.Read(ctx, audience)
		return err
	}); err != nil {
		return nil, fmt.Errorf("failed to read the API %q: %w", audience, err)
	}
	return resourceServer, nil
}

// pickClientGrantScopes drives the interactive scope selection for a grant. It
// first asks how to grant scopes (every scope on the API, a specific set, or
// none when allowNone is set) and, for a specific set, shows a multi-select of
// the scopes the API defines, writing the chosen scopes into result. Any current
// scopes not defined by the API are still offered (and pre-selected) so an update
// never silently drops a scope already on the grant. When the API has no scopes
// at all, it warns and leaves the inputs untouched (an empty scope list, which
// the API accepts).
func (c *cli) pickClientGrantScopes(resourceServer *management.ResourceServer, result *[]string, allowAllScopes, noScopes *bool, currentScopes []string, currentAllowAll, allowNone bool) error {
	options := make([]string, 0, len(resourceServer.GetScopes()))
	seen := make(map[string]bool)
	for _, scope := range resourceServer.GetScopes() {
		options = append(options, scope.GetValue())
		seen[scope.GetValue()] = true
	}
	for _, scope := range currentScopes {
		if !seen[scope] {
			options = append(options, scope)
			seen[scope] = true
		}
	}

	if len(options) == 0 {
		c.renderer.Warnf("The API %s does not have any scopes defined.\n", ansi.Bold(resourceServer.GetName()))
		return nil
	}

	// Auth0 rejects allow_all_scopes on a system API, so only offer that mode
	// for regular APIs. System APIs still support specific scopes and none.
	modeOptions := []string{clientGrantScopesModeSpecific}
	if !resourceServer.GetIsSystem() {
		modeOptions = append(modeOptions, clientGrantScopesModeAll)
	}
	if allowNone {
		modeOptions = append(modeOptions, clientGrantScopesModeNone)
	}

	defaultMode := clientGrantScopesModeSpecific
	if currentAllowAll {
		defaultMode = clientGrantScopesModeAll
	}
	var mode string
	modePrompt := &survey.Select{
		Message: "How would you like to grant scopes?",
		Options: modeOptions,
		Default: defaultMode,
	}
	if err := survey.AskOne(modePrompt, &mode); err != nil {
		return err
	}

	switch mode {
	case clientGrantScopesModeAll:
		*allowAllScopes = true
		return nil
	case clientGrantScopesModeNone:
		*result = nil
		if noScopes != nil {
			*noScopes = true
		}
		return nil
	}

	scopesPrompt := &survey.MultiSelect{
		Message: "Scopes",
		Options: options,
		Default: currentScopes,
	}

	return survey.AskOne(scopesPrompt, result)
}

// pickClientGrantAuthorizationDetailsTypes drives the interactive selection of
// authorization_details types for a grant. The allowed types are defined on the
// audience API, so it shows a multi-select of them, writing the chosen types
// into result. Any current types not (or no longer) defined by the API are still
// offered (and pre-selected) so an update never silently drops a type already on
// the grant. When neither the API nor the grant has any types, it leaves result
// untouched (no prompt) since there is nothing to choose.
func (c *cli) pickClientGrantAuthorizationDetailsTypes(resourceServer *management.ResourceServer, result *[]string, currentTypes []string) error {
	options := make([]string, 0, len(resourceServer.GetAuthorizationDetails()))
	seen := make(map[string]bool)
	for _, detail := range resourceServer.GetAuthorizationDetails() {
		if t := detail.GetType(); t != "" && !seen[t] {
			options = append(options, t)
			seen[t] = true
		}
	}
	for _, t := range currentTypes {
		if !seen[t] {
			options = append(options, t)
			seen[t] = true
		}
	}

	if len(options) == 0 {
		return nil
	}

	typesPrompt := &survey.MultiSelect{
		Message: "Authorization details types",
		Options: options,
		Default: currentTypes,
	}

	return survey.AskOne(typesPrompt, result)
}
