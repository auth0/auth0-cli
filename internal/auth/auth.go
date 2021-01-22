package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
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
	scope = "openid read:roles"
	audiencePath       = "/api/v2/"
)

type Authenticator struct {
}

type Result struct {
	Tenant      string
	AccessToken string
	ExpiresIn   int64
}

type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri_complete"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

func (d *deviceCodeResponse) IntervalDuration() time.Duration {
	return time.Duration(d.Interval) * time.Second
}

func (a *Authenticator) Authenticate(ctx context.Context) (Result, error) {
	dcr, err := a.getDeviceCode(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("cannot get device code: %w", err)
	}

	fmt.Printf("Your pairing code is: %s\n", dcr.UserCode)
	err = openURL(dcr.VerificationURI)
	if err != nil {
		return Result{}, fmt.Errorf("cannot open URL: %w", err)
	}

	return a.awaitResponse(ctx, dcr)
}

func (a *Authenticator) getDeviceCode(ctx context.Context) (*deviceCodeResponse, error) {
	data := url.Values{
		"client_id": {clientID},
		"scope":     {scope},
		"audience":  {"https://*.auth0.com/api/v2/"},
	}
	r, err := http.PostForm(deviceCodeEndpoint, data)
	if err != nil {
		return nil, fmt.Errorf("cannot get device code: %w", err)
	}
	defer r.Body.Close()
	var res deviceCodeResponse
	err = json.NewDecoder(r.Body).Decode(&res)
	if err != nil {
		return nil, fmt.Errorf("cannot decode response: %w", err)
	}

	return &res, nil
}

func (a *Authenticator) awaitResponse(ctx context.Context, dcr *deviceCodeResponse) (Result, error) {
	t := time.NewTicker(dcr.IntervalDuration())
	for {
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		case <-t.C:
			data := url.Values{
				"client_id":   {clientID},
				"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
				"device_code": {dcr.DeviceCode},
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

			t, err := parseTenant(res.AccessToken)
			if err != nil {
				return Result{}, fmt.Errorf("cannot parse tenant from the given access token: %w", err)
			}
			return Result{
				AccessToken: res.AccessToken,
				ExpiresIn:   res.ExpiresIn,
				Tenant:      t,
			}, nil
		}
	}
}

func openURL(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}

func parseTenant(accessToken string) (string, error) {
	parts := strings.Split(accessToken, ".")
	v, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}
	var payload struct {
		AUDs []string `json:"aud"`
	}
	if err := json.Unmarshal([]byte(v), &payload); err != nil {
		return "", err
	}
	for _, aud := range payload.AUDs {
		u, err := url.Parse(aud)
		if err != nil {
			return "", err
		}
		if u.Path == audiencePath {
			parts := strings.Split(u.Host, ".")
			return parts[0], nil
		}
	}
	return "", fmt.Errorf("audience not found for %s", audiencePath)
}
