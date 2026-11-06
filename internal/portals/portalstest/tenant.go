// Package portalstest provides an in-memory tenant for testing code that uses the portals
// stores. It enforces the rules setup and audit depend on: a duplicate resource server
// identifier, portal slug or organization name conflicts, the secret of an application is only
// returned when it is created or read on its own, and a write probe with an empty body is a
// validation error, as it is on the Management API.
package portalstest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/auth0/go-auth0/v3/management/core"

	"github.com/auth0/auth0-cli/internal/portals"
)

// idShape is the look of an ID the Management API returns. IDs come from a counter, so a run
// can be repeated and its output compared byte for byte.
type idShape struct {
	prefix string
	length int
}

var (
	idShapes = map[string]idShape{
		"rs":  {prefix: "", length: 24},
		"cli": {prefix: "", length: 32},
		"cgr": {prefix: "cgr_", length: 16},
	}

	formIDPattern = regexp.MustCompile(`^ap_[A-Za-z0-9]{22}$`)
)

const defaultIDLength = 22

// Tenant is an in-memory tenant that implements portals.API.
//
// A request is named like the Management API route behind it, "METHOD path" without the query
// and with the IDs in place, for example "POST clients" or "PATCH client-grants/cgr_1". Calls,
// Deny and FailNext use these names.
type Tenant struct {
	mu sync.Mutex

	nextID int
	calls  []string

	denials  map[string]string
	failures map[string]int

	resourceServers []portals.ResourceServer
	clients         []clientRecord
	grants          []portals.ClientGrant
	vaults          []vaultRecord
	forms           []formRecord
	portals         []portals.Portal
	users           []portals.User
	organizations   []portals.Organization
	members         map[string][]string
	orgConnections  map[string][]string
	orgRoles        map[string][]string
	roles           []roleRecord
	connections     []connectionRecord
	customDomain    *portals.CustomDomain
}

type clientRecord struct {
	portals.Client
	enabledConnections []string
}

type vaultRecord struct {
	portals.VaultConnection
	setup portals.VaultSetup
	// The scopes the vault session carries, set when the setup is sent.
	mintedWithScopes []string
}

type formRecord struct {
	portals.Form
	flowID string
}

type roleRecord struct {
	portals.Role
	permissions []portals.Permission
}

type connectionRecord struct {
	portals.Connection
	strategy string
}

var _ portals.API = (*Tenant)(nil)

// New returns an empty tenant.
func New() *Tenant {
	return &Tenant{
		nextID:         1,
		denials:        map[string]string{},
		failures:       map[string]int{},
		members:        map[string][]string{},
		orgConnections: map[string][]string{},
		orgRoles:       map[string][]string{},
	}
}

// Deny makes every request named key fail with the Management API's scope error.
func (t *Tenant) Deny(key, message string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.denials[key] = message
}

// FailNext makes the next request named key fail once with the status.
func (t *Tenant) FailNext(key string, status int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.failures[key] = status
}

// Calls returns the requests received, in order.
func (t *Tenant) Calls() []string {
	t.mu.Lock()
	defer t.mu.Unlock()

	return slices.Clone(t.calls)
}

// Writes returns the requests received that are not reads, in order.
func (t *Tenant) Writes() []string {
	var writes []string
	for _, call := range t.Calls() {
		if !strings.HasPrefix(call, http.MethodGet+" ") {
			writes = append(writes, call)
		}
	}

	return writes
}

// begin records a request and returns the error it must fail with, if any. The caller holds the lock.
func (t *Tenant) begin(key string) error {
	if err := t.record(key); err != nil {
		return err
	}

	return t.takeFailure(key)
}

// record records a request and returns the scope error if the request is denied.
func (t *Tenant) record(key string) error {
	t.calls = append(t.calls, key)

	if message, denied := t.denials[key]; denied {
		return withCause(&portals.APIError{Status: http.StatusForbidden, Code: "insufficient_scope", Message: message})
	}

	return nil
}

