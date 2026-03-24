package auth0

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/auth0/go-auth0/management"

	"github.com/auth0/auth0-cli/internal/buildinfo"

	"github.com/auth0/auth0-cli/internal/utils"
)

const (
	quickstartsMetaURL            = "https://auth0.com/docs/meta/quickstarts"
	quickstartsOrg                = "auth0-samples"
	quickstartsDefaultCallbackURL = "https://YOUR_APP/callback"
)

type Quickstarts []Quickstart

type Quickstart struct {
	Name                 string `json:"name"`
	AppType              string `json:"appType"`
	URL                  string `json:"url"`
	Logo                 string `json:"logo"`
	DownloadLink         string `json:"downloadLink"`
	DownloadInstructions string `json:"downloadInstructions"`
}

func (q Quickstart) SamplePath(downloadPath string) (string, error) {
	query, err := url.ParseQuery(q.DownloadLink)
	if err != nil {
		return "", err
	}

	return path.Join(downloadPath, query.Get("path")), nil
}

func (q Quickstart) Download(ctx context.Context, downloadPath string, client *management.Client) error {
	quickstartEndpoint := fmt.Sprintf("https://auth0.com%s", q.DownloadLink)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, quickstartEndpoint, nil)
	if err != nil {
		return err
	}

	params := request.URL.Query()
	params.Add("org", quickstartsOrg)
	params.Add("client_id", client.GetClientID())

	// Callback URL, if not set, it will just take the default one.
	callbackURL := quickstartsDefaultCallbackURL
	if list := client.GetCallbacks(); len(list) > 0 {
		callbackURL = list[0]
	}
	params.Add("callback_url", callbackURL)

	request.URL.RawQuery = params.Encode()
	request.Header.Set("Content-Type", "application/json")

	userAgent := "Auth0 CLI" // Set User-Agent header using the standard CLI format.
	request.Header.Set("User-Agent", fmt.Sprintf("%v/%v", userAgent, strings.TrimPrefix(buildinfo.Version, "v")))

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("expected status %d, got %d", http.StatusOK, response.StatusCode)
	}

	// Check if we're getting a zip file or HTML response.
	contentType := response.Header.Get("Content-Type")
	if contentType != "" && !strings.Contains(contentType, "application/zip") && !strings.Contains(contentType, "application/octet-stream") {
		return fmt.Errorf("expected zip file but got content-type: %s. The quickstart endpoint may have returned an error page", contentType)
	}

	tmpFile, err := os.CreateTemp("", "auth0-quickstart*.zip")
	if err != nil {
		return err
	}

	_, err = io.Copy(tmpFile, response.Body)
	if err != nil {
		return err
	}

	if err = tmpFile.Close(); err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	if err = os.RemoveAll(downloadPath); err != nil {
		return err
	}

	if err = utils.Unzip(tmpFile.Name(), downloadPath); err != nil {
		return fmt.Errorf("failed to unzip file: %w", err)
	}

	return nil
}

func GetQuickstarts(ctx context.Context) (Quickstarts, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, quickstartsMetaURL, nil)
	if err != nil {
		return nil, err
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"failed to fetch quickstarts metadata, response has status code: %d",
			response.StatusCode,
		)
	}

	var quickstarts Quickstarts
	if err := json.NewDecoder(response.Body).Decode(&quickstarts); err != nil {
		return nil, fmt.Errorf("failed to decode quickstarts metadata response: %w", err)
	}

	return quickstarts, nil
}

func (q Quickstarts) FindByStack(stack string) (Quickstart, error) {
	for _, quickstart := range q {
		if quickstart.Name == stack {
			return quickstart, nil
		}
	}

	return Quickstart{}, fmt.Errorf("failed to find any quickstarts for stack: %q", stack)
}

func (q Quickstarts) FilterByType(qsType string) (Quickstarts, error) {
	var filteredQuickstarts []Quickstart
	for _, quickstart := range q {
		if quickstart.AppType == qsType {
			filteredQuickstarts = append(filteredQuickstarts, quickstart)
		}
	}

	if len(filteredQuickstarts) == 0 {
		return nil, fmt.Errorf("failed to find any quickstarts for type: %q", qsType)
	}

	return filteredQuickstarts, nil
}

func (q Quickstarts) Stacks() []string {
	var stacks []string

	for _, qs := range q {
		stacks = append(stacks, qs.Name)
	}

	return stacks
}

const DETECTION_SUB = "DETECTION_SUB"

