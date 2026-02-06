//go:generate mockgen -source auth.go -destination mock/auth.go -package mock
package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const (
	audiencePath           = "/api/v2/"
	waitThresholdInSeconds = 3
)

// Credentials is used to facilitate the login process.
type Credentials struct {
	Audience           string
	ClientID           string
	DeviceCodeEndpoint string
	OauthTokenEndpoint string
}

type Result struct {
	Tenant       string
	Domain       string
	RefreshToken string
	AccessToken  string
	ExpiresAt    time.Time
}

type State struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri_complete"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

func (s *State) IntervalDuration() time.Duration {
	return time.Duration(s.Interval+waitThresholdInSeconds) * time.Second
}

var credentials = &Credentials{
	Audience:           "https://*.auth0.com/api/v2/",
	ClientID:           "2iZo3Uczt5LFHacKdM0zzgUO2eG2uDjT",
	DeviceCodeEndpoint: "https://auth0.auth0.com/oauth/device/code",
	OauthTokenEndpoint: "https://auth0.auth0.com/oauth/token",
}

// WaitUntilUserLogsIn waits until the user is logged in on the browser.
func WaitUntilUserLogsIn(ctx context.Context, httpClient *http.Client, state State) (Result, error) {
	t := time.NewTicker(state.IntervalDuration())
	for {
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		case <-t.C:
			data := url.Values{
				"client_id":   []string{credentials.ClientID},
				"grant_type":  []string{"urn:ietf:params:oauth:grant-type:device_code"},
				"device_code": []string{state.DeviceCode},
			}
			r, err := httpClient.PostForm(credentials.OauthTokenEndpoint, data)
			if err != nil {
				return Result{}, fmt.Errorf("cannot get device code: %w", err)
			}
			defer func() {
				_ = r.Body.Close()
			}()

			var res struct {
				AccessToken      string  `json:"access_token"`
				IDToken          string  `json:"id_token"`
				RefreshToken     string  `json:"refresh_token"`
				Scope            string  `json:"scope"`
				ExpiresIn        int64   `json:"expires_in"`
				TokenType        string  `json:"token_type"`
				Error            *string `json:"error,omitempty"`
				ErrorDescription string  `json:"error_description,omitempty"`
			}

			err = json.NewDecoder(r.Body).Decode(&res)
			if err != nil {
				return Result{}, fmt.Errorf("cannot decode response: %w", err)
			}

			if res.Error != nil {
				if *res.Error == "authorization_pending" {
					continue
				}
				return Result{}, errors.New(res.ErrorDescription)
			}

			ten, domain, err := parseTenant(res.AccessToken)
			if err != nil {
				return Result{}, fmt.Errorf("cannot parse tenant from the given access token: %w", err)
			}

			return Result{
				RefreshToken: res.RefreshToken,
				AccessToken:  res.AccessToken,
				ExpiresAt: time.Now().Add(
					time.Duration(res.ExpiresIn) * time.Second,
				),
				Tenant: ten,
				Domain: domain,
			}, nil
		}
	}
}

var RequiredScopes = []string{
	"openid",
	"offline_access", // For retrieving refresh token.
	"create:clients", "delete:clients", "read:clients", "update:clients",
	"read:client_grants",
	"create:resource_servers", "delete:resource_servers", "read:resource_servers", "update:resource_servers",
	"create:roles", "delete:roles", "read:roles", "update:roles",
	"create:rules", "delete:rules", "read:rules", "update:rules",
	"create:users", "delete:users", "read:users", "update:users",
	"read:branding", "update:branding",
	"create:phone_providers", "read:phone_providers", "update:phone_providers", "delete:phone_providers",
	"create:email_templates", "read:email_templates", "update:email_templates",
	"create:email_provider", "read:email_provider", "update:email_provider", "delete:email_provider",
	"read:flows", "read:forms", "read:flows_vault_connections",
	"read:connections", "update:connections", "read:connections_options", "update:connections_options",
	"read:client_keys", "read:logs", "read:tenant_settings", "update:tenant_settings",
	"read:custom_domains", "create:custom_domains", "update:custom_domains", "delete:custom_domains",
	"read:anomaly_blocks", "delete:anomaly_blocks",
	"create:log_streams", "delete:log_streams", "read:log_streams", "update:log_streams",
	"create:actions", "delete:actions", "read:actions", "update:actions",
	"create:organizations", "delete:organizations", "read:organizations", "update:organizations", "read:organization_members", "read:organization_member_roles", "read:organization_connections",
	"read:prompts", "update:prompts",
	"read:attack_protection", "update:attack_protection",
	"read:event_streams", "create:event_streams", "update:event_streams", "delete:event_streams",
	"read:network_acls", "create:network_acls", "update:network_acls", "delete:network_acls",
	"read:token_exchange_profiles", "create:token_exchange_profiles", "update:token_exchange_profiles", "delete:token_exchange_profiles",
	"read:organization_invitations", "create:organization_invitations", "delete:organization_invitations",
}