// takeFailure returns the failure set for the request, once.
func (t *Tenant) takeFailure(key string) error {
	if status, failing := t.failures[key]; failing {
		delete(t.failures, key)
		return withCause(&portals.APIError{Status: status, Message: "simulated failure"})
	}

	return nil
}

func (t *Tenant) newID(kind string) string {
	t.nextID++

	shape, known := idShapes[kind]
	if !known {
		shape = idShape{prefix: kind + "_", length: defaultIDLength}
	}

	return fmt.Sprintf("%s%0*d", shape.prefix, shape.length, t.nextID)
}

// withCause gives an error the SDK error that the clients of the CLI keep in it, so the CLI classifies
// the failures of the fake as it does those of the API.
func withCause(apiErr *portals.APIError) *portals.APIError {
	apiErr.Err = core.NewAPIError(apiErr.Status, nil, errors.New(apiErr.Message))

	return apiErr
}

func notFound(message string) error {
	return withCause(&portals.APIError{Status: http.StatusNotFound, Message: message})
}

func conflict(message string) error {
	return withCause(&portals.APIError{Status: http.StatusConflict, Message: message})
}

// clone copies a value completely, so a caller cannot change the tenant through what it was given.
func clone[T any](value T) T {
	var copied T

	data, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("portalstest: cannot copy %T: %v", value, err))
	}

	if err := json.Unmarshal(data, &copied); err != nil {
		panic(fmt.Sprintf("portalstest: cannot copy %T: %v", value, err))
	}

	return copied
}

// ListResourceServers implements portals.ResourceServerStore.
func (t *Tenant) ListResourceServers(_ context.Context) ([]portals.ResourceServer, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("GET resource-servers"); err != nil {
		return nil, err
	}

	return clone(t.resourceServers), nil
}

// CreateResourceServer implements portals.ResourceServerStore.
func (t *Tenant) CreateResourceServer(_ context.Context, spec portals.ResourceServerSpec) (portals.ResourceServer, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("POST resource-servers"); err != nil {
		return portals.ResourceServer{}, err
	}

	for _, server := range t.resourceServers {
		if server.Identifier == spec.Identifier {
			return portals.ResourceServer{}, conflict("The resource server identifier already exists")
		}
	}

	server := portals.ResourceServer{ID: t.newID("rs"), Identifier: spec.Identifier, Name: spec.Name}
	t.resourceServers = append(t.resourceServers, server)

	return server, nil
}

// ListClients implements portals.ClientStore. Like the list endpoint of the setup, it returns only
// the ID, name, metadata and session transfer.
func (t *Tenant) ListClients(_ context.Context) ([]portals.Client, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("GET clients"); err != nil {
		return nil, err
	}

	listed := make([]portals.Client, 0, len(t.clients))
	for _, record := range t.clients {
		listed = append(listed, clone(portals.Client{
			ID: record.ID,
			ClientSettings: portals.ClientSettings{
				Name:            record.Name,
				Metadata:        record.Metadata,
				SessionTransfer: record.SessionTransfer,
			},
		}))
	}

	return listed, nil
}

// ReadClient implements portals.ClientStore.
func (t *Tenant) ReadClient(_ context.Context, id string) (portals.Client, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("GET clients/" + id); err != nil {
		return portals.Client{}, err
	}

	record, found := t.findClient(id)
	if !found {
		return portals.Client{}, notFound("not found")
	}

	client := clone(record.Client)
	client.Secret = ""

	return client, nil
}

// ReadClientSecret implements portals.ClientStore.
func (t *Tenant) ReadClientSecret(_ context.Context, id string) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("GET clients/" + id); err != nil {
		return "", err
	}

	record, found := t.findClient(id)
	if !found {
		return "", notFound("not found")
	}

	return record.Secret, nil
}

// CreateClient implements portals.ClientStore.
func (t *Tenant) CreateClient(_ context.Context, settings portals.ClientSettings) (portals.Client, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("POST clients"); err != nil {
		return portals.Client{}, err
	}

	id := t.newID("cli")
	client := portals.Client{
		ID:             id,
		Secret:         fmt.Sprintf("%064d", t.nextID),
		ClientSettings: clone(settings),
	}
	t.clients = append(t.clients, clientRecord{Client: client})

	return clone(client), nil
}

