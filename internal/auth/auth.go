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

	"golang.org/x/oauth2/clientcredentials"
)

const (
	audiencePath           = "/api/v2/"
	waitThresholdInSeconds = 3
	// SecretsNamespace is the namespace used to set/get values from the keychain.
	SecretsNamespace = "auth0-cli"
)

var requiredScopes = []string{
	"openid",
	"offline_access", // This is used to retrieve a refresh token.
	"create:clients", "read:clients", "update:clients", "delete:clients",
	"read:client_keys",
	"create:client_grants", "read:client_grants", "update:client_grants", "delete:client_grants",
	"create:resource_servers", "read:resource_servers", "update:resource_servers", "delete:resource_servers",
	"create:connections", "read:connections", "update:connections", "delete:connections",
	"create:users", "read:users", "update:users", "delete:users",
	"create:roles", "read:roles", "update:roles", "delete:roles",
	"create:actions", "read:actions", "update:actions", "delete:actions",
	"read:triggers", "update:triggers",
	"create:rules", "read:rules", "update:rules", "delete:rules",
	"read:rules_configs", "update:rules_configs", "delete:rules_configs",
	"create:hooks", "read:hooks", "update:hooks", "delete:hooks",
	"read:attack_protection", "update:attack_protection",
	"create:organizations", "read:organizations", "update:organizations", "delete:organizations",
	"create:organization_members", "read:organization_members", "delete:organization_members",
	"create:organization_connections", "read:organization_connections", "update:organization_connections", "delete:organization_connections",
	"create:organization_member_roles", "read:organization_member_roles", "delete:organization_member_roles",
	"create:organization_invitations", "read:organization_invitations", "delete:organization_invitations",
	"read:prompts", "update:prompts",
	"read:branding", "update:branding", "delete:branding",
	"create:custom_domains", "read:custom_domains", "update:custom_domains", "delete:custom_domains",
	"create:email_provider", "read:email_provider", "update:email_provider", "delete:email_provider",
	"create:email_templates", "read:email_templates", "update:email_templates",
	"read:tenant_settings", "update:tenant_settings",
	"read:anomaly_blocks", "delete:anomaly_blocks",
	"create:log_streams", "read:log_streams", "update:log_streams", "delete:log_streams",
	"read:stats",
	"read:insights",
	"read:logs",
	"create:shields", "read:shields", "update:shields", "delete:shields",
	"create:users_app_metadata", "read:users_app_metadata", "update:users_app_metadata", "delete:users_app_metadata",
	"create:user_custom_blocks", "read:user_custom_blocks", "delete:user_custom_blocks",
	"create:user_tickets",
	"blacklist:tokens",
	"read:grants", "delete:grants",
	"read:mfa_policies", "update:mfa_policies",
	"read:guardian_factors", "update:guardian_factors",
	"read:guardian_enrollments", "delete:guardian_enrollments",
	"create:guardian_enrollment_tickets",
	"read:user_idp_tokens",
	"create:passwords_checking_job", "delete:passwords_checking_job",
	"read:limits", "update:limits",
	"read:entitlements",
}

// Authenticator is used to facilitate the login process.
type Authenticator struct {
	Audience           string
	ClientID           string
	DeviceCodeEndpoint string
	OauthTokenEndpoint string
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
	ExpiresAt    time.Time
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

// New returns a new instance of Authenticator using Auth0 Public Cloud default values
func New() *Authenticator {
	return &Authenticator{
		Audience:           "https://*.auth0.com/api/v2/",
		ClientID:           "2iZo3Uczt5LFHacKdM0zzgUO2eG2uDjT",
		DeviceCodeEndpoint: "https://auth0.auth0.com/oauth/device/code",
		OauthTokenEndpoint: "https://auth0.auth0.com/oauth/token",
	}
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
				ExpiresAt: time.Now().Add(
					time.Duration(res.ExpiresIn) * time.Second,
				),
				Tenant: ten,
				Domain: domain,
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

// ClientCredentials encapsulates all data to facilitate access token creation with client credentials (client ID and client secret)
type ClientCredentials struct {
	ClientID     string
	ClientSecret string
	Domain       string
}

// GetAccessTokenFromClientCreds generates an access token from client credentials
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
			"scope":     {strings.Join(RequiredScopesMin(), " ")},
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
