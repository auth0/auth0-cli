package setup

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/auth0/go-auth0/management"
	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"

	"github.com/auth0/auth0-cli/internal/auth0"
)

// ActionReadScopes are the Management API scopes that reading the post-login
// Action and its flow needs. They are only needed when the Action is wanted.
var ActionReadScopes = []string{"read:actions"}

// ActionWriteScopes are the Management API scopes that creating, updating and
// deploying the Action, and binding it, need.
var ActionWriteScopes = []string{"create:actions", "update:actions"}

// ReadScopes are the Management API scopes the reads of this package need. They
// are also the scopes of the config command.
var ReadScopes = []string{
	"read:tenant_settings",
	"read:custom_domains",
	"read:resource_servers",
	"read:clients",
}

const (
	// CustomDomainReady is the status of a verified custom domain.
	customDomainReady = "ready"

	// InvalidQueryString is the error code of a request with a query the tenant
	// does not accept, such as a field that does not exist yet.
	invalidQueryString = "invalid_query_string"

	// ActionPageSize is the page size of the Actions API lists, and maxActionPages
	// the most pages read from one.
	actionPageSize = 50
	maxActionPages = 20

	// AppFields limits the clients listed to find web apps to what is used.
	appFields = "client_id,name,is_first_party,initiate_login_uri,allow_personal_agent_browser_join"
)

// TenantSettingsAPI is the part of the tenant settings API the reader uses.
type TenantSettingsAPI interface {
	Get(ctx context.Context, request *managementv3.GetTenantSettingsRequestParameters, opts ...option.RequestOption) (*managementv3.GetTenantSettingsResponseContent, error)
}

// CustomDomainAPI is the part of the custom domains API the reader uses.
type CustomDomainAPI interface {
	List(ctx context.Context, opts ...management.RequestOption) ([]*management.CustomDomain, error)
}

// ResourceServerAPI is the part of the resource servers API the reader uses.
type ResourceServerAPI interface {
	List(ctx context.Context, request *managementv3.ListResourceServerRequestParameters, opts ...option.RequestOption) (*auth0.ResourceServerPage, error)
}

// ClientAPI is the part of the clients API the reader uses.
type ClientAPI interface {
	Get(ctx context.Context, id string, request *managementv3.GetClientRequestParameters, opts ...option.RequestOption) (*managementv3.GetClientResponseContent, error)
	List(ctx context.Context, request *managementv3.ListClientsRequestParameters, opts ...option.RequestOption) (*auth0.ClientListPage, error)
}

// GrantAPI is the part of the client grants API the reader uses.
type GrantAPI interface {
	List(ctx context.Context, request *managementv3.ListClientGrantsRequestParameters, opts ...option.RequestOption) (*auth0.ClientGrantPage, error)
}

// ActionAPI is the part of the Actions API the reader uses.
type ActionAPI interface {
	List(ctx context.Context, opts ...management.RequestOption) (*management.ActionList, error)
	Bindings(ctx context.Context, triggerID string, opts ...management.RequestOption) (*management.ActionBindingList, error)
}

// Reader reads the tenant through the Management API.
type Reader struct {
	// TenantDomain is the canonical domain of the tenant.
	TenantDomain string
	Tenant       TenantSettingsAPI
	Domains      CustomDomainAPI
	Servers      ResourceServerAPI
	Clients      ClientAPI
	Grants       GrantAPI
	// Actions is only used by ReadAction.
	Actions ActionAPI

	// Domains holds the custom domains once they are read, so that a run lists
	// them once whichever of TenantHosts and Read asks first.
	domains     []*management.CustomDomain
	domainsRead bool
}

// Settings reads the tenant settings that Poppy cares about.
func (r *Reader) Settings(ctx context.Context) (Tenant, error) {
	settings, err := r.Tenant.Get(ctx, &managementv3.GetTenantSettingsRequestParameters{})
	if err != nil {
		return Tenant{}, fmt.Errorf("failed to read the tenant settings: %w", err)
	}

	return Tenant{
		CIMD:                     settings.GetClientIDMetadataDocumentSupported(),
		IssParameter:             settings.GetAuthorizationResponseIssParameterSupported(),
		ResourceParameterProfile: string(settings.GetResourceParameterProfile()),
		DefaultAudience:          settings.GetDefaultAudience(),
		FriendlyName:             settings.GetFriendlyName(),
		CompanyDomains:           settings.GetPersonalAgentProtocolCompanyDomains(),
	}, nil
}