// UpdateClient implements portals.ClientStore.
func (t *Tenant) UpdateClient(_ context.Context, id string, settings portals.ClientSettings) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("PATCH clients/" + id); err != nil {
		return err
	}

	index := slices.IndexFunc(t.clients, func(record clientRecord) bool { return record.ID == id })
	if index < 0 {
		return notFound("not found")
	}

	mergeClientSettings(&t.clients[index].ClientSettings, clone(settings))

	return nil
}

// mergeClientSettings copies the settings that are set over the current ones. A list left empty is
// not set, like in the requests of the real client, so it cannot clear the current one.
func mergeClientSettings(current *portals.ClientSettings, changes portals.ClientSettings) {
	if changes.Name != "" {
		current.Name = changes.Name
	}

	if changes.AppType != "" {
		current.AppType = changes.AppType
	}

	if changes.TokenEndpointAuthMethod != "" {
		current.TokenEndpointAuthMethod = changes.TokenEndpointAuthMethod
	}

	if changes.OrganizationUsage != "" {
		current.OrganizationUsage = changes.OrganizationUsage
	}

	if changes.OrganizationRequireBehavior != "" {
		current.OrganizationRequireBehavior = changes.OrganizationRequireBehavior
	}

	if changes.IsFirstParty != nil {
		current.IsFirstParty = changes.IsFirstParty
	}

	if changes.OIDCConformant != nil {
		current.OIDCConformant = changes.OIDCConformant
	}

	if changes.IPHeaderTrusted != nil {
		current.IPHeaderTrusted = changes.IPHeaderTrusted
	}

	if changes.RequireProofOfPossession != nil {
		current.RequireProofOfPossession = changes.RequireProofOfPossession
	}

	if len(changes.Callbacks) > 0 {
		current.Callbacks = changes.Callbacks
	}

	if len(changes.AllowedLogoutURLs) > 0 {
		current.AllowedLogoutURLs = changes.AllowedLogoutURLs
	}

	if len(changes.GrantTypes) > 0 {
		current.GrantTypes = changes.GrantTypes
	}

	if changes.Metadata != nil {
		current.Metadata = changes.Metadata
	}

	if changes.RefreshToken != nil {
		current.RefreshToken = changes.RefreshToken
	}

	if changes.SessionTransfer != nil {
		current.SessionTransfer = changes.SessionTransfer
	}

	if changes.OIDCLogout != nil {
		current.OIDCLogout = changes.OIDCLogout
	}
}

func (t *Tenant) findClient(id string) (clientRecord, bool) {
	index := slices.IndexFunc(t.clients, func(record clientRecord) bool { return record.ID == id })
	if index < 0 {
		return clientRecord{}, false
	}

	return t.clients[index], true
}

// ListClientGrants implements portals.ClientGrantStore.
func (t *Tenant) ListClientGrants(_ context.Context, clientID string) ([]portals.ClientGrant, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("GET client-grants"); err != nil {
		return nil, err
	}

	var listed []portals.ClientGrant
	for _, grant := range t.grants {
		if grant.ClientID == clientID {
			listed = append(listed, clone(grant))
		}
	}

	return listed, nil
}

// CreateClientGrant implements portals.ClientGrantStore.
func (t *Tenant) CreateClientGrant(_ context.Context, spec portals.ClientGrantSpec) (portals.ClientGrant, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("POST client-grants"); err != nil {
		return portals.ClientGrant{}, err
	}

	grant := portals.ClientGrant{
		ID:       t.newID("cgr"),
		ClientID: spec.ClientID,
		Audience: spec.Audience,
		Scope:    slices.Clone(spec.Scope),
	}
	t.grants = append(t.grants, grant)

	return clone(grant), nil
}

// UpdateClientGrantScope implements portals.ClientGrantStore.
func (t *Tenant) UpdateClientGrantScope(_ context.Context, id string, scope []string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("PATCH client-grants/" + id); err != nil {
		return err
	}

	index := slices.IndexFunc(t.grants, func(grant portals.ClientGrant) bool { return grant.ID == id })
	if index < 0 {
		return notFound("The grant does not exist")
	}

	t.grants[index].Scope = slices.Clone(scope)

	return nil
}

