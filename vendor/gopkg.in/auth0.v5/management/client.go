package management

import (
	"encoding/json"
	"fmt"
	"strconv"

	"gopkg.in/auth0.v5"
)

type Client struct {
	// The name of the client
	Name *string `json:"name,omitempty"`

	// Free text description of the purpose of the Client. (Max character length
	// is 140)
	Description *string `json:"description,omitempty"`

	// The id of the client
	ClientID *string `json:"client_id,omitempty"`

	// The client secret, it must not be public
	ClientSecret *string `json:"client_secret,omitempty"`

	// The type of application this client represents
	AppType *string `json:"app_type,omitempty"`

	// The URL of the client logo (recommended size: 150x150)
	LogoURI *string `json:"logo_uri,omitempty"`

	// Whether this client a first party client or not
	IsFirstParty *bool `json:"is_first_party,omitempty"`

	// Set header `auth0-forwarded-for` as trusted to be used as source
	// of end user ip for brute-force-protection on token endpoint.
	IsTokenEndpointIPHeaderTrusted *bool `json:"is_token_endpoint_ip_header_trusted,omitempty"`

	// Whether this client will conform to strict OIDC specifications
	OIDCConformant *bool `json:"oidc_conformant,omitempty"`

	// The URLs that Auth0 can use to as a callback for the client
	Callbacks      []interface{} `json:"callbacks,omitempty"`
	AllowedOrigins []interface{} `json:"allowed_origins,omitempty"`

	// A set of URLs that represents valid web origins for use with web message
	// response mode
	WebOrigins        []interface{}           `json:"web_origins,omitempty"`
	ClientAliases     []interface{}           `json:"client_aliases,omitempty"`
	AllowedClients    []interface{}           `json:"allowed_clients,omitempty"`
	AllowedLogoutURLs []interface{}           `json:"allowed_logout_urls,omitempty"`
	JWTConfiguration  *ClientJWTConfiguration `json:"jwt_configuration,omitempty"`

	// Client signing keys
	SigningKeys   []map[string]string `json:"signing_keys,omitempty"`
	EncryptionKey map[string]string   `json:"encryption_key,omitempty"`
	SSO           *bool               `json:"sso,omitempty"`

	// True to disable Single Sign On, false otherwise (default: false)
	SSODisabled *bool `json:"sso_disabled,omitempty"`

	// True if this client can be used to make cross-origin authentication
	// requests, false otherwise (default: false)
	CrossOriginAuth *bool `json:"cross_origin_auth,omitempty"`

	// List of acceptable Grant Types for this Client
	GrantTypes []interface{} `json:"grant_types,omitempty"`

	// URL for the location in your site where the cross origin verification
	// takes place for the cross-origin auth flow when performing Auth in your
	// own domain instead of Auth0 hosted login page
	CrossOriginLocation *string `json:"cross_origin_loc,omitempty"`

	// True if the custom login page is to be used, false otherwise. Defaults to
	// true
	CustomLoginPageOn      *bool                  `json:"custom_login_page_on,omitempty"`
	CustomLoginPage        *string                `json:"custom_login_page,omitempty"`
	CustomLoginPagePreview *string                `json:"custom_login_page_preview,omitempty"`
	FormTemplate           *string                `json:"form_template,omitempty"`
	Addons                 map[string]interface{} `json:"addons,omitempty"`

	// Defines the requested authentication method for the token endpoint.
	// Possible values are:
	// 	'none' (public client without a client secret),
	// 	'client_secret_post' (client uses HTTP POST parameters) or
	// 	'client_secret_basic' (client uses HTTP Basic)
	TokenEndpointAuthMethod *string                `json:"token_endpoint_auth_method,omitempty"`
	ClientMetadata          map[string]string      `json:"client_metadata,omitempty"`
	Mobile                  map[string]interface{} `json:"mobile,omitempty"`

	// Initiate login uri, must be https and cannot contain a fragment
	InitiateLoginURI *string `json:"initiate_login_uri,omitempty"`

	NativeSocialLogin *ClientNativeSocialLogin `json:"native_social_login,omitempty"`
	RefreshToken      *ClientRefreshToken      `json:"refresh_token,omitempty"`

	OrganizationUsage           *string `json:"organization_usage,omitempty"`
	OrganizationRequireBehavior *string `json:"organization_require_behavior,omitempty"`
}

type ClientJWTConfiguration struct {
	// The amount of seconds the JWT will be valid (affects exp claim)
	LifetimeInSeconds *int `json:"-"`

	// True if the client secret is base64 encoded, false otherwise. Defaults to
	// true
	SecretEncoded *bool `json:"secret_encoded,omitempty"`

	Scopes map[string]interface{} `json:"scopes,omitempty"`

	// Algorithm used to sign JWTs. Can be "HS256" or "RS256"
	Algorithm *string `json:"alg,omitempty"`
}

type ClientNativeSocialLogin struct {
	// Native Social Login support for the Apple connection
	Apple map[string]interface{} `json:"apple,omitempty"`

	// Native Social Login support for the Facebook connection
	Facebook map[string]interface{} `json:"facebook,omitempty"`
}

