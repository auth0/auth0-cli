package management

type ResourceServer struct {

	// A generated string identifying the resource server.
	ID *string `json:"id,omitempty"`

	// The name of the resource server. Must contain at least one character.
	// Does not allow '<' or '>'
	Name *string `json:"name,omitempty"`

	// The identifier of the resource server.
	Identifier *string `json:"identifier,omitempty"`

	// Scopes supported by the resource server.
	Scopes []*ResourceServerScope `json:"scopes,omitempty"`

	// The algorithm used to sign tokens ["HS256" or "RS256"].
	SigningAlgorithm *string `json:"signing_alg,omitempty"`

	// The secret used to sign tokens when using symmetric algorithms.
	SigningSecret *string `json:"signing_secret,omitempty"`

	// Allows issuance of refresh tokens for this entity.
	AllowOfflineAccess *bool `json:"allow_offline_access,omitempty"`

	// The amount of time in seconds that the token will be valid after being
	// issued.
	TokenLifetime *int `json:"token_lifetime,omitempty"`

	// The amount of time in seconds that the token will be valid after being
	// issued from browser based flows. Value cannot be larger than
	// token_lifetime.
	TokenLifetimeForWeb *int `json:"token_lifetime_for_web,omitempty"`

	// Flag this entity as capable of skipping consent.
	SkipConsentForVerifiableFirstPartyClients *bool `json:"skip_consent_for_verifiable_first_party_clients,omitempty"`

	// A URI from which to retrieve JWKs for this resource server used for
	// verifying the JWT sent to Auth0 for token introspection.
	VerificationLocation *string `json:"verificationLocation,omitempty"`

	Options map[string]interface{} `json:"options,omitempty"`

	// Enables the enforcement of the authorization policies.
	EnforcePolicies *bool `json:"enforce_policies,omitempty"`

	// The dialect for the access token ["access_token" or "access_token_authz"].
	TokenDialect *string `json:"token_dialect,omitempty"`
}

type ResourceServerScope struct {
	// The scope name. Use the format <action>:<resource> for example
	// 'delete:client_grants'.
	Value *string `json:"value,omitempty"`

	// Description of the scope
	Description *string `json:"description,omitempty"`
}

type ResourceServerList struct {
	List
	ResourceServers []*ResourceServer `json:"resource_servers"`
}

type ResourceServerManager struct {
	*Management
}

func newResourceServerManager(m *Management) *ResourceServerManager {
	return &ResourceServerManager{m}
}

// Create a resource server.
//
// See: https://auth0.com/docs/api/management/v2#!/Resource_Servers/post_resource_servers
func (m *ResourceServerManager) Create(rs *ResourceServer, opts ...RequestOption) (err error) {
	return m.Request("POST", m.URI("resource-servers"), rs, opts...)
}

// Read retrieves a resource server by its id or audience.
//
// See: https://auth0.com/docs/api/management/v2#!/Resource_Servers/get_resource_servers_by_id
func (m *ResourceServerManager) Read(id string, opts ...RequestOption) (rs *ResourceServer, err error) {
	err = m.Request("GET", m.URI("resource-servers", id), &rs, opts...)
	return
}

// Update a resource server.
//
// See: https://auth0.com/docs/api/management/v2#!/Resource_Servers/patch_resource_servers_by_id
func (m *ResourceServerManager) Update(id string, rs *ResourceServer, opts ...RequestOption) (err error) {
	return m.Request("PATCH", m.URI("resource-servers", id), rs, opts...)
}

// Delete a resource server.
//
// See: https://auth0.com/docs/api/management/v2#!/Resource_Servers/delete_resource_servers_by_id
func (m *ResourceServerManager) Delete(id string, opts ...RequestOption) (err error) {
	return m.Request("DELETE", m.URI("resource-servers", id), nil, opts...)
}

// List all resource server.
//
// See: https://auth0.com/docs/api/management/v2#!/Resource_Servers/get_resource_servers
func (m *ResourceServerManager) List(opts ...RequestOption) (rl *ResourceServerList, err error) {
	err = m.Request("GET", m.URI("resource-servers"), &rl, applyListDefaults(opts))
	return
}

// Stream is a helper method which handles pagination
func (m *ResourceServerManager) Stream(fn func(s *ResourceServer), opts ...RequestOption) error {
	var page int
	for {
		l, err := m.List(append(opts, Page(page))...)
		if err != nil {
			return err
		}
		for _, s := range l.ResourceServers {
			fn(s)
		}
		if !l.HasNext() {
			break
		}
		page++
	}
	return nil
}