// TenantHosts returns the canonical domain of the tenant and its custom domains.
func (r *Reader) TenantHosts(ctx context.Context) ([]string, error) {
	domains, err := r.customDomains(ctx)
	if err != nil {
		return nil, err
	}

	hosts := []string{r.TenantDomain}
	for _, domain := range domains {
		hosts = append(hosts, domain.GetDomain())
	}

	return hosts, nil
}

// Read reads the tenant settings (R1), the default custom domain (R2), the
// given APIs by ID or identifier (R3) and the web app by client ID or name (R4).
// A missing API or app is an *InputError. With no APIs, or with no app, the part
// is off: it is not read.
func (r *Reader) Read(ctx context.Context, apis []string, app string) (*State, error) {
	state := &State{Domain: r.TenantDomain}

	var err error
	if state.Tenant, err = r.Settings(ctx); err != nil {
		return nil, err
	}

	if state.CustomDomain, err = r.defaultCustomDomain(ctx); err != nil {
		return nil, err
	}

	for _, ref := range apis {
		api, err := r.findAPI(ctx, ref)
		if err != nil {
			return nil, err
		}

		if !slices.ContainsFunc(state.APIs, func(a API) bool { return a.ID == api.ID }) {
			state.APIs = append(state.APIs, api)
		}
	}

	if app == "" {
		return state, nil
	}

	if state.App, err = r.FindApp(ctx, app); err != nil {
		return nil, err
	}

	return state, nil
}

// ReadAction reads the Action of setup that always issues refresh tokens, if the
// tenant has it, and the post-login flow it runs in. The Action is found by its
// exact name. It stops with an *InputError when the flow has a binding that
// setup could not write back as it is.
func (r *Reader) ReadAction(ctx context.Context) (*ActionState, error) {
	actions, err := pages(func(page int) ([]*management.Action, int, error) {
		list, err := r.Actions.List(ctx, management.Parameter("actionName", OfflineAccessActionName), management.Page(page), management.PerPage(actionPageSize))
		if err != nil {
			return nil, 0, fmt.Errorf("failed to list the Actions: %w", err)
		}

		return list.Actions, list.Total, nil
	})
	if err != nil {
		return nil, err
	}

	bindings, err := readBindings(ctx, r.Actions)
	if err != nil {
		return nil, err
	}

	state := &ActionState{}
	if state.Flow, err = flowOf(bindings); err != nil {
		return nil, err
	}

	// The server-side filter is not trusted: only an Action with exactly this name is used.
	if i := slices.IndexFunc(actions, func(a *management.Action) bool { return a.GetName() == OfflineAccessActionName }); i >= 0 {
		action := actions[i]

		state.ID = action.GetID()
		state.CodeCurrent = action.GetCode() == OfflineAccessActionSource
		state.Deployed = state.CodeCurrent && action.DeployedVersion != nil && action.AllChangesDeployed
	}

	return state, nil
}

// bindingsAPI is what reading the bindings of the post-login trigger needs.
type bindingsAPI interface {
	Bindings(ctx context.Context, triggerID string, opts ...management.RequestOption) (*management.ActionBindingList, error)
}

// readBindings reads every binding of the post-login trigger, in order.
func readBindings(ctx context.Context, api bindingsAPI) ([]*management.ActionBinding, error) {
	return pages(func(page int) ([]*management.ActionBinding, int, error) {
		list, err := api.Bindings(ctx, postLoginTrigger, management.Page(page), management.PerPage(actionPageSize))
		if err != nil {
			return nil, 0, fmt.Errorf("failed to read the post-login bindings: %w", err)
		}

		return list.Bindings, list.Total, nil
	})
}