// DeleteClientGrant implements portals.ClientGrantStore.
func (t *Tenant) DeleteClientGrant(_ context.Context, id string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("DELETE client-grants/" + id); err != nil {
		return err
	}

	index := slices.IndexFunc(t.grants, func(grant portals.ClientGrant) bool { return grant.ID == id })
	if index < 0 {
		return notFound("The grant does not exist")
	}

	t.grants = slices.Delete(t.grants, index, index+1)

	return nil
}

// ListVaultConnections implements portals.VaultStore.
func (t *Tenant) ListVaultConnections(_ context.Context) ([]portals.VaultConnection, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("GET flows/vault/connections"); err != nil {
		return nil, err
	}

	listed := make([]portals.VaultConnection, 0, len(t.vaults))
	for _, vault := range t.vaults {
		listed = append(listed, vault.VaultConnection)
	}

	return listed, nil
}

// CreateVaultConnection implements portals.VaultStore.
func (t *Tenant) CreateVaultConnection(_ context.Context, name string, setup portals.VaultSetup) (portals.VaultConnection, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("POST flows/vault/connections"); err != nil {
		return portals.VaultConnection{}, err
	}

	vault := vaultRecord{
		VaultConnection: portals.VaultConnection{ID: t.newID("ac"), Name: name},
		setup:           setup,
	}
	t.vaults = append(t.vaults, vault)

	return vault.VaultConnection, nil
}

// UpdateVaultConnection implements portals.VaultStore.
func (t *Tenant) UpdateVaultConnection(_ context.Context, id string, setup portals.VaultSetup) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("PATCH flows/vault/connections/" + id); err != nil {
		return err
	}

	index := slices.IndexFunc(t.vaults, func(vault vaultRecord) bool { return vault.ID == id })
	if index < 0 {
		return notFound("Connection not found")
	}

	t.vaults[index].setup = setup
	t.vaults[index].mintedWithScopes = t.managementScopes(setup.ClientID)

	return nil
}

// managementScopes are the scopes a token of the client would carry for the Management API.
func (t *Tenant) managementScopes(clientID string) []string {
	for _, grant := range t.grants {
		if grant.ClientID == clientID && strings.HasSuffix(grant.Audience, "/api/v2/") {
			return slices.Clone(grant.Scope)
		}
	}

	return nil
}

// VaultMintedScopes returns the scopes the session of a vault connection carries. The session is
// minted when the connection is created or its setup is sent again.
func (t *Tenant) VaultMintedScopes(id string) []string {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, vault := range t.vaults {
		if vault.ID == id {
			return slices.Clone(vault.mintedWithScopes)
		}
	}

	return nil
}

// ListForms implements portals.FormStore.
func (t *Tenant) ListForms(_ context.Context) ([]portals.Form, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("GET forms"); err != nil {
		return nil, err
	}

	listed := make([]portals.Form, 0, len(t.forms))
	for _, form := range t.forms {
		listed = append(listed, form.Form)
	}

	return listed, nil
}

// ReadForm implements portals.FormStore.
func (t *Tenant) ReadForm(_ context.Context, id string) (portals.FormDetail, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("GET forms/" + id); err != nil {
		return portals.FormDetail{}, err
	}

	if !formIDPattern.MatchString(id) {
		return portals.FormDetail{}, withCause(&portals.APIError{
			Status:  http.StatusBadRequest,
			Message: "The provided input does not satisfy the expected schema",
		})
	}

	index := slices.IndexFunc(t.forms, func(form formRecord) bool { return form.ID == id })
	if index < 0 {
		return portals.FormDetail{}, notFound("Form not found")
	}

	return portals.FormDetail{ID: id, FlowID: t.forms[index].flowID}, nil
}

