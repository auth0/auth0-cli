// Package setup reads the tenant for the Poppy setup and config commands,
// validates their inputs, plans the changes that setup makes and derives the
// values of the config file. It makes no Management API writes and has no
// dependency on Cobra.
package setup

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/auth0/go-auth0/management"
	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/poppy/config"
)

// ReadScopes are the Management API scopes the reads of this package need. They
// are also the scopes of the config command.
var ReadScopes = []string{
	"read:tenant_settings",
	"read:custom_domains",
	"read:resource_servers",
	"read:clients",
	"read:client_grants",
}

const (
	// CustomDomainReady is the status of a verified custom domain.
	customDomainReady = "ready"

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

// Tenant holds the tenant settings that Poppy cares about.
type Tenant struct {
	// CIMD tells whether agents identified by URL (client ID metadata
	// documents) are allowed.
	CIMD bool
	// IssParameter tells whether authorization responses carry the iss parameter.
	IssParameter             bool
	ResourceParameterProfile string
	DefaultAudience          string
	FriendlyName             string
	// CompanyDomains holds entries such as https://shop.example.
	CompanyDomains []string
}

// Scope is a scope of an API.
type Scope struct {
	Name        string
	Description string
}

// API is a resource server with the settings that Poppy changes.
type API struct {
	ID         string
	Identifier string
	// Name is the name of the API in the tenant. The config file uses it as the
	// description of a new apis entry.
	Name               string
	Scopes             []Scope
	AllowOfflineAccess bool
	AgentSubjectClaims string
	// Poppy tells whether the API has the Personal Agent Protocol on.
	Poppy bool
	// GuestPolicy is the policy for the anonymous_user subject type.
	GuestPolicy string
	// UserPolicy and ClientPolicy are the policies for the user and client
	// subject types, empty when the API has none. Updating the anonymous_user
	// policy sends them back unchanged, so that the update cannot reset them.
	UserPolicy   string
	ClientPolicy string
}

// App is the web app used for the browser join.
type App struct {
	ClientID    string
	Name        string
	FirstParty  bool
	LoginURI    string
	BrowserJoin bool
}

// State is what the tenant holds today, as far as setup and config care.
type State struct {
	Tenant Tenant
	// Domain is the canonical domain of the tenant.
	Domain string
	// CustomDomain is the default verified custom domain, empty when there is none.
	CustomDomain string
	// APIs are the requested APIs, in the order they were requested.
	APIs []API
	App  App
	// HasGuestGrant tells, by API identifier, whether the app already has a
	// grant for signed-out guests.
	HasGuestGrant map[string]bool
	// File is the existing config file, nil when there is none. The caller
	// fills it.
	File *config.Config
}

// Issuer returns the issuer: the default verified custom domain, else the
// canonical domain of the tenant.
func (s *State) Issuer() string {
	return "https://" + cmp.Or(s.CustomDomain, s.Domain) + "/"
}

// usesCanonicalDomain tells whether the issuer is the Auth0 domain of the tenant.
func (s *State) usesCanonicalDomain() bool {
	return s.CustomDomain == ""
}

// fileEntries returns the indexes of the apis entries of the existing config
// file that belong to an API, which are those at the origin of its identifier.
func (s *State) fileEntries(identifier string) []int {
	if s.File == nil {
		return nil
	}

	return s.File.EntriesFor(identifier)
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
	domains, err := r.Domains.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list the custom domains: %w", err)
	}

	hosts := []string{r.TenantDomain}
	for _, domain := range domains {
		hosts = append(hosts, domain.GetDomain())
	}

	return hosts, nil
}

// Read reads the tenant settings (R1), the default custom domain (R2), the
// given APIs by ID or identifier (R3), the web app by client ID or name (R4) and
// the grants of the app to each API (R5). A missing API or app is an
// *InputError.
func (r *Reader) Read(ctx context.Context, apis []string, app string) (*State, error) {
	state := &State{Domain: r.TenantDomain, HasGuestGrant: make(map[string]bool, len(apis))}

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

	if state.App, err = r.FindApp(ctx, app); err != nil {
		return nil, err
	}

	for _, api := range state.APIs {
		grants, err := r.grants(ctx, &managementv3.ListClientGrantsRequestParameters{
			ClientID: &state.App.ClientID,
			Audience: &api.Identifier,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to read the grants of %s: %w", state.App.Name, err)
		}

		state.HasGuestGrant[api.Identifier] = slices.ContainsFunc(grants, func(g *managementv3.ClientGrantResponseContent) bool {
			return g.GetSubjectType() == managementv3.ClientGrantSubjectTypeEnumAnonymousUser
		})
	}

	return state, nil
}

// BrowserJoinApps returns the apps that have the browser join on.
func (r *Reader) BrowserJoinApps(ctx context.Context) ([]App, error) {
	include, fields := true, appFields

	page, err := r.Clients.List(ctx, &managementv3.ListClientsRequestParameters{Fields: &fields, IncludeFields: &include})
	if err != nil {
		return nil, fmt.Errorf("failed to list the clients: %w", err)
	}

	clients, err := collect(ctx, page)
	if err != nil {
		return nil, fmt.Errorf("failed to list the clients: %w", err)
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

// defaultCustomDomain returns the default verified custom domain, or "" when the
// tenant has none. A single verified domain is the default whatever its flag.
func (r *Reader) defaultCustomDomain(ctx context.Context) (string, error) {
	domains, err := r.Domains.List(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list the custom domains: %w", err)
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
			Reason:  "api_not_found",
			Message: fmt.Sprintf("API %s not found.", ref),
			Fix:     "Create it first: auth0 apis create",
		}
	}

	server := servers[0]
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

	return api, nil
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
			Reason:  "app_not_found",
			Message: fmt.Sprintf("App %s not found.", name),
			Fix:     "Create it first: auth0 apps create",
		}
	case 1:
		return clients[0].GetClientID(), nil
	}

	return "", &InputError{
		Reason:  "app_ambiguous",
		Message: fmt.Sprintf("%d apps are named %s.", len(clients), name),
		Fix:     "Pass the client ID with --app.",
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