// pages reads every page of a list of the Actions API. The server may cap the page
// size below the one asked for, so a short page does not end the list: the
// total the API reports does. When the API reports no total, an empty page ends
// the list. It fails instead of returning part of the list, because a post-login
// flow that is written back with entries missing loses them.
func pages[T any](fetch func(page int) ([]T, int, error)) ([]T, error) {
	var all []T

	for page := range maxActionPages {
		items, total, err := fetch(page)
		if err != nil {
			return nil, err
		}

		all = append(all, items...)

		switch {
		case total > 0 && len(all) == total:
			return all, nil
		case total > 0 && (len(items) == 0 || len(all) > total):
			return nil, fmt.Errorf("the Actions API listed %d items but says the list has %d", len(all), total)
		case total == 0 && len(items) == 0:
			return all, nil
		}
	}

	return nil, fmt.Errorf("the Actions API list did not end after %d pages", maxActionPages)
}

// flowOf turns the bindings of the post-login trigger into the entries the update
// writes back. The API lists each binding with its Action, and an update refers
// to an Action by ID. The secrets of an Action belong to the Action, not to the
// binding, so they are not sent. A binding that lists secrets, or that names no
// Action, is refused, because the update replaces the whole list and could not
// repeat it faithfully.
func flowOf(bindings []*management.ActionBinding) ([]FlowAction, error) {
	flow := make([]FlowAction, 0, len(bindings))

	for _, binding := range bindings {
		entry := FlowAction{BindingID: binding.GetID(), DisplayName: binding.GetDisplayName()}

		switch {
		case binding.Action.GetID() != "":
			entry.RefType, entry.RefValue = management.ActionBindingReferenceByID, binding.Action.GetID()
		case binding.Ref.GetType() != "" && binding.Ref.GetValue() != "":
			entry.RefType, entry.RefValue = binding.Ref.GetType(), binding.Ref.GetValue()
		default:
			return nil, unsupportedFlow(fmt.Sprintf("A binding of the post-login trigger (%s) does not say which Action it is.", clean(binding.GetID())))
		}

		entry.Name = clean(cmp.Or(entry.DisplayName, binding.Action.GetName(), entry.RefValue))

		if len(binding.Secrets) > 0 {
			return nil, unsupportedFlow(fmt.Sprintf("The binding %s of the post-login trigger has secrets, which setup cannot read back to keep them.", entry.Name))
		}

		flow = append(flow, entry)
	}

	return flow, nil
}

func unsupportedFlow(message string) *InputError {
	return &InputError{
		Reason:  ReasonActionFlowUnsupported,
		Message: message + " Setup writes back the whole list of bindings, and could not repeat this one.",
		Fix:     "Pass --always-issue-refresh-tokens=false, or add the Action to the post-login flow in the Dashboard.",
	}
}

// BrowserJoinApps returns the apps that have the browser join on.
func (r *Reader) BrowserJoinApps(ctx context.Context) ([]App, error) {
	include, fields := true, appFields

	page, err := r.Clients.List(ctx, &managementv3.ListClientsRequestParameters{Fields: &fields, IncludeFields: &include})
	if err != nil {
		return nil, listClientsError(err)
	}

	clients, err := collect(ctx, page)
	if err != nil {
		return nil, listClientsError(err)
	}

	var apps []App
	for _, client := range clients {
		if client.GetAllowPersonalAgentBrowserJoin() {
			apps = append(apps, App{
				ClientID:    client.GetClientID(),
				Name:        client.GetName(),
				FirstParty:  client.GetIsFirstParty(),
				LoginURI:    client.GetInitiateLoginURI(),
				BrowserJoin: true,
			})
		}
	}

	return apps, nil
}

// customDomains lists the custom domains of the tenant, once per reader. A failed
// read is not kept, so that the next call tries again.
func (r *Reader) customDomains(ctx context.Context) ([]*management.CustomDomain, error) {
	if r.domainsRead {
		return r.domains, nil
	}

	domains, err := r.Domains.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list the custom domains: %w", err)
	}

	r.domains, r.domainsRead = domains, true

	return domains, nil
}

