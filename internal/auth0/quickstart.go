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
	"sort"
	"strings"
	"time"

	"github.com/auth0/go-auth0/management"

	"github.com/auth0/auth0-cli/internal/buildinfo"
	"github.com/auth0/auth0-cli/internal/utils"
)

const (
	quickstartsMetaURL            = "https://auth0.com/docs/meta/quickstarts"
	quickstartsOrg                = "auth0-samples"
	quickstartsDefaultCallbackURL = "https://YOUR_APP/callback"
)

const (
	quickstartHTTPTimeout = 30 * time.Second
	maxDownloadSize       = 100 * 1024 * 1024 // 100 MB.
)

var quickstartHTTPClient = &http.Client{Timeout: quickstartHTTPTimeout}

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

	response, err := quickstartHTTPClient.Do(request)
	if err != nil {
		return err
	}

	defer func() {
		_ = response.Body.Close()
	}()

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

	_, err = io.Copy(tmpFile, io.LimitReader(response.Body, maxDownloadSize))
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

	response, err := quickstartHTTPClient.Do(request)
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

const (
	// DetectionSub is replaced at runtime with baseURL+CallbackPath ("/callback" by default).
	DetectionSub = "DETECTION_SUB"
	// DetectionSubAsBase is replaced at runtime with just the baseURL (no path suffix).
	// Use this for SPA callback/logout URLs where the path is the app root.
	DetectionSubAsBase = "DETECTION_SUB_AS_BASE"
)

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
	// CallbackPath is the path suffix appended to baseURL when resolving DetectionSub
	// in Callbacks. Leave empty to use the default "/callback". Examples:
	//   "/api/auth/callback"  (Next.js)
	//   "/auth/callback"      (Fastify)
	CallbackPath string
}

type AppConfig struct {
	EnvValues     map[string]string
	RequestParams RequestParams
	Strategy      FileOutputStrategy
	// AudienceVar is the env variable name to add when --api is also specified.
	// It receives the API identifier (audience URL) as its value.
	// Leave empty for frameworks where the audience is not configured via an env file.
	AudienceVar string
}

// FrameworkBuildTools maps "type:framework" to the sorted list of build-tool
// variants from QuickstartConfigs (including the "none" sentinel).
// FrameworkSupportedBuildTools is the same map with "none" stripped, so it
// only contains frameworks that accept a --build-tool flag.
var (
	FrameworkBuildTools          map[string][]string
	FrameworkSupportedBuildTools map[string][]string
)

func init() {
	FrameworkBuildTools, FrameworkSupportedBuildTools = buildFrameworkBuildToolMaps(QuickstartConfigs)
}

// buildFrameworkBuildToolMaps walks configs once and returns FrameworkBuildTools
// (all variants) and FrameworkSupportedBuildTools (without the "none" sentinel).
// Malformed keys (not "type:framework:tool") are skipped.
func buildFrameworkBuildToolMaps(configs map[string]AppConfig) (all, supported map[string][]string) {
	all = make(map[string][]string, len(configs))
	supported = make(map[string][]string, len(configs))

	for k := range configs {
		parts := strings.SplitN(k, ":", 3)
		if len(parts) != 3 {
			continue
		}
		tf, tool := parts[0]+":"+parts[1], parts[2]
		all[tf] = append(all[tf], tool)
		if tool != "none" {
			supported[tf] = append(supported[tf], tool)
		}
	}

	for tf, tools := range all {
		if len(tools) > 1 {
			sort.Strings(tools)
		}
		all[tf] = tools
	}
	for tf, tools := range supported {
		if len(tools) > 1 {
			sort.Strings(tools)
		}
		supported[tf] = tools
	}
	return all, supported
}

// FrameworkBuildToolRequired reports whether a "type:framework" key requires
// --build-tool, i.e. is present in FrameworkSupportedBuildTools.
func FrameworkBuildToolRequired(typeFramework string) bool {
	_, ok := FrameworkSupportedBuildTools[typeFramework]
	return ok
}

// FrameworkBundleIDRequired lists native frameworks that need a resolved
// bundle/application ID (for example, "com.example.myapp") during project
// detection. Keep in sync with DetectProject BundleID assignments.
var FrameworkBundleIDRequired = map[string]bool{
	"native:flutter":       true,
	"native:react-native":  true,
	"native:android":       true,
	"native:ios-swift":     true,
	"native:ionic-angular": true,
	"native:ionic-react":   true,
	"native:ionic-vue":     true,
	"native:maui":          true,
	"native:dotnet-mobile": true,
}

