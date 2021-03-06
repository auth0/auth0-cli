package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

const (
	tokenEndpoint = "oauth/token"
)

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type TokenRetriever struct {
	Secrets SecretStore
	Client  *http.Client
}

func (t *TokenRetriever) Refresh(ctx context.Context, tenant string) (TokenResponse, error) {
	// get stored refresh token:
	refreshToken, err := t.Secrets.Get(secretsNamespace, tenant)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("cannot get the stored refresh token: %w", err)
	}
	if refreshToken == "" {
		return TokenResponse{}, errors.New("cannot use the stored refresh token: the token is empty")
	}

	// get access token:
	r, err := t.Client.PostForm(oauthTokenEndpoint, url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID},
		"refresh_token": {refreshToken},
	})
	if err != nil {
		return TokenResponse{}, fmt.Errorf("cannot get a new access token from the refresh token: %w", err)
	}

	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		b, _ := ioutil.ReadAll(r.Body)
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