// ImportForm implements portals.FormStore. A document is a form with the name at form.name.
func (t *Tenant) ImportForm(_ context.Context, document any) (portals.ImportedForm, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("POST forms/import"); err != nil {
		return portals.ImportedForm{}, err
	}

	var parsed struct {
		Form struct {
			Name string `json:"name"`
		} `json:"form"`
	}

	data, err := json.Marshal(document)
	if err != nil {
		return portals.ImportedForm{}, fmt.Errorf("portalstest: cannot read the form document: %w", err)
	}

	if err := json.Unmarshal(data, &parsed); err != nil {
		return portals.ImportedForm{}, fmt.Errorf("portalstest: cannot read the form document: %w", err)
	}

	form := formRecord{Form: portals.Form{ID: t.newID("ap"), Name: parsed.Form.Name}, flowID: t.newID("af")}
	t.forms = append(t.forms, form)

	return portals.ImportedForm{ID: form.ID, FlowID: form.flowID}, nil
}

// ListPortals implements portals.PortalStore.
func (t *Tenant) ListPortals(_ context.Context) ([]portals.PortalSummary, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("GET portals"); err != nil {
		return nil, err
	}

	listed := make([]portals.PortalSummary, 0, len(t.portals))
	for _, portal := range t.portals {
		listed = append(listed, portals.PortalSummary{ID: portal.ID, Slug: portal.Slug, Name: portal.Name})
	}

	return listed, nil
}

// ReadPortal implements portals.PortalStore.
func (t *Tenant) ReadPortal(_ context.Context, id string) (portals.Portal, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("GET portals/" + id); err != nil {
		return portals.Portal{}, err
	}

	index := slices.IndexFunc(t.portals, func(portal portals.Portal) bool { return portal.ID == id })
	if index < 0 {
		return portals.Portal{}, notFound("not found")
	}

	return clone(t.portals[index]), nil
}

// CreatePortal implements portals.PortalStore. Like the API, it keeps only the ID of the application.
func (t *Tenant) CreatePortal(_ context.Context, request portals.CreatePortalRequest) (portals.Portal, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("POST portals"); err != nil {
		return portals.Portal{}, err
	}

	for _, portal := range t.portals {
		if portal.Slug == request.Slug {
			return portals.Portal{}, withCause(&portals.APIError{
				Status:  http.StatusConflict,
				Code:    "portal_conflict",
				Message: "Slug already in use",
			})
		}
	}

	portal := portals.Portal{
		ID:         t.newID("up"),
		Slug:       request.Slug,
		Name:       request.Name,
		Navigation: clone(request.Navigation),
		Pages:      clone(request.Pages),
	}
	if request.Client != nil {
		portal.Client = &portals.PortalClient{ClientID: request.Client.ClientID}
	}

	t.portals = append(t.portals, portal)

	return clone(portal), nil
}

// UpdatePortal implements portals.PortalStore.
func (t *Tenant) UpdatePortal(_ context.Context, id string, patch portals.PortalPatch) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("PATCH portals/" + id); err != nil {
		return err
	}

	index := slices.IndexFunc(t.portals, func(portal portals.Portal) bool { return portal.ID == id })
	if index < 0 {
		return notFound("not found")
	}

	portal := &t.portals[index]
	if patch.Client != nil {
		portal.Client = &portals.PortalClient{ClientID: patch.Client.ClientID}
	}

	if patch.Navigation != nil {
		portal.Navigation = clone(patch.Navigation)
	}

	if patch.Pages != nil {
		portal.Pages = clone(patch.Pages)
	}

	return nil
}

// SetCustomDomain sets the default custom domain of the tenant. A nil domain removes it.
func (t *Tenant) SetCustomDomain(domain *portals.CustomDomain) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.customDomain = clone(domain)
}

// ReadDefaultCustomDomain implements portals.CustomDomainStore.
func (t *Tenant) ReadDefaultCustomDomain(_ context.Context) (portals.CustomDomain, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("GET custom-domains/default"); err != nil {
		return portals.CustomDomain{}, err
	}

	if t.customDomain == nil {
		return portals.CustomDomain{}, notFound("Not Found")
	}

	return *t.customDomain, nil
}

