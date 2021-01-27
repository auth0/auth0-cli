package authutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// TokenResponse stores token information as retrieved from the /oauth/token
// endpoint when exchanging a code.
type TokenResponse struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	ExpiresIn    int64  `json:"expires_in,omitempty"`
}

// ExchangeCodeForToken fetches an access token for the given client using the provided code.
func ExchangeCodeForToken(baseDomain, clientID, clientSecret, code, cbURL string) (*TokenResponse, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"code":          {code},
		"redirect_uri":  {cbURL},
	}

	u := url.URL{Scheme: "https", Host: baseDomain, Path: "/oauth/token"}
	r, err := http.PostForm(u.String(), data)
	if err != nil {
		return nil, fmt.Errorf("unable to exchange code for token: %w", err)
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unable to exchange code for token: %s", r.Status)
	}

	var res *TokenResponse
	err = json.NewDecoder(r.Body).Decode(&res)
	if err != nil {
		return nil, fmt.Errorf("cannot decode response: %w", err)
	}

	return res, nil
}
