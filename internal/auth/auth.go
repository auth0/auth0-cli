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

	"github.com/joeshaw/envdecode"
)

const (
	audiencePath           = "/api/v2/"
	waitThresholdInSeconds = 3
	// SecretsNamespace is the namespace used to set/get values from the keychain.
	SecretsNamespace = "auth0-cli"
)

var requiredScopes = []string{
	"openid",
	"offline_access", // <-- to get a refresh token.
	"create:clients", "delete:clients", "read:clients", "update:clients",
	"create:resource_servers", "delete:resource_servers", "read:resource_servers", "update:resource_servers",
	"create:roles", "delete:roles", "read:roles", "update:roles",
	"create:rules", "delete:rules", "read:rules", "update:rules",
	"create:users", "delete:users", "read:users", "update:users",
	"read:branding", "update:branding",
	"read:email_templates", "update:email_templates",
	"read:connections", "update:connections",
	"read:client_keys", "read:logs", "read:tenant_settings",
	"read:custom_domains", "create:custom_domains", "update:custom_domains", "delete:custom_domains",
	"read:anomaly_blocks", "delete:anomaly_blocks",
	"create:log_streams", "delete:log_streams", "read:log_streams", "update:log_streams",
	"create:actions", "delete:actions", "read:actions", "update:actions",
	"create:organizations", "delete:organizations", "read:organizations", "update:organizations", "read:organization_members", "read:organization_member_roles",
	"read:prompts", "update:prompts",
	"read:attack_protection", "update:attack_protection",
}

// Authenticator is used to facilitate the login process.
type Authenticator struct {
	Audience           string `env:"AUTH0_AUDIENCE,default=https://*.auth0.com/api/v2/"`
	ClientID           string `env:"AUTH0_CLIENT_ID,default=2iZo3Uczt5LFHacKdM0zzgUO2eG2uDjT"`
	DeviceCodeEndpoint string `env:"AUTH0_DEVICE_CODE_ENDPOINT,default=https://auth0.auth0.com/oauth/device/code"`
	OauthTokenEndpoint string `env:"AUTH0_OAUTH_TOKEN_ENDPOINT,default=https://auth0.auth0.com/oauth/token"`
}

// SecretStore provides access to stored sensitive data.
type SecretStore interface {
	// Get gets the secret
	Get(namespace, key string) (string, error)
	// Delete removes the secret
	Delete(namespace, key string) error
}

type Result struct {
	Tenant       string
	Domain       string
	RefreshToken string
	AccessToken  string
	ExpiresIn    int64
}

type State struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri_complete"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// RequiredScopes returns the scopes used for login.
func RequiredScopes() []string {
	return requiredScopes
}

// RequiredScopesMin returns minimum scopes used for login in integration tests.
func RequiredScopesMin() []string {
	var min []string
	for _, s := range requiredScopes {
		if s != "offline_access" && s != "openid" {
			min = append(min, s)
		}
	}
	return min
}

func (s *State) IntervalDuration() time.Duration {
	return time.Duration(s.Interval+waitThresholdInSeconds) * time.Second
}

// New returns a new instance of Authenticator
// after decoding its parameters from env vars.
func New() (*Authenticator, error) {
	authenticator := Authenticator{}

	if err := envdecode.StrictDecode(&authenticator); err != nil {
		return nil, fmt.Errorf("failed to decode env vars for authenticator: %w", err)
	}

	return &authenticator, nil
}

// Start kicks-off the device authentication flow by requesting
// a device code from Auth0. The returned state contains the
// URI for the next step of the flow.
func (a *Authenticator) Start(ctx context.Context) (State, error) {
	state, err := a.getDeviceCode(ctx)
	if err != nil {
		return State{}, fmt.Errorf("failed to get the device code: %w", err)
	}

	return state, nil
}

// Wait waits until the user is logged in on the browser.
func (a *Authenticator) Wait(ctx context.Context, state State) (Result, error) {
	t := time.NewTicker(state.IntervalDuration())
	for {
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		case <-t.C:
			data := url.Values{
				"client_id":   []string{a.ClientID},
				"grant_type":  []string{"urn:ietf:params:oauth:grant-type:device_code"},
				"device_code": []string{state.DeviceCode},
			}
			r, err := http.PostForm(a.OauthTokenEndpoint, data)
			if err != nil {
				return Result{}, fmt.Errorf("cannot get device code: %w", err)
			}
			defer r.Body.Close()

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
				ExpiresIn:    res.ExpiresIn,
				Tenant:       ten,
				Domain:       domain,
			}, nil
		}
	}
}

func (a *Authenticator) getDeviceCode(ctx context.Context) (State, error) {
	data := url.Values{
		"client_id": []string{a.ClientID},
		"scope":     []string{strings.Join(requiredScopes, " ")},
		"audience":  []string{a.Audience},
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

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return State{}, fmt.Errorf("failed to send the request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusBadRequest {
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
	err = json.NewDecoder(response.Body).Decode(&state)
	if err != nil {
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
