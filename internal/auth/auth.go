package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// 1st request
// curl --request POST \
//   --url 'https://auth0.auth0.com/oauth/device/code' \
//   --header 'content-type: application/x-www-form-urlencoded' \
//   --data 'client_id=2iZo3Uczt5LFHacKdM0zzgUO2eG2uDjT' \
//   --data 'scope=openid read:roles' \
//   --data audience=https://\*.auth0.com/api/v2/

// polling request
// curl --request POST \
//   --url 'https://auth0.auth0.com/oauth/token' \
//   --header 'content-type: application/x-www-form-urlencoded' \
//   --data grant_type=urn:ietf:params:oauth:grant-type:device_code \
//   --data device_code=9GtgUcsGKzXkU-i70RN74baY \
//   --data 'client_id=2iZo3Uczt5LFHacKdM0zzgUO2eG2uDjT'

const (
	clientID           = "2iZo3Uczt5LFHacKdM0zzgUO2eG2uDjT"
	deviceCodeEndpoint = "https://auth0.auth0.com/oauth/device/code"
	oauthTokenEndpoint = "https://auth0.auth0.com/oauth/token"
	// TODO(jfatta) extend the scope as we extend the CLI:
	scope        = "openid blacklist:tokens create:actions create:actions create:client_grants create:client_keys create:clients create:connections create:custom_domains create:device_credentials create:email_provider create:email_templates create:guardian_enrollment_tickets create:hooks create:log_streams create:organization_connections create:organization_invitations create:organization_member_roles create:organization_members create:organizations create:passwords_checking_job create:resource_servers create:role_members create:roles create:rules create:shields create:signing_keys create:user_custom_blocks create:user_tickets create:users create:users_app_metadata delete:actions delete:actions delete:anomaly_blocks delete:branding delete:client_grants delete:client_keys delete:clients delete:connections delete:custom_domains delete:device_credentials delete:email_provider delete:grants delete:guardian_enrollments delete:hooks delete:log_streams delete:organization_connections delete:organization_invitations delete:organization_member_roles delete:organization_members delete:organizations delete:passwords_checking_job delete:resource_servers delete:role_members delete:roles delete:rules delete:rules_configs delete:shields delete:user_custom_blocks delete:users delete:users_app_metadata read:actions read:actions read:anomaly_blocks read:branding read:client_grants read:client_keys read:clients read:connections read:custom_domains read:device_credentials read:email_provider read:email_templates read:grants read:guardian_enrollments read:guardian_factors read:hooks read:limits read:log_streams read:logs read:logs_users read:mfa_policies read:organization_connections read:organization_invitations read:organization_member_roles read:organization_members read:organizations read:prompts read:resource_servers read:role_members read:roles read:rules read:rules_configs read:shields read:signing_keys read:stats read:tenant_settings read:triggers read:user_custom_blocks read:user_idp_tokens read:users read:users_app_metadata update:actions update:actions update:branding update:client_grants update:client_keys update:clients update:connections update:custom_domains update:device_credentials update:email_provider update:email_templates update:guardian_factors update:hooks update:limits update:log_streams update:mfa_policies update:organization_connections update:organizations update:prompts update:resource_servers update:roles update:rules update:rules_configs update:shields update:signing_keys update:tenant_settings update:triggers update:users update:users_app_metadata"
	audiencePath = "/api/v2/"
)

type Authenticator struct {
}

type Result struct {
	Tenant      string
	Domain      string
	AccessToken string
	ExpiresIn   int64
}

type State struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri_complete"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

func (s *State) IntervalDuration() time.Duration {
	return time.Duration(s.Interval) * time.Second
}

func (a *Authenticator) Start(ctx context.Context) (State, error) {
	s, err := a.getDeviceCode(ctx)
	if err != nil {
		return State{}, fmt.Errorf("cannot get device code: %w", err)
	}
	return s, nil
}

func (a *Authenticator) Wait(ctx context.Context, state State) (Result, error) {
	t := time.NewTicker(state.IntervalDuration())
	for {
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		case <-t.C:
			data := url.Values{
				"client_id":   {clientID},
				"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
				"device_code": {state.DeviceCode},
			}
			r, err := http.PostForm(oauthTokenEndpoint, data)
			if err != nil {
				return Result{}, fmt.Errorf("cannot get device code: %w", err)
			}
			defer r.Body.Close()

			var res struct {
				AccessToken      string  `json:"access_token"`
				IDToken          string  `json:"id_token"`
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
				AccessToken: res.AccessToken,
				ExpiresIn:   res.ExpiresIn,
				Tenant:      ten,
				Domain:      domain,
			}, nil
		}
	}
}

func (a *Authenticator) getDeviceCode(ctx context.Context) (State, error) {
	data := url.Values{
		"client_id": {clientID},
		"scope":     {scope},
		"audience":  {"https://*.auth0.com/api/v2/"},
	}
	r, err := http.PostForm(deviceCodeEndpoint, data)
	if err != nil {
		return State{}, fmt.Errorf("cannot get device code: %w", err)
	}
	defer r.Body.Close()
	var res State
	err = json.NewDecoder(r.Body).Decode(&res)
	if err != nil {
		return State{}, fmt.Errorf("cannot decode response: %w", err)
	}

	return res, nil
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
	if err := json.Unmarshal([]byte(v), &payload); err != nil {
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
