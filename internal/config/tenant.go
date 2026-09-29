package config

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwt"

	"github.com/auth0/auth0-cli/internal/auth"
	"github.com/auth0/auth0-cli/internal/keyring"
)

const accessTokenExpThreshold = 5 * time.Minute

var (
	// ErrInvalidToken is thrown when the token is invalid.
	ErrInvalidToken = errors.New("token is invalid")
	// ErrMalformedToken indicates a corrupted JWT token was found in keyring.
	ErrMalformedToken = errors.New("corrupted authentication token detected")
	// ErrStoredTokenUnavailable indicates the login still looks live (its config
	// expiry is in the future) but the stored access token could not be read from
	// the keyring, for example because the OS keychain is locked or inaccessible.
	// This is distinct from an expired session: the token has not expired, it just
	// cannot be retrieved, so the caller should retry with keychain access rather
	// than assume the session ended.
	ErrStoredTokenUnavailable = errors.New("stored access token is unavailable")
)

type ErrTokenMissingRequiredScopes struct {
	MissingScopes []string
}

func (e ErrTokenMissingRequiredScopes) Error() string {
	return "token is missing required scopes"
}

type (
	// Tenants keeps track of all the tenants we
	// logged into. The key is the tenant domain.
	Tenants map[string]Tenant

	// Tenant keeps track of auth0 config for the tenant.
	Tenant struct {
		Name         string    `json:"name"`
		Domain       string    `json:"domain"`
		AccessToken  string    `json:"access_token,omitempty"`
		Scopes       []string  `json:"scopes,omitempty"`
		ExpiresAt    time.Time `json:"expires_at"`
		DefaultAppID string    `json:"default_app_id,omitempty"`
		ClientID     string    `json:"client_id"`
	}
)

// GetMissingRequiredScopes returns a slice of required scopes
// that are missing from the tenant's current scopes.
func (t *Tenant) GetMissingRequiredScopes() []string {
	var missingScopes []string
	for _, requiredScope := range auth.RequiredScopes {
		if !slices.Contains(t.Scopes, requiredScope) {
			missingScopes = append(missingScopes, requiredScope)
		}
	}

	return missingScopes
}

// GetExtraRequestedScopes retrieves any extra scopes requested
// for the tenant when logging in through the device code flow.
func (t *Tenant) GetExtraRequestedScopes() []string {
	additionallyRequestedScopes := make([]string, 0)

	for _, scope := range t.Scopes {
		found := false

		for _, defaultScope := range auth.RequiredScopes {
			if scope == defaultScope {
				found = true
				break
			}
		}

		if !found {
			additionallyRequestedScopes = append(additionallyRequestedScopes, scope)
		}
	}

	return additionallyRequestedScopes
}

// IsAuthenticatedWithClientCredentials checks to see if the
// tenant has been authenticated through client credentials.
func (t *Tenant) IsAuthenticatedWithClientCredentials() bool {
	return t.ClientID != ""
}

// IsAuthenticatedWithDeviceCodeFlow checks to see if the
// tenant has been authenticated through device code flow.
func (t *Tenant) IsAuthenticatedWithDeviceCodeFlow() bool {
	return t.ClientID == ""
}

// HasExpiredToken checks whether the tenant has an expired token.
func (t *Tenant) HasExpiredToken() bool {
	return time.Now().Add(accessTokenExpThreshold).After(t.ExpiresAt)
}

// resolveAccessToken retrieves the tenant's access token and reports any keyring
// read error. The token is first read from the keyring; when that yields an empty
// value it falls back to the legacy inline token persisted in config.json. An
// empty token together with a non-nil keyringErr means the stored token could not
// be read (for example the keychain is locked or inaccessible), which is distinct
// from a tenant that simply has no token stored.
func (t *Tenant) resolveAccessToken() (accessToken string, keyringErr error) {
	accessToken, keyringErr = keyring.GetAccessToken(t.Domain)
	if keyringErr == nil && accessToken != "" {
		return accessToken, nil
	}

	if t.AccessToken != "" {
		return t.AccessToken, nil
	}

	return "", keyringErr
}

// GetAccessToken retrieves the tenant's access token.
func (t *Tenant) GetAccessToken() string {
	accessToken, _ := t.resolveAccessToken()
	return accessToken
}

// CheckAuthenticationStatus checks to see if the tenant in the config has all
// the required scopes and that the access token is not expired. On success it
// returns the validated access token it read, so the caller can hand that exact
// token to the SDK without reading the keyring a second time; a second read
// could fail differently (an intermittently locked keychain) and change the
// result after validation already passed. On any error the returned token is
// empty.
func (t *Tenant) CheckAuthenticationStatus() (string, error) {
	if missingScopes := t.GetMissingRequiredScopes(); len(missingScopes) > 0 && t.IsAuthenticatedWithDeviceCodeFlow() {
		return "", ErrTokenMissingRequiredScopes{MissingScopes: missingScopes}
	}

	accessToken, keyringErr := t.resolveAccessToken()
	if accessToken == "" {
		// The login still looks live (expiry in the future) but no token could be
		// read from the keyring: report it as unavailable rather than expired, so
		// the caller can suggest retrying with keychain access instead of a
		// re-login. This only applies to user (device-code) logins; client-
		// credential tenants recover by regenerating the token from the stored
		// client secret, so they keep the existing invalid-token handling.
		if t.IsAuthenticatedWithDeviceCodeFlow() && keyringErr != nil && !t.HasExpiredToken() {
			return "", ErrStoredTokenUnavailable
		}

		return "", ErrInvalidToken
	}

	if t.HasExpiredToken() {
		return "", ErrInvalidToken
	}

	// Validate that the access token is a well-formed JWT token.
	if _, err := jwt.ParseInsecure([]byte(accessToken)); err != nil {
		return "", ErrMalformedToken
	}

	return accessToken, nil
}

// RegenerateAccessToken regenerates the access token for the tenant.
func (t *Tenant) RegenerateAccessToken(ctx context.Context) error {
	clientSecret, err := keyring.GetClientSecret(t.Domain)
	if err != nil {
		return fmt.Errorf("failed to retrieve client secret from keyring: %w", err)
	}

	token, err := auth.GetAccessTokenFromClientCreds(
		ctx,
		auth.ClientCredentials{
			ClientID:     t.ClientID,
			ClientSecret: clientSecret,
			Domain:       t.Domain,
		},
	)
	if err != nil {
		return err
	}

	t.AccessToken = token.AccessToken
	t.ExpiresAt = token.ExpiresAt

	if err := keyring.StoreAccessToken(t.Domain, t.AccessToken); err == nil {
		t.AccessToken = ""
	}

	return nil
}
