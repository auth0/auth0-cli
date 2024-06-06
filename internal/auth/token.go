package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/auth0/auth0-cli/internal/keyring"
)

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// RefreshAccessToken retrieves a new access token using a refresh token.
// This occurs when the access token has expired or is otherwise removed/inaccessible.
// The request uses Auth0's dedicated public cloud client for token exchange.
// This process will not work for Private Cloud tenants.
func RefreshAccessToken(httpClient *http.Client, tenant string) (TokenResponse, error) {
	refreshToken, err := keyring.GetRefreshToken(tenant)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("failed to retrieve refresh token from keyring: %w", err)
	}
	if refreshToken == "" {
		return TokenResponse{}, errors.New("failed to use stored refresh token: the token is empty")
	}

	r, err := httpClient.PostForm(credentials.OauthTokenEndpoint, url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {credentials.ClientID},
		"refresh_token": {refreshToken},
	})
	if err != nil {
		return TokenResponse{}, fmt.Errorf("cannot get a new access token from the refresh token: %w", err)
	}

	defer func() {
		_ = r.Body.Close()
	}()

	if r.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(r.Body)
		bodyStr := string(b)
		return TokenResponse{}, fmt.Errorf("cannot get a new access token from the refresh token: %s", bodyStr)
	}

	var res TokenResponse
	err = json.NewDecoder(r.Body).Decode(&res)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("cannot decode response: %w", err)
	}

	return res, nil
}