// SearchUsersByEmail implements portals.UserStore. The search matches the text anywhere in the
// email, like the real search.
func (t *Tenant) SearchUsersByEmail(_ context.Context, email string) ([]portals.User, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("GET users"); err != nil {
		return nil, err
	}

	var found []portals.User
	for _, user := range t.users {
		if strings.Contains(user.Email, email) {
			found = append(found, user)
		}
	}

	return found, nil
}

// ReadUser implements portals.UserStore.
func (t *Tenant) ReadUser(_ context.Context, id string) (portals.User, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("GET users/" + id); err != nil {
		return portals.User{}, err
	}

	index := slices.IndexFunc(t.users, func(user portals.User) bool { return user.ID == id })
	if index < 0 {
		return portals.User{}, notFound("The user does not exist.")
	}

	return t.users[index], nil
}

// ListUserOrganizations implements portals.UserStore.
func (t *Tenant) ListUserOrganizations(_ context.Context, userID string) ([]portals.Organization, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("GET users/" + userID + "/organizations"); err != nil {
		return nil, err
	}

	var memberships []portals.Organization
	for _, organization := range t.organizations {
		if slices.Contains(t.members[organization.ID], userID) {
			memberships = append(memberships, organization)
		}
	}

	return memberships, nil
}

// ListOrganizations implements portals.OrganizationStore.
func (t *Tenant) ListOrganizations(_ context.Context) ([]portals.Organization, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("GET organizations"); err != nil {
		return nil, err
	}

	return slices.Clone(t.organizations), nil
}

// ReadOrganization implements portals.OrganizationStore.
func (t *Tenant) ReadOrganization(_ context.Context, id string) (portals.Organization, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("GET organizations/" + id); err != nil {
		return portals.Organization{}, err
	}

	index := slices.IndexFunc(t.organizations, func(organization portals.Organization) bool { return organization.ID == id })
	if index < 0 {
		return portals.Organization{}, notFound("The organization does not exist.")
	}

	return t.organizations[index], nil
}

// CreateOrganization implements portals.OrganizationStore.
func (t *Tenant) CreateOrganization(_ context.Context, spec portals.OrganizationSpec) (portals.Organization, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("POST organizations"); err != nil {
		return portals.Organization{}, err
	}

	for _, organization := range t.organizations {
		if organization.Name == spec.Name {
			return portals.Organization{}, conflict("The organization name is already in use.")
		}
	}

	organization := portals.Organization{ID: t.newID("org"), Name: spec.Name, DisplayName: spec.DisplayName}
	t.organizations = append(t.organizations, organization)

	return organization, nil
}

// ListOrganizationConnections implements portals.OrganizationStore.
func (t *Tenant) ListOrganizationConnections(_ context.Context, id string) ([]string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("GET organizations/" + id + "/enabled_connections"); err != nil {
		return nil, err
	}

	return slices.Clone(t.orgConnections[id]), nil
}

// EnableOrganizationConnection implements portals.OrganizationStore.
func (t *Tenant) EnableOrganizationConnection(_ context.Context, id, connectionID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("POST organizations/" + id + "/enabled_connections"); err != nil {
		return err
	}

	t.orgConnections[id] = append(t.orgConnections[id], connectionID)

	return nil
}

// AddOrganizationMember implements portals.OrganizationStore.
func (t *Tenant) AddOrganizationMember(_ context.Context, id, userID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("POST organizations/" + id + "/members"); err != nil {
		return err
	}

	if !slices.Contains(t.members[id], userID) {
		t.members[id] = append(t.members[id], userID)
	}

	return nil
}

// ListMemberRoles implements portals.OrganizationStore.
func (t *Tenant) ListMemberRoles(_ context.Context, id, userID string) ([]portals.Role, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("GET organizations/" + id + "/members/" + userID + "/roles"); err != nil {
		return nil, err
	}

	var assigned []portals.Role
	for _, roleID := range t.orgRoles[id+":"+userID] {
		if index := slices.IndexFunc(t.roles, func(role roleRecord) bool { return role.ID == roleID }); index >= 0 {
			assigned = append(assigned, t.roles[index].Role)
		}
	}

	return assigned, nil
}

