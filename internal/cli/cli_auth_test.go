package cli

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/auth0/auth0-cli/internal/auth"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/keyring"
)

const authTestDomain = "tenant.us.auth0.com"

// tokenEndpoint is a stand-in for the token endpoint of authTestDomain.
type tokenEndpoint struct {
	// Ctx makes the token requests of the code under test reach the server.
	ctx context.Context
	// Calls counts the token requests.
	calls atomic.Int32
}

// newTokenEndpoint starts a TLS server that answers token requests with the given
// status, and routes every host name of the context to it.
func newTokenEndpoint(t *testing.T, status int) *tokenEndpoint {
	t.Helper()

	endpoint := &tokenEndpoint{}

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		endpoint.calls.Add(1)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status != http.StatusOK {
			_, _ = w.Write([]byte(`{"error":"access_denied"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": poppySessionToken, "token_type": "Bearer", "expires_in": 3600})
	}))
	t.Cleanup(srv.Close)

	transport := srv.Client().Transport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
	}
	// The certificate of the test server is for example.com.
	transport.TLSClientConfig.ServerName = "example.com"

	endpoint.ctx = context.WithValue(t.Context(), oauth2.HTTPClient, &http.Client{Transport: transport})

	return endpoint
}

// expiredClientCredentialsLogin saves a client-credentials login whose token has
// expired, with its client secret in the keyring.
func expiredClientCredentialsLogin(t *testing.T) {
	t.Helper()

	saveSession(t, authTestDomain, map[string]any{authTestDomain: map[string]any{
		"domain":       authTestDomain,
		"client_id":    "client",
		"access_token": poppySessionToken,
		"scopes":       auth.RequiredScopes,
		"expires_at":   time.Now().Add(-time.Hour).Format(time.RFC3339),
	}})

	require.NoError(t, keyring.StoreClientSecret(authTestDomain, "secret"))
}

// savedExpiry reads the expiry of the saved login from the config file.
func savedExpiry(t *testing.T) time.Time {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".config", "auth0", "config.json"))
	require.NoError(t, err)

	var saved struct {
		Tenants map[string]struct {
			ExpiresAt time.Time `json:"expires_at"`
		} `json:"tenants"`
	}
	require.NoError(t, json.Unmarshal(data, &saved))

	return saved.Tenants[authTestDomain].ExpiresAt
}

func TestSetupWithAuthenticationRefreshes(t *testing.T) {
	t.Run("an expired client-credentials login gets a new token", func(t *testing.T) {
		expiredClientCredentialsLogin(t)
		endpoint := newTokenEndpoint(t, http.StatusOK)

		testCLI := &cli{renderer: display.NewRenderer()}
		require.NoError(t, testCLI.setupWithAuthentication(endpoint.ctx))

		assert.EqualValues(t, 1, endpoint.calls.Load())
		assert.NotNil(t, testCLI.api)
		assert.NotNil(t, testCLI.apiv3)
		assert.True(t, savedExpiry(t).After(time.Now()), "the new expiry is saved")
	})

	t.Run("a refused refresh is a credentials error", func(t *testing.T) {
		expiredClientCredentialsLogin(t)
		endpoint := newTokenEndpoint(t, http.StatusUnauthorized)

		testCLI := &cli{renderer: display.NewRenderer()}
		err := testCLI.setupWithAuthentication(endpoint.ctx)

		require.Error(t, err)
		assert.Equal(t, "credentials_refresh_failed", errorReason(err))
		assert.Equal(t, "auth", errorClass(err))
		assert.Contains(t, err.Error(), "failed to fetch access token using client credentials")
		assert.Nil(t, testCLI.apiv3)
	})

	t.Run("a valid login is not refreshed", func(t *testing.T) {
		saveSession(t, authTestDomain, map[string]any{authTestDomain: map[string]any{
			"domain":       authTestDomain,
			"client_id":    "client",
			"access_token": poppySessionToken,
			"scopes":       auth.RequiredScopes,
			"expires_at":   time.Now().Add(time.Hour).Format(time.RFC3339),
		}})
		endpoint := newTokenEndpoint(t, http.StatusOK)

		testCLI := &cli{renderer: display.NewRenderer()}
		require.NoError(t, testCLI.setupWithAuthentication(endpoint.ctx))

		assert.Zero(t, endpoint.calls.Load())
		assert.NotNil(t, testCLI.apiv3)
	})

	t.Run("an expired device login without a terminal asks for a login", func(t *testing.T) {
		saveSession(t, authTestDomain, map[string]any{authTestDomain: map[string]any{
			"domain":       authTestDomain,
			"access_token": poppySessionToken,
			"scopes":       auth.RequiredScopes,
			"expires_at":   time.Now().Add(-time.Hour).Format(time.RFC3339),
		}})
		endpoint := newTokenEndpoint(t, http.StatusOK)

		testCLI := &cli{renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: io.Discard}, noInput: true}
		err := testCLI.setupWithAuthentication(endpoint.ctx)

		require.Error(t, err)
		assert.Equal(t, "session_expired", errorReason(err))
		assert.Zero(t, endpoint.calls.Load(), "a device login is never refreshed without a person")
		assert.Nil(t, testCLI.apiv3)
	})
}

func TestSetupPoppyAuthentication(t *testing.T) {
	t.Run("an expired client-credentials login is refreshed", func(t *testing.T) {
		expiredClientCredentialsLogin(t)
		endpoint := newTokenEndpoint(t, http.StatusOK)

		testCLI := &cli{renderer: display.NewRenderer()}
		require.NoError(t, testCLI.setupPoppyAuthentication(endpoint.ctx))

		assert.EqualValues(t, 1, endpoint.calls.Load())
		assert.Equal(t, authTestDomain, testCLI.tenant)
		assert.NotNil(t, testCLI.apiv3)
		assert.True(t, savedExpiry(t).After(time.Now()), "the new expiry is saved, so the next run does not refresh")
	})

	t.Run("a refresh that fails leaves no session", func(t *testing.T) {
		expiredClientCredentialsLogin(t)
		endpoint := newTokenEndpoint(t, http.StatusUnauthorized)

		testCLI := &cli{renderer: display.NewRenderer()}
		err := testCLI.setupPoppyAuthentication(endpoint.ctx)

		require.ErrorIs(t, err, errNoPoppySession)
		assert.Positive(t, endpoint.calls.Load(), "the token endpoint was asked")
		assert.Nil(t, testCLI.apiv3, "no client is built")
	})

	t.Run("a refresh without a stored secret leaves no session", func(t *testing.T) {
		expiredClientCredentialsLogin(t)
		require.NoError(t, keyring.DeleteSecretsForTenant(authTestDomain))
		endpoint := newTokenEndpoint(t, http.StatusOK)

		testCLI := &cli{renderer: display.NewRenderer()}

		require.ErrorIs(t, testCLI.setupPoppyAuthentication(endpoint.ctx), errNoPoppySession)
		assert.Zero(t, endpoint.calls.Load())
	})

	t.Run("an expired device login is not refreshed", func(t *testing.T) {
		saveSession(t, authTestDomain, map[string]any{authTestDomain: map[string]any{
			"domain":       authTestDomain,
			"access_token": poppySessionToken,
			"scopes":       auth.RequiredScopes,
			"expires_at":   time.Now().Add(-time.Hour).Format(time.RFC3339),
		}})
		endpoint := newTokenEndpoint(t, http.StatusOK)

		testCLI := &cli{renderer: display.NewRenderer()}

		require.ErrorIs(t, testCLI.setupPoppyAuthentication(endpoint.ctx), errNoPoppySession)
		assert.Zero(t, endpoint.calls.Load())
	})
}
