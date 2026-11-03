package config

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zalando/go-keyring"

	"github.com/auth0/auth0-cli/internal/auth"
)

func TestTenant_GetExtraRequestedScopes(t *testing.T) {
	t.Run("tenant has no extra requested scopes", func(t *testing.T) {
		tenant := &Tenant{
			Scopes: auth.RequiredScopes,
		}

		assert.Empty(t, tenant.GetExtraRequestedScopes())
	})

	t.Run("tenant has extra requested scopes", func(t *testing.T) {
		extraScopes := []string{
			"create:extra_scope1",
			"read:extra_scope2",
			"delete:extra_scope3",
		}

		tenant := &Tenant{
			Scopes: extraScopes,
		}

		assert.ElementsMatch(t, extraScopes, tenant.GetExtraRequestedScopes())
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

func TestTenant_CheckAuthenticationStatus(t *testing.T) {
	// The mock keyring holds no access tokens, so every keyring read below returns
	// ErrNotFound. That mirrors both a keychain that is locked/inaccessible and one
	// with no stored token, which is exactly the ambiguity CheckAuthenticationStatus
	// resolves using the config expiry.
	keyring.MockInit()

	var testCases = []struct {
		name          string
		givenTenant   Tenant
		expectedError string
	}{
		{
			name: "it throws an error when required scopes are missing",
			givenTenant: Tenant{
				Scopes:   []string{"read:magazines"},
				ClientID: "",
			},
			expectedError: "token is missing required scopes",
		},
		{
			name: "it throws an error when the token is empty",
			givenTenant: Tenant{
				AccessToken: "",
				ClientID:    "123",
			},
			expectedError: "token is invalid",
		},
		{
			name: "it throws an error when the token is expired and we are authenticated through client credentials",
			givenTenant: Tenant{
				ExpiresAt: time.Now().Add(-time.Minute),
				ClientID:  "123",
			},
			expectedError: "token is invalid",
		},
		{
			name: "it reports the stored token as unavailable for a user login whose expiry is still in the future",
			givenTenant: Tenant{
				Domain:    "unavailable-token.us.auth0.com",
				Scopes:    auth.RequiredScopes,
				ExpiresAt: time.Now().Add(10 * time.Minute),
			},
			expectedError: "stored access token is unavailable",
		},
		{
			name: "it reports an expired user login as invalid rather than unavailable",
			givenTenant: Tenant{
				Domain:    "expired-token.us.auth0.com",
				Scopes:    auth.RequiredScopes,
				ExpiresAt: time.Now().Add(-time.Minute),
			},
			expectedError: "token is invalid",
		},
		{
			name: "it does not report client-credential tenants as unavailable so they can regenerate",
			givenTenant: Tenant{
				Domain:    "client-creds.us.auth0.com",
				ClientID:  "123",
				ExpiresAt: time.Now().Add(10 * time.Minute),
			},
			expectedError: "token is invalid",
		},
		{
			name: "it accepts a valid inline token when the keyring has none",
			givenTenant: Tenant{
				Domain:      "legacy-inline.us.auth0.com",
				AccessToken: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJodHRwczovL2F1dGgwLmF1dGgwLmNvbS8iLCJpYXQiOjE2ODExNDcwNjAsImV4cCI6OTY4MTgzMzQ2MH0.DsEpQkL0MIWcGJOIfEY8vr3MVS_E0GYsachNLQwBu5Q",
				ExpiresAt:   time.Now().Add(10 * time.Minute),
				ClientID:    "123",
			},
		},
		{
			name: "tenant has a valid token",
			givenTenant: Tenant{
				AccessToken: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJodHRwczovL2F1dGgwLmF1dGgwLmNvbS8iLCJpYXQiOjE2ODExNDcwNjAsImV4cCI6OTY4MTgzMzQ2MH0.DsEpQkL0MIWcGJOIfEY8vr3MVS_E0GYsachNLQwBu5Q",
				ExpiresAt:   time.Now().Add(10 * time.Minute),
				ClientID:    "123",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			accessToken, err := testCase.givenTenant.CheckAuthenticationStatus()
			if testCase.expectedError != "" {
				assert.EqualError(t, err, testCase.expectedError)
				assert.Empty(t, accessToken)
				return
			}
			assert.NoError(t, err)
			// On success the validated token is returned so the caller can reuse it
			// without a second keyring read.
			assert.Equal(t, testCase.givenTenant.GetAccessToken(), accessToken)
			assert.NotEmpty(t, accessToken)
		})
	}
}
