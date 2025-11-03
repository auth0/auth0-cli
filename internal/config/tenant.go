package config

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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

// GetAccessToken retrieves the tenant's access token.
func (t *Tenant) GetAccessToken() string {
	accessToken, err := keyring.GetAccessToken(t.Domain)
	if err == nil && accessToken != "" {
		return accessToken
	}

	return t.AccessToken
}

// CheckAuthenticationStatus checks to see if the tenant in the config
// has all the required scopes and that the access token is not expired.
func (t *Tenant) CheckAuthenticationStatus() error {
	if missingScopes := t.GetMissingRequiredScopes(); len(missingScopes) > 0 && t.IsAuthenticatedWithDeviceCodeFlow() {
		return ErrTokenMissingRequiredScopes{MissingScopes: missingScopes}
	}

	accessToken := t.GetAccessToken()
	if accessToken == "" || t.HasExpiredToken() {
		return ErrInvalidToken
	}

	// Validate that the access token is a well-formed JWT token.
	if _, err := jwt.ParseInsecure([]byte(accessToken)); err != nil {
		return ErrMalformedToken
	}

	return nil
}

// RegenerateAccessToken regenerates the access token for the tenant.
func (t *Tenant) RegenerateAccessToken(ctx context.Context) error {
	if t.IsAuthenticatedWithClientCredentials() {
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
	}

	if t.IsAuthenticatedWithDeviceCodeFlow() {
		tokenResponse, err := auth.RefreshAccessToken(http.DefaultClient, t.Domain)
		if err != nil {
			return err
		}

		t.AccessToken = tokenResponse.AccessToken
		t.ExpiresAt = time.Now().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second)
	}

	if err := keyring.StoreAccessToken(t.Domain, t.AccessToken); err == nil {
		t.AccessToken = ""
	}

	return nil
}