// listClientsError explains a failure to list the clients with the Poppy fields.
// A tenant without the Poppy beta rejects the fields with a 400 invalid_query_string
// error, which says that the beta is missing.
func listClientsError(err error) error {
	var apiErr *core.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusBadRequest && strings.Contains(err.Error(), invalidQueryString) {
		return &InputError{
			Reason:  ReasonPoppyUnavailable,
			Message: "This tenant doesn't have the Poppy beta.",
			Fix:     "Contact Auth0 support to enable it.",
		}
	}

	return fmt.Errorf("failed to list the clients: %w", err)
}

// defaultCustomDomain returns the default verified custom domain, or "" when the
// tenant has none. A single verified domain is the default whatever its flag.
func (r *Reader) defaultCustomDomain(ctx context.Context) (string, error) {
	domains, err := r.customDomains(ctx)
	if err != nil {
		return "", err
	}

	ready := slices.DeleteFunc(slices.Clone(domains), func(d *management.CustomDomain) bool {
		return d.GetStatus() != customDomainReady
	})

	for _, domain := range ready {
		if domain.GetIsDefault() || len(ready) == 1 {
			return domain.GetDomain(), nil
		}
	}

	return "", nil
}

// findAPI reads an API by identifier, or by ID when no identifier matches.
func (r *Reader) findAPI(ctx context.Context, ref string) (API, error) {
	servers, err := r.servers(ctx, &managementv3.ListResourceServerRequestParameters{Identifiers: []*string{&ref}})
	if err != nil {
		return API{}, err
	}

	// The server-side filter is not trusted: only an API that is the one asked for is used.
	servers = slices.DeleteFunc(servers, func(s *managementv3.ResourceServer) bool { return s.GetIdentifier() != ref })

	if len(servers) == 0 {
		if servers, err = r.servers(ctx, &managementv3.ListResourceServerRequestParameters{}); err != nil {
			return API{}, err
		}

		servers = slices.DeleteFunc(servers, func(s *managementv3.ResourceServer) bool { return s.GetID() != ref })
	}

	if len(servers) == 0 {
		return API{}, &InputError{
			Reason:  ReasonAPINotFound,
			Message: fmt.Sprintf("API %s not found.", ref),
			Fix:     "Create it first: auth0 apis create",
		}
	}

	return apiFromServer(servers[0]), nil
}

// apiFromServer reads the settings that Poppy changes from a resource server.
func apiFromServer(server *managementv3.ResourceServer) API {
	api := API{
		ID:                 server.GetID(),
		Identifier:         server.GetIdentifier(),
		Name:               server.GetName(),
		AllowOfflineAccess: server.GetAllowOfflineAccess(),
		AgentSubjectClaims: string(server.GetAgentSubjectClaims()),
		Poppy:              server.GetUseForPersonalAgentProtocol(),
	}

	for _, scope := range server.GetScopes() {
		api.Scopes = append(api.Scopes, Scope{Name: scope.GetValue(), Description: scope.GetDescription()})
	}

	subjects := server.GetSubjectTypeAuthorization()
	guests := subjects.GetAnonymousUser()
	api.GuestPolicy = string(guests.GetPolicy())

	user, client := subjects.GetUser(), subjects.GetClient()
	api.UserPolicy = string(user.GetPolicy())
	api.ClientPolicy = string(client.GetPolicy())

	return api
}

// ListAPIs returns every API of the tenant with the settings that Poppy changes,
// whether or not it has the Personal Agent Protocol on.
func (r *Reader) ListAPIs(ctx context.Context) ([]API, error) {
	servers, err := r.servers(ctx, &managementv3.ListResourceServerRequestParameters{})
	if err != nil {
		return nil, err
	}

	apis := make([]API, len(servers))
	for i, server := range servers {
		apis[i] = apiFromServer(server)
	}

	return apis, nil
}

