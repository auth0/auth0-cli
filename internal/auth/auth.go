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
	audiencePath       = "/api/v2/"
)

var requiredScopes = []string{
	"openid",
	"create:actions", "delete:actions", "read:actions", "update:actions",
	"create:clients", "delete:clients", "read:clients", "update:clients",
	"create:connections", "delete:connections", "read:connections", "update:connections",
	"create:hooks", "delete:hooks", "read:hooks", "update:hooks",
	"create:resource_servers", "delete:resource_servers", "read:resource_servers", "update:resource_servers",
	"create:rules", "delete:rules", "read:rules", "update:rules",
	"read:client_keys", "read:logs",
	"create:roles", "delete:roles", "read:roles", "update:roles",
	"create:custom_domains", "delete:custom_domains", "read:custom_domains", "update:custom_domains",
	"read:users",
}

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
		"scope":     {strings.Join(requiredScopes, " ")},
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