// IsBundleIDRequired reports whether a "type:framework" key needs a native
// bundle / application identifier resolved during project detection or setup.
func IsBundleIDRequired(typeFramework string) bool {
	return FrameworkBundleIDRequired[typeFramework]
}

var QuickstartConfigs = map[string]AppConfig{

	// SPA (Single Page Applications).
	"spa:react:vite": {
		EnvValues: map[string]string{
			"VITE_AUTH0_DOMAIN":    DetectionSub,
			"VITE_AUTH0_CLIENT_ID": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "spa",
			Callbacks:         []string{DetectionSubAsBase},
			AllowedLogoutURLs: []string{DetectionSub},
			WebOrigins:        []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy:    FileOutputStrategy{Path: ".env", Format: "dotenv"},
		AudienceVar: "VITE_AUTH0_AUDIENCE",
	},
	"spa:angular:none": {
		EnvValues: map[string]string{
			"domain":   DetectionSub,
			"clientId": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "spa",
			Callbacks:         []string{DetectionSubAsBase},
			AllowedLogoutURLs: []string{DetectionSub},
			WebOrigins:        []string{DetectionSub},
			Name:              DetectionSub,
		},
		// Nests domain/clientId under auth0:{} (environment.auth0.domain / .clientId).
		Strategy: FileOutputStrategy{Path: "src/environments/environment.ts", Format: "angular-ts"},
	},
	"spa:vue:vite": {
		EnvValues: map[string]string{
			"VITE_AUTH0_DOMAIN":    DetectionSub,
			"VITE_AUTH0_CLIENT_ID": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "spa",
			Callbacks:         []string{DetectionSubAsBase},
			AllowedLogoutURLs: []string{DetectionSub},
			WebOrigins:        []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy:    FileOutputStrategy{Path: ".env", Format: "dotenv"},
		AudienceVar: "VITE_AUTH0_AUDIENCE",
	},
	"spa:svelte:vite": {
		EnvValues: map[string]string{
			"VITE_AUTH0_DOMAIN":    DetectionSub,
			"VITE_AUTH0_CLIENT_ID": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "spa",
			Callbacks:         []string{DetectionSubAsBase},
			AllowedLogoutURLs: []string{DetectionSub},
			WebOrigins:        []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy:    FileOutputStrategy{Path: ".env", Format: "dotenv"},
		AudienceVar: "VITE_AUTH0_AUDIENCE",
	},
	"spa:vanilla-javascript:vite": {
		EnvValues: map[string]string{
			"VITE_AUTH0_DOMAIN":    DetectionSub,
			"VITE_AUTH0_CLIENT_ID": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "spa",
			Callbacks:         []string{DetectionSubAsBase},
			AllowedLogoutURLs: []string{DetectionSub},
			WebOrigins:        []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy:    FileOutputStrategy{Path: ".env", Format: "dotenv"},
		AudienceVar: "VITE_AUTH0_AUDIENCE",
	},
	"spa:flutter-web:none": {
		EnvValues: map[string]string{
			"domain":   DetectionSub,
			"clientId": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "spa",
			Callbacks:         []string{DetectionSubAsBase},
			AllowedLogoutURLs: []string{DetectionSub},
			WebOrigins:        []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: "lib/auth_config.dart", Format: "dart"},
	},

	// Regular Web Applications.
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
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
			CallbackPath:      "/auth/callback",
		},
		Strategy:    FileOutputStrategy{Path: ".env", Format: "dotenv"},
		AudienceVar: "AUTH0_AUDIENCE",
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
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
			CallbackPath:      "/auth/callback",
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
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
			CallbackPath:      "/auth/callback",
		},
		Strategy: FileOutputStrategy{Path: ".env", Format: "dotenv"},
	},
	"regular:sveltekit:none": {
		EnvValues: map[string]string{
			"AUTH0_DOMAIN":        DetectionSub,
			"AUTH0_CLIENT_ID":     DetectionSub,
			"AUTH0_CLIENT_SECRET": DetectionSub,
			"AUTH0_SECRET":        DetectionSub,
			"APP_BASE_URL":        DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
			CallbackPath:      "/auth/callback",
		},
		Strategy: FileOutputStrategy{Path: ".env", Format: "dotenv"},
	},
	// SvelteKit with Vite: SSR requires a client secret regardless of build tool.
	"regular:sveltekit:vite": {
		EnvValues: map[string]string{
			"AUTH0_DOMAIN":        DetectionSub,
			"AUTH0_CLIENT_ID":     DetectionSub,
			"AUTH0_CLIENT_SECRET": DetectionSub,
			"AUTH0_SECRET":        DetectionSub,
			"APP_BASE_URL":        DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
			CallbackPath:      "/auth/callback",
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
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
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
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
			CallbackPath:      "/auth/callback",
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
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
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
			"com.auth0.domain":       DetectionSub,
			"com.auth0.clientId":     DetectionSub,
			"com.auth0.clientSecret": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: "src/main/webapp/WEB-INF/web.xml", Format: "webxml"},
	},
	"regular:java-ee:maven": {
		EnvValues: map[string]string{
			"auth0/domain":       DetectionSub,
			"auth0/clientId":     DetectionSub,
			"auth0/clientSecret": DetectionSub,
			// Fixed value for JNDI lookup by Auth0AuthenticationConfig.
			"auth0/scope": "openid profile email",
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
		},
		// JNDI env-entry elements in web.xml.
		Strategy: FileOutputStrategy{Path: "src/main/webapp/WEB-INF/web.xml", Format: "javaee-webxml"},
	},
	"regular:spring-boot:maven": {
		EnvValues: map[string]string{
			"okta.oauth2.issuer":        DetectionSub,
			"okta.oauth2.client-id":     DetectionSub,
			"okta.oauth2.client-secret": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
			// Okta-spring-boot-starter registers redirect under "oidc" registration ID.
			CallbackPath: "/login/oauth2/code/oidc",
		},
		Strategy: FileOutputStrategy{Path: "src/main/resources/application.yml", Format: "yaml"},
	},
	// Spring Boot with Gradle: generates application.yml with okta.oauth2.* properties.
	"regular:spring-boot:gradle": {
		EnvValues: map[string]string{
			"okta.oauth2.issuer":        DetectionSub,
			"okta.oauth2.client-id":     DetectionSub,
			"okta.oauth2.client-secret": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
			CallbackPath:      "/login/oauth2/code/okta",
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
			"AUTH0_BASE_URL":      DetectionSub,
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
			"AUTH0_BASE_URL":      DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
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
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
			CallbackPath:      "/auth/auth0/callback",
		},
		Strategy: FileOutputStrategy{Path: "config/auth0.yml", Format: "rails-yaml"},
	},
	"regular:jhipster:none": {
		EnvValues: map[string]string{
			"SPRING_SECURITY_OAUTH2_CLIENT_PROVIDER_OIDC_ISSUER_URI":        DetectionSub,
			"SPRING_SECURITY_OAUTH2_CLIENT_REGISTRATION_OIDC_CLIENT_ID":     DetectionSub,
			"SPRING_SECURITY_OAUTH2_CLIENT_REGISTRATION_OIDC_CLIENT_SECRET": DetectionSub,
			"JHIPSTER_SECURITY_OAUTH2_AUDIENCE":                             DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "regular_web",
			Callbacks:         []string{DetectionSub},
			AllowedLogoutURLs: []string{DetectionSub},
			Name:              DetectionSub,
			CallbackPath:      "/login/oauth2/code/oidc",
		},
		Strategy: FileOutputStrategy{Path: ".auth0.env", Format: "dotenv"},
	},

	// Native / Mobile Applications.
	// Callbacks are left empty when the custom URI scheme depends on a bundle ID
	// not known at setup time. Exceptions: Expo (exp://localhost:19000) and
	// Ionic/Capacitor (http://localhost).
	"native:flutter:none": {
		EnvValues: map[string]string{
			"domain":   DetectionSub,
			"clientId": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "native",
			Callbacks:         []string{},
			AllowedLogoutURLs: []string{},
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
			Callbacks:         []string{},
			AllowedLogoutURLs: []string{},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: ".env", Format: "dotenv"},
	},
	"native:expo:none": {
		EnvValues: map[string]string{
			"EXPO_PUBLIC_AUTH0_DOMAIN":    DetectionSub,
			"EXPO_PUBLIC_AUTH0_CLIENT_ID": DetectionSub,
		},
		// Expo Go uses exp://localhost:19000 as the standard redirect URI.
		RequestParams: RequestParams{
			AppType:           "native",
			Callbacks:         []string{"exp://localhost:19000"},
			AllowedLogoutURLs: []string{"exp://localhost:19000"},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: ".env", Format: "dotenv"},
	},
	"native:ionic-angular:none": {
		EnvValues: map[string]string{
			"domain":   DetectionSub,
			"clientId": DetectionSub,
		},
		// Capacitor intercepts http://localhost redirects in the WebView.
		RequestParams: RequestParams{
			AppType:           "native",
			Callbacks:         []string{"http://localhost"},
			AllowedLogoutURLs: []string{"http://localhost"},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: "src/environments/environment.ts", Format: "ts"},
	},
	"native:ionic-react:vite": {
		EnvValues: map[string]string{
			"VITE_AUTH0_DOMAIN":    DetectionSub,
			"VITE_AUTH0_CLIENT_ID": DetectionSub,
		},
		// Capacitor intercepts http://localhost redirects in the WebView.
		RequestParams: RequestParams{
			AppType:           "native",
			Callbacks:         []string{"http://localhost"},
			AllowedLogoutURLs: []string{"http://localhost"},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: ".env", Format: "dotenv"},
	},
	"native:ionic-vue:vite": {
		EnvValues: map[string]string{
			"VITE_AUTH0_DOMAIN":    DetectionSub,
			"VITE_AUTH0_CLIENT_ID": DetectionSub,
		},
		// Capacitor intercepts http://localhost redirects in the WebView.
		RequestParams: RequestParams{
			AppType:           "native",
			Callbacks:         []string{"http://localhost"},
			AllowedLogoutURLs: []string{"http://localhost"},
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
			Callbacks:         []string{},
			AllowedLogoutURLs: []string{},
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
			Callbacks:         []string{},
			AllowedLogoutURLs: []string{},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: "appsettings.json", Format: "json"},
	},
	"native:wpf-winforms:none": {
		EnvValues: map[string]string{
			"Auth0:Domain":   DetectionSub,
			"Auth0:ClientId": DetectionSub,
			// No Auth0:ClientSecret — WPF/WinForms apps are public native clients
			// that use Authorization Code + PKCE; the client secret is unused and
			// Auth0 returns an empty/placeholder value for native app types.
		},
		// WPF/WinForms uses the bare loopback http://localhost (no port, no path) per Auth0 docs.
		RequestParams: RequestParams{
			AppType:           "native",
			Callbacks:         []string{"http://localhost"},
			AllowedLogoutURLs: []string{"http://localhost"},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: "appsettings.json", Format: "json"},
	},

	"native:android:gradle": {
		EnvValues: map[string]string{
			"com_auth0_domain":    DetectionSub,
			"com_auth0_client_id": DetectionSub,
			// Com_auth0_scheme is always "https" for App Links (HTTPS callback scheme).
			"com_auth0_scheme": "https",
		},
		// Android uses App Links (https://<domain>/android/<packageName>/callback).
		// Package name is not known at setup time; user must add the URL in the Auth0 Dashboard.
		RequestParams: RequestParams{
			AppType:           "native",
			Callbacks:         []string{},
			AllowedLogoutURLs: []string{},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: "app/src/main/res/values/strings.xml", Format: "android-strings"},
	},
	"native:ios-swift:none": {
		EnvValues: map[string]string{
			"ClientId": DetectionSub,
			"Domain":   DetectionSub,
		},
		// IOS Swift uses universal links or custom URI scheme callbacks based on the bundle
		// identifier. Bundle ID is not known at setup time; user must add URL in Auth0 Dashboard.
		RequestParams: RequestParams{
			AppType:           "native",
			Callbacks:         []string{},
			AllowedLogoutURLs: []string{},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: "Auth0.plist", Format: "plist"},
	},

	// ==========================================
	// M2M apps use the client_credentials flow — no frontend, no port, no callback URLs.
	"m2m:none:none": {
		EnvValues: map[string]string{
			"AUTH0_DOMAIN":        DetectionSub,
			"AUTH0_CLIENT_ID":     DetectionSub,
			"AUTH0_CLIENT_SECRET": DetectionSub,
		},
		RequestParams: RequestParams{
			AppType:           "non_interactive",
			Callbacks:         []string{},
			AllowedLogoutURLs: []string{},
			Name:              DetectionSub,
		},
		Strategy: FileOutputStrategy{Path: ".env", Format: "dotenv"},
	},
}
