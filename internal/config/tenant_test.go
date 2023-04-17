package config

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zalando/go-keyring"

	"github.com/auth0/auth0-cli/internal/auth"
)

func TestTenant_HasAllRequiredScopes(t *testing.T) {
	t.Run("tenant has all required scopes", func(t *testing.T) {
		tenant := &Tenant{
			Scopes: auth.RequiredScopes,
		}

		assert.True(t, tenant.HasAllRequiredScopes())
	})

	t.Run("tenant does not have all required scopes", func(t *testing.T) {
		tenant := &Tenant{
			Scopes: []string{"read:clients"},
		}

		assert.False(t, tenant.HasAllRequiredScopes())
	})
}

func TestTenant_GetExtraRequestedScopes(t *testing.T) {
	t.Run("tenant has no extra requested scopes", func(t *testing.T) {
		tenant := &Tenant{
			Scopes: auth.RequiredScopes,
		}

		assert.Empty(t, tenant.GetExtraRequestedScopes())
	})

	t.Run("tenant has extra requested scopes", func(t *testing.T) {
		tenant := &Tenant{
			Scopes: []string{
				"create:organization_invitations",
				"read:organization_invitations",
				"delete:organization_invitations",
			},
		}

		expected := []string{
			"create:organization_invitations",
			"read:organization_invitations",
			"delete:organization_invitations",
		}

		assert.ElementsMatch(t, expected, tenant.GetExtraRequestedScopes())
	})
}

func TestTenant_IsAuthenticatedWithClientCredentials(t *testing.T) {
	t.Run("tenant is authenticated with client credentials", func(t *testing.T) {
		tenant := &Tenant{
			ClientID: "test-client-id",
		}

		assert.True(t, tenant.IsAuthenticatedWithClientCredentials())
	})

	t.Run("tenant is not authenticated with client credentials", func(t *testing.T) {
		tenant := &Tenant{}

		assert.False(t, tenant.IsAuthenticatedWithClientCredentials())
	})
}

func TestTenant_IsAuthenticatedWithDeviceCodeFlow(t *testing.T) {
	t.Run("tenant is authenticated with device code flow", func(t *testing.T) {
		tenant := &Tenant{}

		assert.True(t, tenant.IsAuthenticatedWithDeviceCodeFlow())
	})

	t.Run("tenant is not authenticated with device code flow", func(t *testing.T) {
		tenant := &Tenant{
			ClientID: "test-client-id",
		}

		assert.False(t, tenant.IsAuthenticatedWithDeviceCodeFlow())
	})
}

func TestTenant_HasExpiredToken(t *testing.T) {
	t.Run("token has not expired", func(t *testing.T) {
		tenant := &Tenant{
			ExpiresAt: time.Now().Add(10 * time.Minute),
		}

		assert.False(t, tenant.HasExpiredToken())
	})

	t.Run("token has expired", func(t *testing.T) {
		tenant := &Tenant{
			ExpiresAt: time.Now().Add(-10 * time.Minute),
		}

		assert.True(t, tenant.HasExpiredToken())
	})
}

func TestTenant_GetAccessToken(t *testing.T) {
	const testTenantName = "auth0-cli-test.us.auth0.com"
	expectedToken := "chunk0chunk1chunk2"

	keyring.MockInit()

	t.Run("token is retrieved from the keyring", func(t *testing.T) {
		const secretAccessToken = "Auth0 CLI Access Token"

		err := keyring.Set(fmt.Sprintf("%s %d", secretAccessToken, 0), testTenantName, "chunk0")
		assert.NoError(t, err)
		err = keyring.Set(fmt.Sprintf("%s %d", secretAccessToken, 1), testTenantName, "chunk1")
		assert.NoError(t, err)
		err = keyring.Set(fmt.Sprintf("%s %d", secretAccessToken, 2), testTenantName, "chunk2")
		assert.NoError(t, err)

		tenant := &Tenant{
			Domain: testTenantName,
		}

		actualToken := tenant.GetAccessToken()

		assert.Equal(t, expectedToken, actualToken)
	})

	t.Run("token is retrieved from the config when not found in the keyring", func(t *testing.T) {
		tenant := &Tenant{
			Domain:      testTenantName,
			AccessToken: "chunk0chunk1chunk2",
		}

		actualToken := tenant.GetAccessToken()

		assert.Equal(t, expectedToken, actualToken)
	})
}
