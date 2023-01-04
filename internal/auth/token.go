package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/auth0/auth0-cli/internal/secret"
)

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type TokenRetriever struct {
	Authenticator *Authenticator
	Client        *http.Client
}

// Refresh gets a new access token from the provided refresh token,
// The request is used the default client_id and endpoint for device authentication.
func (t *TokenRetriever) Refresh(ctx context.Context, tenant string) (TokenResponse, error) {
	refreshToken, err := secret.GetRefreshToken(tenant)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("cannot get the stored refresh token: %w", err)
	}
	if refreshToken == "" {
		return TokenResponse{}, errors.New("cannot use the stored refresh token: the token is empty")
	}
	// get access token:
	r, err := t.Client.PostForm(t.Authenticator.OauthTokenEndpoint, url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {t.Authenticator.ClientID},
		"refresh_token": {refreshToken},
	})
	if err != nil {
		return TokenResponse{}, fmt.Errorf("cannot get a new access token from the refresh token: %w", err)
	}

	defer r.Body.Close()
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