// GetDeviceCode kicks-off the device authentication flow by requesting
// a device code from Auth0. The returned state contains the
// URI for the next step of the flow.
func GetDeviceCode(ctx context.Context, httpClient *http.Client, additionalScopes []string, domain string) (State, error) {
	a := credentials

	data := url.Values{
		"client_id": []string{a.ClientID},
		"scope":     []string{strings.Join(append(RequiredScopes, additionalScopes...), " ")},
		"audience":  []string{domain},
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		a.DeviceCodeEndpoint,
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return State{}, fmt.Errorf("failed to create the request: %w", err)
	}

	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := httpClient.Do(request)
	if err != nil {
		return State{}, fmt.Errorf("failed to send the request: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(response.Body)
		if err != nil {
			return State{}, fmt.Errorf(
				"received a %d response and failed to read the response",
				response.StatusCode,
			)
		}

		return State{}, fmt.Errorf("received a %d response: %s", response.StatusCode, bodyBytes)
	}

	var state State
	if err = json.NewDecoder(response.Body).Decode(&state); err != nil {
		return State{}, fmt.Errorf("failed to decode the response: %w", err)
	}

	return state, nil
}

func parseTenant(accessToken string) (tenant, domain string, err error) {
	parts := strings.Split(accessToken, ".")
	v, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", "", err
	}
	var payload struct {
		AUDs []string `json:"aud"`
	}
	if err := json.Unmarshal(v, &payload); err != nil {
		return "", "", err
	}

	for _, aud := range payload.AUDs {
		u, err := url.Parse(aud)
		if err != nil {
			return "", "", err
		}
		if u.Path == audiencePath {
			parts := strings.Split(u.Host, ".")
			return parts[0], u.Host, nil
		}
	}
	return "", "", fmt.Errorf("audience not found for %s", audiencePath)
}

// ClientCredentials encapsulates all data to facilitate access token creation with client credentials (client ID and client secret).
type ClientCredentials struct {
	ClientID     string
	ClientSecret string
	Domain       string
}

// GetAccessTokenFromClientCreds generates an access token from client credentials.
func GetAccessTokenFromClientCreds(ctx context.Context, args ClientCredentials) (Result, error) {
	u, err := url.Parse("https://" + args.Domain)
	if err != nil {
		return Result{}, err
	}

	credsConfig := &clientcredentials.Config{
		ClientID:     args.ClientID,
		ClientSecret: args.ClientSecret,
		TokenURL:     u.String() + "/oauth/token",
		EndpointParams: url.Values{
			"client_id": {args.ClientID},
			"audience":  {u.String() + "/api/v2/"},
		},
	}

	resp, err := credsConfig.Token(ctx)
	if err != nil {
		return Result{}, err
	}

	return Result{
		AccessToken: resp.AccessToken,
		ExpiresAt:   resp.Expiry,
	}, nil
}

// GetAccessTokenFromClientPrivateJWT generates an access token from client prviateJWT.
func GetAccessTokenFromClientPrivateJWT(args PrivateKeyJwtTokenSource) (Result, error) {
	resp, err := args.Token()
	if err != nil {
		return Result{}, err
	}

	return Result{
		AccessToken: resp.AccessToken,
		ExpiresAt:   resp.Expiry,
	}, nil
}

// PrivateKeyJwtTokenSource implements oauth2.TokenSource for Private Key JWT client authentication.
type PrivateKeyJwtTokenSource struct {
	Ctx                       context.Context
	URI                       string
	ClientID                  string
	ClientAssertionSigningAlg string
	ClientAssertionPrivateKey string
	Audience                  string
}

// Token generates a new token using Private Key JWT client authentication.
func (p PrivateKeyJwtTokenSource) Token() (*oauth2.Token, error) {
	alg, err := DetermineSigningAlgorithm(p.ClientAssertionSigningAlg)
	if err != nil {
		return nil, fmt.Errorf("invalid algorithm: %w", err)
	}

	baseURL, err := url.Parse(p.URI)
	if err != nil {
		return nil, fmt.Errorf("invalid URI: %w", err)
	}

	assertion, err := CreateClientAssertion(
		alg,
		p.ClientAssertionPrivateKey,
		p.ClientID,
		baseURL.JoinPath("/").String(),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create client assertion: %w", err)
	}

	cfg := &clientcredentials.Config{
		TokenURL:  p.URI + "/oauth/token",
		AuthStyle: oauth2.AuthStyleInParams,
		EndpointParams: url.Values{
			"audience":              []string{p.Audience},
			"client_assertion_type": []string{"urn:ietf:params:oauth:client-assertion-type:jwt-bearer"},
			"client_assertion":      []string{assertion},
			"grant_type":            []string{"client_credentials"},
		},
	}

	token, err := cfg.Token(p.Ctx)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}

	return token, nil
}

// DetermineSigningAlgorithm returns the appropriate JWA signature algorithm based on the string representation.
func DetermineSigningAlgorithm(alg string) (jwa.SignatureAlgorithm, error) {
	switch alg {
	case "RS256":
		return jwa.RS256, nil
	case "RS384":
		return jwa.RS384, nil
	case "PS256":
		return jwa.PS256, nil
	default:
		return "", fmt.Errorf("unsupported client assertion algorithm %q", alg)
	}
}

// CreateClientAssertion creates a JWT token for client authentication with the specified lifetime.
func CreateClientAssertion(alg jwa.SignatureAlgorithm, signingKey, clientID, audience string) (string, error) {
	key, err := jwk.ParseKey([]byte(signingKey), jwk.WithPEM(true))
	if err != nil {
		return "", fmt.Errorf("failed to parse signing key: %w", err)
	}

	// Verify that the key type is compatible with the algorithm.
	if key.KeyType() != "RSA" {
		return "", fmt.Errorf("%s algorithm requires an RSA key, but got %s", alg, key.KeyType())
	}

	now := time.Now()

	token, err := jwt.NewBuilder().
		IssuedAt(now).
		NotBefore(now).
		Subject(clientID).
		JwtID(uuid.NewString()).
		Issuer(clientID).
		Audience([]string{audience}).
		Expiration(now.Add(2 * time.Minute)).
		Build()
	if err != nil {
		return "", fmt.Errorf("failed to build JWT: %w", err)
	}

	signedToken, err := jwt.Sign(token, jwt.WithKey(alg, key))
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	return string(signedToken), nil
}
