package portals

import "context"

// The stores are the calls to the tenant that setup and audit make. Each method is one request
// to the Management API, so the order of the work stays in this package. A store returns an
// *APIError for a failed request, and the package decides what a status means: a 404 can be an
// answer, a 409 a race, and a 403 a missing scope.

// ResourceServerStore reads and creates resource servers.
type ResourceServerStore interface {
	ListResourceServers(ctx context.Context) ([]ResourceServer, error)
	CreateResourceServer(ctx context.Context, spec ResourceServerSpec) (ResourceServer, error)
}

// ClientStore reads and writes applications.
type ClientStore interface {
	// ListClients returns the ID, name, metadata and session transfer of every application.
	ListClients(ctx context.Context) ([]Client, error)
	// ReadClient returns the whole configuration of an application, without its secret.
	ReadClient(ctx context.Context, id string) (Client, error)
	// ReadClientSecret returns the secret of an application.
	ReadClientSecret(ctx context.Context, id string) (string, error)
	// CreateClient creates an application and returns it with its secret.
	CreateClient(ctx context.Context, settings ClientSettings) (Client, error)
	// UpdateClient changes the settings that are set, and keeps the others.
	UpdateClient(ctx context.Context, id string, settings ClientSettings) error
}

// ClientGrantStore reads and writes the grants of an application.
type ClientGrantStore interface {
	ListClientGrants(ctx context.Context, clientID string) ([]ClientGrant, error)
	CreateClientGrant(ctx context.Context, spec ClientGrantSpec) (ClientGrant, error)
	UpdateClientGrantScope(ctx context.Context, id string, scope []string) error
	DeleteClientGrant(ctx context.Context, id string) error
}

// VaultStore reads and writes the connections of the Flows vault.
type VaultStore interface {
	ListVaultConnections(ctx context.Context) ([]VaultConnection, error)
	CreateVaultConnection(ctx context.Context, name string, setup VaultSetup) (VaultConnection, error)
	// UpdateVaultConnection sends the setup again, so the vault signs in again with the grants in force.
	UpdateVaultConnection(ctx context.Context, id string, setup VaultSetup) error
}

// FormStore reads and imports forms.
type FormStore interface {
	ListForms(ctx context.Context) ([]Form, error)
	// ReadForm fails with a 404, or a 400 for an ID the API cannot match, when the form is missing.
	ReadForm(ctx context.Context, id string) (FormDetail, error)
	// ImportForm creates a form, and its flow, from a document.
	ImportForm(ctx context.Context, document any) (ImportedForm, error)
}

// PortalStore reads and writes portals.
type PortalStore interface {
	ListPortals(ctx context.Context) ([]PortalSummary, error)
	ReadPortal(ctx context.Context, id string) (Portal, error)
	CreatePortal(ctx context.Context, request CreatePortalRequest) (Portal, error)
	UpdatePortal(ctx context.Context, id string, patch PortalPatch) error
}

// CustomDomainStore reads the custom domains of the tenant.
type CustomDomainStore interface {
	// ReadDefaultCustomDomain fails with a 404 when the tenant has none.
	ReadDefaultCustomDomain(ctx context.Context) (CustomDomain, error)
	ListCustomDomains(ctx context.Context) ([]CustomDomain, error)
}

// UserStore reads users.
type UserStore interface {
	// SearchUsersByEmail returns the users whose email contains the text, like the search does.
	SearchUsersByEmail(ctx context.Context, email string) ([]User, error)
	// ReadUser fails with a 404 when the user does not exist.
	ReadUser(ctx context.Context, id string) (User, error)
	ListUserOrganizations(ctx context.Context, userID string) ([]Organization, error)
}

// OrganizationStore reads and writes organizations.
type OrganizationStore interface {
	ListOrganizations(ctx context.Context) ([]Organization, error)
	ReadOrganization(ctx context.Context, id string) (Organization, error)
	CreateOrganization(ctx context.Context, spec OrganizationSpec) (Organization, error)
	// ListOrganizationConnections returns the IDs of the connections enabled for the organization.
	ListOrganizationConnections(ctx context.Context, id string) ([]string, error)
	// EnableOrganizationConnection enables a connection without assigning membership on login.
	EnableOrganizationConnection(ctx context.Context, id, connectionID string) error
	AddOrganizationMember(ctx context.Context, id, userID string) error
	ListMemberRoles(ctx context.Context, id, userID string) ([]Role, error)
	AssignMemberRole(ctx context.Context, id, userID, roleID string) error
}

// RoleStore reads and writes roles.
type RoleStore interface {
	ListRoles(ctx context.Context) ([]Role, error)
	CreateRole(ctx context.Context, spec RoleSpec) (Role, error)
	ListRolePermissions(ctx context.Context, roleID string) ([]Permission, error)
	AddRolePermissions(ctx context.Context, roleID string, permissions []Permission) error
}

// ConnectionStore reads connections.
type ConnectionStore interface {
	// ListDatabaseConnections returns the connections that use the Auth0 database.
	ListDatabaseConnections(ctx context.Context) ([]Connection, error)
	// ListClientConnections returns the connections enabled for an application.
	ListClientConnections(ctx context.Context, clientID string) ([]Connection, error)
}

// Prober sends the requests that check a permission without changing anything.
type Prober interface {
	// Probe sends one request. A write probe carries a body the API rejects, or names a
	// resource that does not exist, so it can only fail.
	Probe(ctx context.Context, method, path string, body any) error
}

// API is everything setup and audit need from a tenant.
type API interface {
	ResourceServerStore
	ClientStore
	ClientGrantStore
	VaultStore
	FormStore
	PortalStore
	CustomDomainStore
	UserStore
	OrganizationStore
	RoleStore
	ConnectionStore
	Prober
}
