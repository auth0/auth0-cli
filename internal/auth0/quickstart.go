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
	// "github.com/auth0/auth0-cli/internal/prompt"
	// "github.com/auth0/auth0-cli/internal/prompt"

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

const DetectionSub = "DETECTION_SUB"

type FileOutputStrategy struct {
	Path   string
	Format string
}

type RequestParams struct {
	AppType           string
	Callbacks         []string
	AllowedLogoutURLs []string
	WebOrigins        []string
	Name              string
}

type AppConfig struct {
	EnvValues     map[string]string
	RequestParams RequestParams
	Strategy      FileOutputStrategy
}

var QuickstartConfigs = map[string]AppConfig{

	// ==========================================
	// Single Page Applications (SPA)
	// ==========================================
	"spa:react:vite": {
		EnvValues: map[string]string{
			"VITE_AUTH0_DOMAIN":    DetectionSub,
			"VITE_AUTH0_CLIENT_ID": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "spa",
			Callbacks:         []string{"http://localhost:5173/callback"},
			AllowedLogoutURLs: []string{"http://localhost:5173"},
			WebOrigins:        []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: ".env", Format: "dotenv"},
	},
	"spa:angular:none": {
		EnvValues: map[string]string{
			"domain":   DetectionSub,
			"clientId": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "spa",
			Callbacks:         []string{"http://localhost:4200/callback"},
			AllowedLogoutURLs: []string{"http://localhost:4200"},
			WebOrigins:        []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: "src/environments/environment.ts", Format: "ts"},
	},
	"spa:vue:vite": {
		EnvValues: map[string]string{
			"VITE_AUTH0_DOMAIN":    DetectionSub,
			"VITE_AUTH0_CLIENT_ID": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "spa",
			Callbacks:         []string{"http://localhost:5173/callback"},
			AllowedLogoutURLs: []string{"http://localhost:5173"},
			WebOrigins:        []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: ".env", Format: "dotenv"},
	},
	"spa:svelte:vite": {
		EnvValues: map[string]string{
			"VITE_AUTH0_DOMAIN":    DetectionSub,
			"VITE_AUTH0_CLIENT_ID": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "spa",
			Callbacks:         []string{"http://localhost:5173/callback"},
			AllowedLogoutURLs: []string{"http://localhost:5173"},
			WebOrigins:        []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: ".env", Format: "dotenv"},
	},
	"spa:vanilla-javascript:vite": {
		EnvValues: map[string]string{
			"VITE_AUTH0_DOMAIN":    DetectionSub,
			"VITE_AUTH0_CLIENT_ID": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "spa",
			Callbacks:         []string{"http://localhost:5173/callback"},
			AllowedLogoutURLs: []string{"http://localhost:5173"},
			WebOrigins:        []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: ".env", Format: "dotenv"},
	},
	"spa:flutter-web:none": {
		EnvValues: map[string]string{
			"domain":   DetectionSub,
			"clientId": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "spa",
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			WebOrigins:        []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: "lib/auth_config.dart", Format: "dart"},
	},

	// ==========================================
	// Regular Web Applications
	// ==========================================
	"regular:nextjs:none": {
		EnvValues: map[string]string{
			"AUTH0_DOMAIN":        DetectionSub,
			"AUTH0_CLIENT_ID":     DetectionSub,
			"AUTH0_CLIENT_SECRET": DetectionSub,
			"AUTH0_SECRET":        DetectionSub,
			"APP_BASE_URL":        DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{"http://localhost:3000/callback"},
			AllowedLogoutURLs: []string{"http://localhost:3000"},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: ".env", Format: "dotenv"},
	},
	"regular:nuxt:none": {
		EnvValues: map[string]string{
			"NUXT_AUTH0_DOMAIN":         DetectionSub,
			"NUXT_AUTH0_CLIENT_ID":      DetectionSub,
			"NUXT_AUTH0_CLIENT_SECRET":  DetectionSub,
			"NUXT_AUTH0_SESSION_SECRET": DetectionSub,
			"NUXT_AUTH0_APP_BASE_URL":   DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{"http://localhost:3000/callback"},
			AllowedLogoutURLs: []string{"http://localhost:3000"},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: ".env", Format: "dotenv"},
	},
	"regular:fastify:none": {
		EnvValues: map[string]string{
			"AUTH0_DOMAIN":        DetectionSub,
			"AUTH0_CLIENT_ID":     DetectionSub,
			"AUTH0_CLIENT_SECRET": DetectionSub,
			"SESSION_SECRET":      DetectionSub,
			"APP_BASE_URL":        DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{"http://localhost:3000/callback"},
			AllowedLogoutURLs: []string{"http://localhost:3000"},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: ".env", Format: "dotenv"},
	},
	"regular:sveltekit:none": {
		EnvValues: map[string]string{
			"VITE_AUTH0_DOMAIN":    DetectionSub,
			"VITE_AUTH0_CLIENT_ID": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: ".env", Format: "dotenv"},
	},
	"regular:express:none": {
		EnvValues: map[string]string{
			"ISSUER_BASE_URL": DetectionSub,
			"CLIENT_ID":       DetectionSub,
			"SECRET":          DetectionSub,
			"BASE_URL":        DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{"http://localhost:3000/callback"},
			AllowedLogoutURLs: []string{"http://localhost:3000"},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: ".env", Format: "dotenv"},
	},
	"regular:hono:none": {
		EnvValues: map[string]string{
			"AUTH0_DOMAIN":                 DetectionSub,
			"AUTH0_CLIENT_ID":              DetectionSub,
			"AUTH0_CLIENT_SECRET":          DetectionSub,
			"AUTH0_SESSION_ENCRYPTION_KEY": DetectionSub,
			"BASE_URL":                     DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{"http://localhost:3000/callback"},
			AllowedLogoutURLs: []string{"http://localhost:3000"},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: ".env", Format: "dotenv"},
	},
	"regular:vanilla-python:none": {
		EnvValues: map[string]string{
			"AUTH0_DOMAIN":        DetectionSub,
			"AUTH0_CLIENT_ID":     DetectionSub,
			"AUTH0_CLIENT_SECRET": DetectionSub,
			"AUTH0_SECRET":        DetectionSub,
			"AUTH0_REDIRECT_URI":  DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{"http://localhost:5000/callback"},
			AllowedLogoutURLs: []string{"http://localhost:5000"},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: ".env", Format: "dotenv"},
	},
	"regular:django:none": {
		EnvValues: map[string]string{
			"AUTH0_DOMAIN":        DetectionSub,
			"AUTH0_CLIENT_ID":     DetectionSub,
			"AUTH0_CLIENT_SECRET": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: ".env", Format: "dotenv"},
	},
	"regular:vanilla-go:none": {
		EnvValues: map[string]string{
			"AUTH0_DOMAIN":        DetectionSub,
			"AUTH0_CLIENT_ID":     DetectionSub,
			"AUTH0_CLIENT_SECRET": DetectionSub,
			"AUTH0_CALLBACK_URL":  DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: ".env", Format: "dotenv"},
	},
	"regular:vanilla-java:maven": {
		EnvValues: map[string]string{
			"auth0.domain":       DetectionSub,
			"auth0.clientId":     DetectionSub,
			"auth0.clientSecret": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: "src/main/resources/application.properties", Format: "properties"},
	},
	"regular:java-ee:maven": {
		EnvValues: map[string]string{
			"auth0.domain":       DetectionSub,
			"auth0.clientId":     DetectionSub,
			"auth0.clientSecret": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: "src/main/resources/META-INF/microprofile-config.properties", Format: "properties"},
	},
	"regular:spring-boot:maven": {
		EnvValues: map[string]string{
			"okta.oauth2.issuer":        DetectionSub,
			"okta.oauth2.client-id":     DetectionSub,
			"okta.oauth2.client-secret": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{"http://localhost:8000/callback"},
			AllowedLogoutURLs: []string{"http://localhost:8000"},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: "src/main/resources/application.yml", Format: "yaml"},
	},
	"regular:aspnet-mvc:none": {
		EnvValues: map[string]string{
			"Auth0:Domain":       DetectionSub,
			"Auth0:ClientId":     DetectionSub,
			"Auth0:ClientSecret": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: "appsettings.json", Format: "json"},
	},
	"regular:aspnet-blazor:none": {
		EnvValues: map[string]string{
			"Auth0:Domain":   DetectionSub,
			"Auth0:ClientId": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: "appsettings.json", Format: "json"},
	},
	"regular:aspnet-owin:none": {
		EnvValues: map[string]string{
			"auth0:Domain":       DetectionSub,
			"auth0:ClientId":     DetectionSub,
			"auth0:ClientSecret": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: "Web.config", Format: "xml"},
	},
	"regular:vanilla-php:composer": {
		EnvValues: map[string]string{
			"AUTH0_DOMAIN":        DetectionSub,
			"AUTH0_CLIENT_ID":     DetectionSub,
			"AUTH0_CLIENT_SECRET": DetectionSub,
			"AUTH0_COOKIE_SECRET": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: ".env", Format: "dotenv"},
	},
	"regular:laravel:composer": {
		EnvValues: map[string]string{
			"AUTH0_DOMAIN":        DetectionSub,
			"AUTH0_CLIENT_ID":     DetectionSub,
			"AUTH0_CLIENT_SECRET": DetectionSub,
			"AUTH0_COOKIE_SECRET": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{"http://localhost:8000/callback"},
			AllowedLogoutURLs: []string{"http://localhost:8000"},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: ".env", Format: "dotenv"},
	},
	"regular:rails:none": {
		EnvValues: map[string]string{
			"auth0_domain":        DetectionSub,
			"auth0_client_id":     DetectionSub,
			"auth0_client_secret": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{"http://localhost:3000/callback"},
			AllowedLogoutURLs: []string{"http://localhost:3000"},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: ".env", Format: "dotenv"},
	},

	// ==========================================
	// Native / Mobile Applications
	// ==========================================
	"native:flutter:none": {
		EnvValues: map[string]string{
			"domain":   DetectionSub,
			"clientId": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "native",
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: "lib/auth_config.dart", Format: "dart"},
	},
	"native:react-native:none": {
		EnvValues: map[string]string{
			"AUTH0_DOMAIN":    DetectionSub,
			"AUTH0_CLIENT_ID": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "native",
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: ".env", Format: "dotenv"},
	},
	"native:expo:none": {
		EnvValues: map[string]string{
			"EXPO_PUBLIC_AUTH0_DOMAIN":    DetectionSub,
			"EXPO_PUBLIC_AUTH0_CLIENT_ID": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "native",
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: ".env", Format: "dotenv"},
	},
	"native:ionic-angular:none": {
		EnvValues: map[string]string{
			"domain":   DetectionSub,
			"clientId": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "native",
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: "src/environments/environment.ts", Format: "ts"},
	},
	"native:ionic-react:vite": {
		EnvValues: map[string]string{
			"VITE_AUTH0_DOMAIN":    DetectionSub,
			"VITE_AUTH0_CLIENT_ID": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "native",
			Callbacks:         []string{DetectionSub},
			Name:              DetectionSub,
			AllowedLogoutURLs: []string{DetectionSub},
		},
		Strategy: FileOutputStrategy{Path: ".env", Format: "dotenv"},
	},
	"native:ionic-vue:vite": {
		EnvValues: map[string]string{
			"VITE_AUTH0_DOMAIN":    DetectionSub,
			"VITE_AUTH0_CLIENT_ID": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "native",
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: ".env", Format: "dotenv"},
	},
	"native:dotnet-mobile:none": {
		EnvValues: map[string]string{
			"Auth0:Domain":   DetectionSub,
			"Auth0:ClientId": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "native",
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: "appsettings.json", Format: "json"},
	},
	"native:maui:none": {
		EnvValues: map[string]string{
			"Auth0:Domain":   DetectionSub,
			"Auth0:ClientId": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "native",
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: "appsettings.json", Format: "json"},
	},
	"native:wpf-winforms:none": {
		EnvValues: map[string]string{
			"Auth0:Domain":       DetectionSub,
			"Auth0:ClientId":     DetectionSub,
			"Auth0:ClientSecret": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "native",
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: "appsettings.json", Format: "json"},
	},
}