type ClientRefreshToken struct {
	// Refresh token types, one of: reusable, rotating
	//
	// Deprecated: use RotationType and ExpirationType instead
	Type *string `json:"type,omitempty"`

	// Refresh token rotation type. Can be "rotating" or "non-rotating"
	RotationType *string `json:"rotation_type,omitempty"`

	// Refresh token expiration type. Can be "expiring" or "non-expiring"
	ExpirationType *string `json:"expiration_type,omitempty"`

	// Period in seconds where the previous refresh token can be exchanged
	// without triggering breach detection
	Leeway *int `json:"leeway,omitempty"`

	// Period in seconds for which refresh tokens will remain valid
	TokenLifetime *int `json:"token_lifetime,omitempty"`

	// Whether or not refresh tokens should remain valid indefinitely. If false,
	// "TokenLifetime" should be set
	InfiniteTokenLifetime *bool `json:"infinite_token_lifetime,omitempty"`

	// Whether or not inactive refresh tokens should be remain valid indefinitely
	InfiniteIdleTokenLifetime *bool `json:"infinite_idle_token_lifetime,omitempty"`

	// Period in seconds after which inactive refresh tokens will expire
	IdleTokenLifetime *int `json:"idle_token_lifetime,omitempty"`
}

type ClientList struct {
	List
	Clients []*Client `json:"clients"`
}

type ClientManager struct {
	*Management
}

func newClientManager(m *Management) *ClientManager {
	return &ClientManager{m}
}

// Create a new client application.
//
// See: https://auth0.com/docs/api/management/v2#!/Clients/post_clients
func (m *ClientManager) Create(c *Client, opts ...RequestOption) (err error) {
	return m.Request("POST", m.URI("clients"), c, opts...)
}

// Read a client by its id.
//
// See: https://auth0.com/docs/api/management/v2#!/Clients/get_clients_by_id
func (m *ClientManager) Read(id string, opts ...RequestOption) (c *Client, err error) {
	err = m.Request("GET", m.URI("clients", id), &c, opts...)
	return
}

// List all client applications.
//
// See: https://auth0.com/docs/api/management/v2#!/Clients/get_clients
func (m *ClientManager) List(opts ...RequestOption) (c *ClientList, err error) {
	err = m.Request("GET", m.URI("clients"), &c, applyListDefaults(opts))
	return
}

// Update a client.
//
// See: https://auth0.com/docs/api/management/v2#!/Clients/patch_clients_by_id
func (m *ClientManager) Update(id string, c *Client, opts ...RequestOption) (err error) {
	return m.Request("PATCH", m.URI("clients", id), c, opts...)
}

// RotateSecret rotates a client secret.
//
// See: https://auth0.com/docs/api/management/v2#!/Clients/post_rotate_secret
func (m *ClientManager) RotateSecret(id string, opts ...RequestOption) (c *Client, err error) {
	err = m.Request("POST", m.URI("clients", id, "rotate-secret"), &c, opts...)
	return
}

// Delete a client and all its related assets (like rules, connections, etc)
// given its id.
//
// See: https://auth0.com/docs/api/management/v2#!/Clients/delete_clients_by_id
func (m *ClientManager) Delete(id string, opts ...RequestOption) error {
	return m.Request("DELETE", m.URI("clients", id), nil, opts...)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
//
// It is required to handle the json field lifetime_in_seconds, which can either
// be an int, or a string in older tenants.
func (jc *ClientJWTConfiguration) UnmarshalJSON(b []byte) error {
	type clientJWTConfiguration ClientJWTConfiguration
	type clientJWTConfigurationWrapper struct {
		*clientJWTConfiguration
		RawLifetimeInSeconds interface{} `json:"lifetime_in_seconds,omitempty"`
	}

	alias := &clientJWTConfigurationWrapper{(*clientJWTConfiguration)(jc), nil}

	err := json.Unmarshal(b, alias)
	if err != nil {
		return err
	}

	unexpectedTypeError := fmt.Errorf("unexpected type for field lifetime_in_seconds")

	if alias.RawLifetimeInSeconds != nil {
		switch rawLifetimeInSeconds := alias.RawLifetimeInSeconds.(type) {
		case float64:
			jc.LifetimeInSeconds = auth0.Int(int(rawLifetimeInSeconds))
		case string:
			value, err := strconv.Atoi(rawLifetimeInSeconds)
			if err != nil {
				return unexpectedTypeError
			}
			jc.LifetimeInSeconds = &value
		default:
			return unexpectedTypeError
		}
	}

	return nil
}

func (jc *ClientJWTConfiguration) MarshalJSON() ([]byte, error) {
	type clientJWTConfiguration ClientJWTConfiguration
	type clientJWTConfigurationWrapper struct {
		*clientJWTConfiguration
		RawLifetimeInSeconds interface{} `json:"lifetime_in_seconds,omitempty"`
	}

	alias := &clientJWTConfigurationWrapper{(*clientJWTConfiguration)(jc), nil}
	if jc.LifetimeInSeconds != nil {
		alias.RawLifetimeInSeconds = jc.LifetimeInSeconds
	}

	return json.Marshal(alias)
}
