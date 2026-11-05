package discovery

import (
	"encoding/json"
	"fmt"
)

// Poppy is the content of /.well-known/poppy.json that the checks read.
type Poppy struct {
	ProtocolVersion string `json:"protocol_version"`
	// SupportedVersions is the supported_versions member of protocol 0.4. It is
	// kept as decoded so that a check can tell a wrong type from a missing member.
	SupportedVersions any `json:"supported_versions"`
	Organization      struct {
		Domain  string `json:"domain"`
		LogoURI string `json:"logo_uri"`
	} `json:"organization"`
	Auth  Auth `json:"auth"`
	Agent struct {
		Protocols []struct {
			Endpoint string `json:"endpoint"`
		} `json:"protocols"`
	} `json:"agent"`
	Web struct {
		BrowserSessionEndpoint string `json:"browser_session_endpoint"`
		// Disabled and Unscoped are the web.disabled and web.unscoped members of
		// protocol 0.4, kept as decoded for the same reason as SupportedVersions.
		Disabled any `json:"disabled"`
		Unscoped any `json:"unscoped"`
	} `json:"web"`
	Feedback struct {
		Endpoint string `json:"endpoint"`
	} `json:"feedback"`
	ExcludedTasks struct {
		URL string `json:"url"`
	} `json:"excluded_tasks"`
	APIs []API `json:"apis"`
}

// Auth is the auth section of poppy.json.
type Auth struct {
	Disabled   bool                      `json:"disabled"`
	Issuer     string                    `json:"issuer"`
	Scopes     map[string]string         `json:"scopes"`
	ScopeHints map[string]map[string]any `json:"scope_hints"`
}

// API is an entry of the apis list of poppy.json.
type API struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// ProtectedResource is the content of /.well-known/oauth-protected-resource.
type ProtectedResource struct {
	Resource             string   `json:"resource"`
	AuthorizationServers []string `json:"authorization_servers"`
}

// AuthorizationServer is the authorization server metadata that the checks read.
type AuthorizationServer struct {
	Issuer                                     string   `json:"issuer"`
	ProtectedResources                         []string `json:"protected_resources"`
	AuthorizationEndpoint                      string   `json:"authorization_endpoint"`
	TokenEndpoint                              string   `json:"token_endpoint"`
	RevocationEndpoint                         string   `json:"revocation_endpoint"`
	DeviceAuthorizationEndpoint                string   `json:"device_authorization_endpoint"`
	ScopesSupported                            []string `json:"scopes_supported"`
	GrantTypesSupported                        []string `json:"grant_types_supported"`
	TokenEndpointAuthSigningAlgValuesSupported []string `json:"token_endpoint_auth_signing_alg_values_supported"`
	AuthorizationResponseIssParameterSupported bool     `json:"authorization_response_iss_parameter_supported"`
}

// Decode parses a JSON document into T. Fields that T does not declare are ignored.
func Decode[T any](body []byte) (*T, error) {
	var doc T
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	return &doc, nil
}
