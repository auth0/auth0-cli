package portals

// ResourceServer is an API registered in the tenant.
type ResourceServer struct {
	ID         string
	Identifier string
	Name       string
}

// ResourceServerSpec describes a resource server to create.
type ResourceServerSpec struct {
	Name       string
	Identifier string
	// SkipConsentForVerifiableFirstPartyClients stops first-party applications from asking for consent.
	SkipConsentForVerifiableFirstPartyClients bool
	// TokenDialect is the token profile, for example "rfc9068_profile".
	TokenDialect string
}

// Client is an application. A list returns only the ID, name, metadata and session transfer,
// a read returns the whole configuration, and only a create returns the secret.
type Client struct {
	ID     string
	Secret string
	ClientSettings
}

// ClientSettings is the configuration of an application. When it updates an application, the
// fields left empty or nil are not sent, so they keep their value.
type ClientSettings struct {
	Name                        string
	AppType                     string
	IsFirstParty                *bool
	OIDCConformant              *bool
	TokenEndpointAuthMethod     string
	IPHeaderTrusted             *bool
	RequireProofOfPossession    *bool
	Callbacks                   []string
	AllowedLogoutURLs           []string
	GrantTypes                  []string
	OrganizationUsage           string
	OrganizationRequireBehavior string
	Metadata                    map[string]string
	RefreshToken                *RefreshToken
	SessionTransfer             *SessionTransfer
	OIDCLogout                  *OIDCLogout
}

// RefreshToken is the refresh token configuration of an application.
type RefreshToken struct {
	RotationType              string
	ExpirationType            string
	TokenLifetime             *int
	InfiniteTokenLifetime     *bool
	IdleTokenLifetime         *int
	InfiniteIdleTokenLifetime *bool
	Policies                  []RefreshTokenPolicy
}

// RefreshTokenPolicy limits what a refresh token can ask for on one API.
type RefreshTokenPolicy struct {
	Audience string
	Scope    []string
}

// SessionTransfer lets an application hand its session to the portal.
type SessionTransfer struct {
	AllowedAuthenticationMethods []string
	EnforceDeviceBinding         string
}

// OIDCLogout configures back-channel logout.
type OIDCLogout struct {
	BackchannelLogoutURLs []string
	// Mode is how the initiators are chosen, for example "custom".
	Mode               string
	SelectedInitiators []string
}

// ClientGrant lets an application call an API with some scopes.
type ClientGrant struct {
	ID       string
	ClientID string
	Audience string
	Scope    []string
	// SubjectType is "user" or "client", and empty when the API did not tell.
	SubjectType string
}

// forUsers reports whether the grant lets the users of the application call the API. A grant that does
// not tell its subject is given the benefit of the doubt.
func (g ClientGrant) forUsers() bool { return g.SubjectType == "" || g.SubjectType == "user" }

// ClientGrantSpec describes a client grant to create.
type ClientGrantSpec struct {
	ClientID string
	Audience string
	Scope    []string
	// SubjectType is "user" or "client"; empty leaves the API default.
	SubjectType string
}

// VaultConnection is a connection in the Flows vault.
type VaultConnection struct {
	ID   string
	Name string
}

// VaultSetup is how the vault signs in to the tenant.
type VaultSetup struct {
	Domain       string
	ClientID     string
	ClientSecret string
}

// Form is a Forms form in the list of the tenant.
type Form struct {
	ID   string
	Name string
}

// FormDetail is a form read on its own.
type FormDetail struct {
	ID string
	// FlowID is the flow the form runs, empty when it runs none.
	FlowID string
}

// ImportedForm is the result of importing a form.
type ImportedForm struct {
	ID string
	// FlowID is the flow the import created, empty when it created none.
	FlowID string
}

// CustomDomain is a custom domain of the tenant.
type CustomDomain struct {
	Domain  string
	Status  string
	Primary bool
}

// IsReady reports whether the portal is served from the domain: it is verified, and it has a name.
func (d CustomDomain) IsReady() bool { return d.Status == "ready" && d.Domain != "" }

// User is a user of the tenant.
type User struct {
	ID    string
	Email string
	Name  string
}

// Organization is an organization of the tenant.
type Organization struct {
	ID          string
	Name        string
	DisplayName string
}

// OrganizationSpec describes an organization to create.
type OrganizationSpec struct {
	Name        string
	DisplayName string
}

// Role is a role of the tenant.
type Role struct {
	ID   string
	Name string
}

// RoleSpec describes a role to create.
type RoleSpec struct {
	Name        string
	Description string
}

// Permission is a scope of an API, granted through a role.
type Permission struct {
	Name                     string
	ResourceServerIdentifier string
}

// Connection is a connection of the tenant.
type Connection struct {
	ID   string
	Name string
}