// GrantedScopes returns the scopes the app holds on an API for signed-in users
// and for guests, without duplicates.
func (r *Reader) GrantedScopes(ctx context.Context, clientID, audience string) ([]string, error) {
	grants, err := r.grants(ctx, &managementv3.ListClientGrantsRequestParameters{ClientID: &clientID, Audience: &audience})
	if err != nil {
		return nil, fmt.Errorf("failed to read the grants of %s: %w", clientID, err)
	}

	scopes := []string{}
	for _, grant := range grants {
		switch grant.GetSubjectType() {
		case managementv3.ClientGrantSubjectTypeEnumUser, managementv3.ClientGrantSubjectTypeEnumAnonymousUser:
			for _, scope := range grant.GetScope() {
				if !slices.Contains(scopes, scope) {
					scopes = append(scopes, scope)
				}
			}
		}
	}

	return scopes, nil
}

func (r *Reader) servers(ctx context.Context, request *managementv3.ListResourceServerRequestParameters) ([]*managementv3.ResourceServer, error) {
	page, err := r.Servers.List(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to list the APIs: %w", err)
	}

	servers, err := collect(ctx, page)
	if err != nil {
		return nil, fmt.Errorf("failed to list the APIs: %w", err)
	}

	return servers, nil
}

// FindApp reads the web app by client ID, or by name when no client has that
// ID. A name is not a valid client ID, so the API may answer a name with 400
// instead of 404; both mean the reference is not an ID.
func (r *Reader) FindApp(ctx context.Context, ref string) (App, error) {
	client, err := r.Clients.Get(ctx, ref, &managementv3.GetClientRequestParameters{})

	var apiErr *core.APIError
	if errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusNotFound || apiErr.StatusCode == http.StatusBadRequest) {
		id, byNameErr := r.clientIDByName(ctx, ref)
		if byNameErr != nil {
			return App{}, byNameErr
		}

		client, err = r.Clients.Get(ctx, id, &managementv3.GetClientRequestParameters{})
	}

	if err != nil {
		return App{}, fmt.Errorf("failed to read the app %s: %w", ref, err)
	}

	return App{
		ClientID:    client.GetClientID(),
		Name:        client.GetName(),
		FirstParty:  client.GetIsFirstParty(),
		LoginURI:    client.GetInitiateLoginURI(),
		BrowserJoin: client.GetAllowPersonalAgentBrowserJoin(),
	}, nil
}

// clientIDByName returns the ID of the only client with exactly the given name.
func (r *Reader) clientIDByName(ctx context.Context, name string) (string, error) {
	include, fields := true, "client_id,name"

	// The API has no filter by name: q accepts only a few client_grant queries.
	// So list every client and keep the ones with this name.
	page, err := r.Clients.List(ctx, &managementv3.ListClientsRequestParameters{Fields: &fields, IncludeFields: &include})
	if err != nil {
		return "", fmt.Errorf("failed to look up the app %s: %w", name, err)
	}

	clients, err := collect(ctx, page)
	if err != nil {
		return "", fmt.Errorf("failed to look up the app %s: %w", name, err)
	}

	clients = slices.DeleteFunc(clients, func(c *managementv3.Client) bool { return c.GetName() != name })

	switch len(clients) {
	case 0:
		return "", &InputError{
			Reason:  ReasonAppNotFound,
			Message: fmt.Sprintf("App %s not found.", name),
			Fix:     "Create it first: auth0 apps create",
		}
	case 1:
		return clients[0].GetClientID(), nil
	}

	ids := make([]string, len(clients))
	for i, client := range clients {
		ids[i] = client.GetClientID()
	}

	return "", &InputError{
		Reason:     ReasonAmbiguousApp,
		Message:    fmt.Sprintf("%d apps are named %s.", len(clients), name),
		Fix:        "Pass the client ID with --app.",
		Candidates: ids,
	}
}

func (r *Reader) grants(ctx context.Context, request *managementv3.ListClientGrantsRequestParameters) ([]*managementv3.ClientGrantResponseContent, error) {
	page, err := r.Grants.List(ctx, request)
	if err != nil {
		return nil, err
	}

	return collect(ctx, page)
}

// collect reads every page of a list response.
func collect[C comparable, T, R any](ctx context.Context, page *core.Page[C, T, R]) ([]T, error) {
	var items []T

	iter := page.Iterator()
	for iter.Next(ctx) {
		items = append(items, iter.Current())
	}

	return items, iter.Err()
}