// AssignMemberRole implements portals.OrganizationStore.
func (t *Tenant) AssignMemberRole(_ context.Context, id, userID, roleID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("POST organizations/" + id + "/members/" + userID + "/roles"); err != nil {
		return err
	}

	key := id + ":" + userID
	t.orgRoles[key] = append(t.orgRoles[key], roleID)

	return nil
}

// ListRoles implements portals.RoleStore.
func (t *Tenant) ListRoles(_ context.Context) ([]portals.Role, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("GET roles"); err != nil {
		return nil, err
	}

	listed := make([]portals.Role, 0, len(t.roles))
	for _, role := range t.roles {
		listed = append(listed, role.Role)
	}

	return listed, nil
}

// CreateRole implements portals.RoleStore.
func (t *Tenant) CreateRole(_ context.Context, spec portals.RoleSpec) (portals.Role, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("POST roles"); err != nil {
		return portals.Role{}, err
	}

	role := roleRecord{Role: portals.Role{ID: t.newID("rol"), Name: spec.Name}}
	t.roles = append(t.roles, role)

	return role.Role, nil
}

// ListRolePermissions implements portals.RoleStore.
func (t *Tenant) ListRolePermissions(_ context.Context, roleID string) ([]portals.Permission, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("GET roles/" + roleID + "/permissions"); err != nil {
		return nil, err
	}

	index := slices.IndexFunc(t.roles, func(role roleRecord) bool { return role.ID == roleID })
	if index < 0 {
		return nil, nil
	}

	return slices.Clone(t.roles[index].permissions), nil
}

// AddRolePermissions implements portals.RoleStore.
func (t *Tenant) AddRolePermissions(_ context.Context, roleID string, permissions []portals.Permission) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("POST roles/" + roleID + "/permissions"); err != nil {
		return err
	}

	index := slices.IndexFunc(t.roles, func(role roleRecord) bool { return role.ID == roleID })
	if index < 0 {
		return notFound("The role does not exist")
	}

	t.roles[index].permissions = append(t.roles[index].permissions, permissions...)

	return nil
}

// ListDatabaseConnections implements portals.ConnectionStore.
func (t *Tenant) ListDatabaseConnections(_ context.Context) ([]portals.Connection, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("GET connections"); err != nil {
		return nil, err
	}

	var listed []portals.Connection
	for _, connection := range t.connections {
		if connection.strategy == "auth0" {
			listed = append(listed, connection.Connection)
		}
	}

	return listed, nil
}

// ListClientConnections implements portals.ConnectionStore.
func (t *Tenant) ListClientConnections(_ context.Context, clientID string) ([]portals.Connection, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.begin("GET clients/" + clientID + "/connections"); err != nil {
		return nil, err
	}

	record, found := t.findClient(clientID)
	if !found {
		return nil, notFound("not found")
	}

	var listed []portals.Connection
	for _, connectionID := range record.enabledConnections {
		if index := slices.IndexFunc(t.connections, func(connection connectionRecord) bool { return connection.ID == connectionID }); index >= 0 {
			listed = append(listed, t.connections[index].Connection)
		}
	}

	return listed, nil
}

// Probe implements portals.Prober.
func (t *Tenant) Probe(_ context.Context, method, path string, body any) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := method + " " + strings.SplitN(path, "?", 2)[0]
	if err := t.record(key); err != nil {
		return err
	}

	// Like the real API, a create with an empty body is a validation error and not a success.
	// Preflight relies on this to tell a present scope from a missing one. The check comes before
	// the failures a test sets, so a probe never uses up the failure meant for the real request.
	if method == http.MethodPost && isEmptyBody(body) {
		return withCause(&portals.APIError{
			Status:  http.StatusBadRequest,
			Message: "Payload validation error: 'Missing required property'",
		})
	}

	if err := t.takeFailure(key); err != nil {
		return err
	}

	if method != http.MethodGet && strings.Contains(path, "preflight") {
		return notFound("The resource does not exist")
	}

	return nil
}

func isEmptyBody(body any) bool {
	if body == nil {
		return false
	}

	data, err := json.Marshal(body)

	return err == nil && string(data) == "{}"
}