type RequestParams struct {
	AppType           string
	Callbacks         []string
	AllowedLogoutURLs []string
	WebOrigins        []string
}

type AppConfig struct {
	EnvValues     map[string]string
	RequestParams RequestParams
}

// Map key format: "type:framework:build_tool"
var QuickstartConfigs = map[string]AppConfig{

	// ==========================================
	// Single Page Applications (SPA)
	// ==========================================
	"spa:react:vite": {
		EnvValues: map[string]string{
			"VITE_AUTH0_DOMAIN":    DETECTION_SUB,
			"VITE_AUTH0_CLIENT_ID": DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "spa",
			Callbacks:         []string{"http://localhost:5173/callback"},
			AllowedLogoutURLs: []string{"http://localhost:5173"},
			WebOrigins:        []string{DETECTION_SUB},
		},
	},
	"spa:angular:none": {
		EnvValues: map[string]string{
			"domain":   DETECTION_SUB,
			"clientId": DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "spa",
			Callbacks:         []string{"http://localhost:4200/callback"},
			AllowedLogoutURLs: []string{"http://localhost:4200"},
			WebOrigins:        []string{DETECTION_SUB},
		},
	},
	"spa:vue:vite": {
		EnvValues: map[string]string{
			"VITE_AUTH0_DOMAIN":    DETECTION_SUB,
			"VITE_AUTH0_CLIENT_ID": DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "spa",
			Callbacks:         []string{"http://localhost:5173/callback"},
			AllowedLogoutURLs: []string{"http://localhost:5173"},
			WebOrigins:        []string{DETECTION_SUB},
		},
	},
	"spa:svelte:vite": {
		EnvValues: map[string]string{
			"VITE_AUTH0_DOMAIN":    DETECTION_SUB,
			"VITE_AUTH0_CLIENT_ID": DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "spa",
			Callbacks:         []string{"http://localhost:5173/callback"},
			AllowedLogoutURLs: []string{"http://localhost:5173"},
			WebOrigins:        []string{DETECTION_SUB},
		},
	},
	"spa:vanilla-javascript:vite": {
		EnvValues: map[string]string{
			"VITE_AUTH0_DOMAIN":    DETECTION_SUB,
			"VITE_AUTH0_CLIENT_ID": DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "spa",
			Callbacks:         []string{"http://localhost:5173/callback"},
			AllowedLogoutURLs: []string{"http://localhost:5173"},
			WebOrigins:        []string{DETECTION_SUB},
		},
	},
	"spa:flutter-web:none": {
		EnvValues: map[string]string{
			"domain":   DETECTION_SUB,
			"clientId": DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "spa",
			Callbacks:         []string{DETECTION_SUB},
			AllowedLogoutURLs: []string{DETECTION_SUB},
			WebOrigins:        []string{DETECTION_SUB},
		},
	},

	// ==========================================
	// Regular Web Applications
	// ==========================================
	"regular:nextjs:none": {
		EnvValues: map[string]string{
			"AUTH0_DOMAIN":        DETECTION_SUB,
			"AUTH0_CLIENT_ID":     DETECTION_SUB,
			"AUTH0_CLIENT_SECRET": DETECTION_SUB,
			"AUTH0_SECRET":        DETECTION_SUB,
			"APP_BASE_URL":        DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{"http://localhost:3000/callback"},
			AllowedLogoutURLs: []string{"http://localhost:3000"},
		},
	},
	"regular:nuxt:none": {
		EnvValues: map[string]string{
			"NUXT_AUTH0_DOMAIN":         DETECTION_SUB,
			"NUXT_AUTH0_CLIENT_ID":      DETECTION_SUB,
			"NUXT_AUTH0_CLIENT_SECRET":  DETECTION_SUB,
			"NUXT_AUTH0_SESSION_SECRET": DETECTION_SUB,
			"NUXT_AUTH0_APP_BASE_URL":   DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{"http://localhost:3000/callback"},
			AllowedLogoutURLs: []string{"http://localhost:3000"},
		},
	},
	"regular:fastify:none": {
		EnvValues: map[string]string{
			"AUTH0_DOMAIN":        DETECTION_SUB,
			"AUTH0_CLIENT_ID":     DETECTION_SUB,
			"AUTH0_CLIENT_SECRET": DETECTION_SUB,
			"SESSION_SECRET":      DETECTION_SUB,
			"APP_BASE_URL":        DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{"http://localhost:3000/callback"},
			AllowedLogoutURLs: []string{"http://localhost:3000"},
		},
	},
	"regular:sveltekit:none": {
		EnvValues: map[string]string{
			"VITE_AUTH0_DOMAIN":    DETECTION_SUB,
			"VITE_AUTH0_CLIENT_ID": DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{DETECTION_SUB},
			AllowedLogoutURLs: []string{DETECTION_SUB},
		},
	},
	"regular:express:none": {
		EnvValues: map[string]string{
			"ISSUER_BASE_URL": DETECTION_SUB,
			"CLIENT_ID":       DETECTION_SUB,
			"SECRET":          DETECTION_SUB,
			"BASE_URL":        DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{"http://localhost:3000/callback"},
			AllowedLogoutURLs: []string{"http://localhost:3000"},
		},
	},
	"regular:hono:none": {
		EnvValues: map[string]string{
			"AUTH0_DOMAIN":                 DETECTION_SUB,
			"AUTH0_CLIENT_ID":              DETECTION_SUB,
			"AUTH0_CLIENT_SECRET":          DETECTION_SUB,
			"AUTH0_SESSION_ENCRYPTION_KEY": DETECTION_SUB,
			"BASE_URL":                     DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{"http://localhost:3000/callback"},
			AllowedLogoutURLs: []string{"http://localhost:3000"},
		},
	},
	"regular:vanilla-python:none": {
		EnvValues: map[string]string{
			"AUTH0_DOMAIN":        DETECTION_SUB,
			"AUTH0_CLIENT_ID":     DETECTION_SUB,
			"AUTH0_CLIENT_SECRET": DETECTION_SUB,
			"AUTH0_SECRET":        DETECTION_SUB,
			"AUTH0_REDIRECT_URI":  DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{"http://localhost:5000/callback"},
			AllowedLogoutURLs: []string{"http://localhost:5000"},
		},
	},
	"regular:django:none": {
		EnvValues: map[string]string{
			"AUTH0_DOMAIN":        DETECTION_SUB,
			"AUTH0_CLIENT_ID":     DETECTION_SUB,
			"AUTH0_CLIENT_SECRET": DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{DETECTION_SUB},
			AllowedLogoutURLs: []string{DETECTION_SUB},
		},
	},
	"regular:vanilla-go:none": {
		EnvValues: map[string]string{
			"AUTH0_DOMAIN":        DETECTION_SUB,
			"AUTH0_CLIENT_ID":     DETECTION_SUB,
			"AUTH0_CLIENT_SECRET": DETECTION_SUB,
			"AUTH0_CALLBACK_URL":  DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{DETECTION_SUB},
			AllowedLogoutURLs: []string{DETECTION_SUB},
		},
	},
	"regular:vanilla-java:maven": {
		EnvValues: map[string]string{
			"auth0.domain":       DETECTION_SUB,
			"auth0.clientId":     DETECTION_SUB,
			"auth0.clientSecret": DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{DETECTION_SUB},
			AllowedLogoutURLs: []string{DETECTION_SUB},
		},
	},
	"regular:java-ee:maven": {
		EnvValues: map[string]string{
			"auth0.domain":       DETECTION_SUB,
			"auth0.clientId":     DETECTION_SUB,
			"auth0.clientSecret": DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{DETECTION_SUB},
			AllowedLogoutURLs: []string{DETECTION_SUB},
		},
	},
	"regular:spring-boot:maven": {
		EnvValues: map[string]string{
			"okta.oauth2.issuer":        DETECTION_SUB,
			"okta.oauth2.client-id":     DETECTION_SUB,
			"okta.oauth2.client-secret": DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{"http://localhost:8000/callback"},
			AllowedLogoutURLs: []string{"http://localhost:8000"},
		},
	},
	"regular:aspnet-mvc:none": {
		EnvValues: map[string]string{
			"Auth0:Domain":       DETECTION_SUB,
			"Auth0:ClientId":     DETECTION_SUB,
			"Auth0:ClientSecret": DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{DETECTION_SUB},
			AllowedLogoutURLs: []string{DETECTION_SUB},
		},
	},
	"regular:aspnet-blazor:none": {
		EnvValues: map[string]string{
			"Auth0:Domain":   DETECTION_SUB,
			"Auth0:ClientId": DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{DETECTION_SUB},
			AllowedLogoutURLs: []string{DETECTION_SUB},
		},
	},
	"regular:aspnet-owin:none": {
		EnvValues: map[string]string{
			"auth0:Domain":       DETECTION_SUB,
			"auth0:ClientId":     DETECTION_SUB,
			"auth0:ClientSecret": DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{DETECTION_SUB},
			AllowedLogoutURLs: []string{DETECTION_SUB},
		},
	},
	"regular:vanilla-php:composer": {
		EnvValues: map[string]string{
			"AUTH0_DOMAIN":        DETECTION_SUB,
			"AUTH0_CLIENT_ID":     DETECTION_SUB,
			"AUTH0_CLIENT_SECRET": DETECTION_SUB,
			"AUTH0_COOKIE_SECRET": DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{DETECTION_SUB},
			AllowedLogoutURLs: []string{DETECTION_SUB},
		},
	},
	"regular:laravel:composer": {
		EnvValues: map[string]string{
			"AUTH0_DOMAIN":        DETECTION_SUB,
			"AUTH0_CLIENT_ID":     DETECTION_SUB,
			"AUTH0_CLIENT_SECRET": DETECTION_SUB,
			"AUTH0_COOKIE_SECRET": DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{"http://localhost:8000/callback"},
			AllowedLogoutURLs: []string{"http://localhost:8000"},
		},
	},
	"regular:rails:none": {
		EnvValues: map[string]string{
			"auth0_domain":        DETECTION_SUB,
			"auth0_client_id":     DETECTION_SUB,
			"auth0_client_secret": DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{"http://localhost:3000/callback"},
			AllowedLogoutURLs: []string{"http://localhost:3000"},
		},
	},

	// ==========================================
	// Native / Mobile Applications
	// ==========================================
	"native:flutter:none": {
		EnvValues: map[string]string{
			"domain":   DETECTION_SUB,
			"clientId": DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "native",
			Callbacks:         []string{DETECTION_SUB},
			AllowedLogoutURLs: []string{DETECTION_SUB},
		},
	},
	"native:react-native:none": {
		EnvValues: map[string]string{
			"AUTH0_DOMAIN":    DETECTION_SUB,
			"AUTH0_CLIENT_ID": DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "native",
			Callbacks:         []string{DETECTION_SUB},
			AllowedLogoutURLs: []string{DETECTION_SUB},
		},
	},
	"native:expo:none": {
		EnvValues: map[string]string{
			"EXPO_PUBLIC_AUTH0_DOMAIN":    DETECTION_SUB,
			"EXPO_PUBLIC_AUTH0_CLIENT_ID": DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "native",
			Callbacks:         []string{DETECTION_SUB},
			AllowedLogoutURLs: []string{DETECTION_SUB},
		},
	},
	"native:ionic-angular:none": {
		EnvValues: map[string]string{
			"domain":   DETECTION_SUB,
			"clientId": DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "native",
			Callbacks:         []string{DETECTION_SUB},
			AllowedLogoutURLs: []string{DETECTION_SUB},
		},
	},
	"native:ionic-react:vite": {
		EnvValues: map[string]string{
			"VITE_AUTH0_DOMAIN":    DETECTION_SUB,
			"VITE_AUTH0_CLIENT_ID": DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "native",
			Callbacks:         []string{DETECTION_SUB},
			AllowedLogoutURLs: []string{DETECTION_SUB},
		},
	},
	"native:ionic-vue:vite": {
		EnvValues: map[string]string{
			"VITE_AUTH0_DOMAIN":    DETECTION_SUB,
			"VITE_AUTH0_CLIENT_ID": DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "native",
			Callbacks:         []string{DETECTION_SUB},
			AllowedLogoutURLs: []string{DETECTION_SUB},
		},
	},
	"native:dotnet-mobile:none": {
		EnvValues: map[string]string{
			"Auth0:Domain":   DETECTION_SUB,
			"Auth0:ClientId": DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "native",
			Callbacks:         []string{DETECTION_SUB},
			AllowedLogoutURLs: []string{DETECTION_SUB},
		},
	},
	"native:maui:none": {
		EnvValues: map[string]string{
			"Auth0:Domain":   DETECTION_SUB,
			"Auth0:ClientId": DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "native",
			Callbacks:         []string{DETECTION_SUB},
			AllowedLogoutURLs: []string{DETECTION_SUB},
		},
	},
	"native:wpf-winforms:none": {
		EnvValues: map[string]string{
			"Auth0:Domain":       DETECTION_SUB,
			"Auth0:ClientId":     DETECTION_SUB,
			"Auth0:ClientSecret": DETECTION_SUB,
		},
		RequestParams: RequestParams{
			AppType:           "native",
			Callbacks:         []string{DETECTION_SUB},
			AllowedLogoutURLs: []string{DETECTION_SUB},
		},
	},
}
