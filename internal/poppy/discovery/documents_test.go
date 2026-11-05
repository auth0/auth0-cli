package discovery

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecode(t *testing.T) {
	t.Run("poppy.json ignores unknown fields", func(t *testing.T) {
		doc, err := Decode[Poppy]([]byte(`{
			"protocol_version": "0.3",
			"organization": {"name": "Shop", "domain": "shop.example", "logo_uri": "https://shop.example/logo.png", "future": 1},
			"auth": {
				"disabled": true,
				"issuer": "https://auth.shop.example/",
				"scopes": {"orders:read": "See your orders"},
				"scope_hints": {"orders:read": {"changes": "none", "personal_data": true}}
			},
			"agent": {"protocols": [{"type": "poppy", "endpoint": "https://api.shop.example/poppy"}]},
			"web": {"browser_session_endpoint": "https://shop.example/poppy/browser-session"},
			"feedback": {"endpoint": "https://api.shop.example/feedback"},
			"excluded_tasks": {"url": "https://shop.example/excluded.json"},
			"apis": [{"type": "openapi", "url": "https://api.shop.example/openapi.json", "description": "Orders"}],
			"extensions": {"x": 1},
			"skill_index": "https://shop.example/skills"
		}`))

		require.NoError(t, err)
		assert.Equal(t, "0.3", doc.ProtocolVersion)
		assert.Equal(t, "shop.example", doc.Organization.Domain)
		assert.Equal(t, "https://shop.example/logo.png", doc.Organization.LogoURI)
		assert.True(t, doc.Auth.Disabled)
		assert.Equal(t, "https://auth.shop.example/", doc.Auth.Issuer)
		assert.Equal(t, map[string]string{"orders:read": "See your orders"}, doc.Auth.Scopes)
		assert.Equal(t, map[string]any{"changes": "none", "personal_data": true}, doc.Auth.ScopeHints["orders:read"])
		require.Len(t, doc.Agent.Protocols, 1)
		assert.Equal(t, "https://api.shop.example/poppy", doc.Agent.Protocols[0].Endpoint)
		assert.Equal(t, "https://shop.example/poppy/browser-session", doc.Web.BrowserSessionEndpoint)
		assert.Equal(t, "https://api.shop.example/feedback", doc.Feedback.Endpoint)
		assert.Equal(t, "https://shop.example/excluded.json", doc.ExcludedTasks.URL)
		assert.Equal(t, []API{{Type: "openapi", URL: "https://api.shop.example/openapi.json"}}, doc.APIs)
	})

	t.Run("protected resource", func(t *testing.T) {
		doc, err := Decode[ProtectedResource]([]byte(`{"resource":"https://shop.example","authorization_servers":["https://auth.shop.example/"],"bearer_methods_supported":["header"]}`))

		require.NoError(t, err)
		assert.Equal(t, "https://shop.example", doc.Resource)
		assert.Equal(t, []string{"https://auth.shop.example/"}, doc.AuthorizationServers)
	})

	t.Run("authorization server", func(t *testing.T) {
		doc, err := Decode[AuthorizationServer]([]byte(`{
			"issuer": "https://auth.shop.example/",
			"protected_resources": ["https://shop.example"],
			"authorization_endpoint": "https://auth.shop.example/authorize",
			"token_endpoint": "https://auth.shop.example/oauth/token",
			"revocation_endpoint": "https://auth.shop.example/oauth/revoke",
			"device_authorization_endpoint": "https://auth.shop.example/oauth/device/code",
			"scopes_supported": ["orders:read"],
			"grant_types_supported": ["authorization_code"],
			"token_endpoint_auth_signing_alg_values_supported": ["ES256"],
			"authorization_response_iss_parameter_supported": true,
			"jwks_uri": "https://auth.shop.example/jwks.json"
		}`))

		require.NoError(t, err)
		assert.Equal(t, "https://auth.shop.example/", doc.Issuer)
		assert.Equal(t, []string{"https://shop.example"}, doc.ProtectedResources)
		assert.Equal(t, "https://auth.shop.example/oauth/token", doc.TokenEndpoint)
		assert.Equal(t, "https://auth.shop.example/oauth/revoke", doc.RevocationEndpoint)
		assert.Equal(t, "https://auth.shop.example/authorize", doc.AuthorizationEndpoint)
		assert.Equal(t, "https://auth.shop.example/oauth/device/code", doc.DeviceAuthorizationEndpoint)
		assert.Equal(t, []string{"orders:read"}, doc.ScopesSupported)
		assert.Equal(t, []string{"authorization_code"}, doc.GrantTypesSupported)
		assert.Equal(t, []string{"ES256"}, doc.TokenEndpointAuthSigningAlgValuesSupported)
		assert.True(t, doc.AuthorizationResponseIssParameterSupported)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := Decode[Poppy]([]byte(`<html>`))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid JSON")
	})

	t.Run("wrong type", func(t *testing.T) {
		_, err := Decode[AuthorizationServer]([]byte(`{"authorization_response_iss_parameter_supported": "yes"}`))

		require.Error(t, err)
	})
}
