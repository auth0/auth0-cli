package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auth0/go-auth0/management"
	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/option"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
	"github.com/auth0/auth0-cli/internal/config"
	"github.com/auth0/auth0-cli/internal/display"
)

// -- DetectionSubAsBase --.

// TestResolveRequestParams_DetectionSubAsBase verifies that DetectionSubAsBase in
// callbacks resolves to baseURL with no path suffix (unlike DetectionSub which
// appends "/callback").
func TestResolveRequestParams_DetectionSubAsBase(t *testing.T) {
	t.Parallel()

	t.Run("callback resolves to baseURL only", func(t *testing.T) {
		t.Parallel()
		req := auth0.RequestParams{
			AppType:           "spa",
			Callbacks:         []string{auth0.DetectionSubAsBase},
			AllowedLogoutURLs: []string{auth0.DetectionSub},
			WebOrigins:        []string{auth0.DetectionSub},
			Name:              auth0.DetectionSub,
		}
		got := resolveRequestParams(req, "MyApp", 5173)
		assert.Equal(t, []string{"http://localhost:5173"}, got.Callbacks, "callback should be baseURL with no path")
		assert.Equal(t, []string{"http://localhost:5173"}, got.AllowedLogoutURLs)
		assert.Equal(t, []string{"http://localhost:5173"}, got.WebOrigins)
	})

	t.Run("DetectionSubAsBase in logoutURLs is not substituted (only DetectionSub is)", func(t *testing.T) {
		t.Parallel()
		req := auth0.RequestParams{
			AllowedLogoutURLs: []string{auth0.DetectionSubAsBase},
		}
		got := resolveRequestParams(req, "App", 3000)
		assert.Equal(t, []string{auth0.DetectionSubAsBase}, got.AllowedLogoutURLs)
	})
}

// TestResolveRequestParams_CallbackPath verifies that a custom CallbackPath
// overrides the default "/callback" suffix when resolving DetectionSub in
// Callbacks.
func TestResolveRequestParams_CallbackPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		callbackPath string
		port         int
		want         string
	}{
		{"/api/auth/callback", 3000, "http://localhost:3000/api/auth/callback"},
		{"/auth/callback", 3000, "http://localhost:3000/auth/callback"},
		{"/login/oauth2/code/oidc", 8080, "http://localhost:8080/login/oauth2/code/oidc"},
		{"", 3000, "http://localhost:3000/callback"}, // Default when empty.
	}

	for _, tc := range cases {
		tc := tc
		t.Run(fmt.Sprintf("%s:%d", tc.callbackPath, tc.port), func(t *testing.T) {
			t.Parallel()
			req := auth0.RequestParams{
				Callbacks:    []string{auth0.DetectionSub},
				CallbackPath: tc.callbackPath,
			}
			got := resolveRequestParams(req, "App", tc.port)
			require.Len(t, got.Callbacks, 1)
			assert.Equal(t, tc.want, got.Callbacks[0])
		})
	}
}

// -- resolveRequestParams with QuickstartConfigs --.

// TestResolveRequestParams_AllQuickstartConfigs verifies that each entry in
// auth0.QuickstartConfigs produces the correct resolved callback and logout URLs
// when given a specific port, matching the patterns required by each framework.
func TestResolveRequestParams_AllQuickstartConfigs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		configKey      string
		port           int
		wantCallbacks  []string
		wantLogoutURLs []string
		wantWebOrigins []string
		wantAppType    string
	}{
		// SPA: callback = just baseURL (no /callback suffix per Auth0 SPA SDK usage).
		{"spa:react:vite", 5173,
			[]string{"http://localhost:5173"},
			[]string{"http://localhost:5173"},
			[]string{"http://localhost:5173"}, "spa"},
		{"spa:vue:vite", 5173,
			[]string{"http://localhost:5173"},
			[]string{"http://localhost:5173"},
			[]string{"http://localhost:5173"}, "spa"},
		{"spa:svelte:vite", 5173,
			[]string{"http://localhost:5173"},
			[]string{"http://localhost:5173"},
			[]string{"http://localhost:5173"}, "spa"},
		{"spa:vanilla-javascript:vite", 5173,
			[]string{"http://localhost:5173"},
			[]string{"http://localhost:5173"},
			[]string{"http://localhost:5173"}, "spa"},
		// Webpack SPAs: webpack-dev-server default port 8080.
		{"spa:react:webpack", 8080,
			[]string{"http://localhost:8080"},
			[]string{"http://localhost:8080"},
			[]string{"http://localhost:8080"}, "spa"},
		{"spa:vanilla-javascript:webpack", 8080,
			[]string{"http://localhost:8080"},
			[]string{"http://localhost:8080"},
			[]string{"http://localhost:8080"}, "spa"},
		{"spa:angular:none", 4200,
			[]string{"http://localhost:4200"},
			[]string{"http://localhost:4200"},
			[]string{"http://localhost:4200"}, "spa"},
		{"spa:flutter-web:none", 3000,
			[]string{"http://localhost:3000"},
			[]string{"http://localhost:3000"},
			[]string{"http://localhost:3000"}, "spa"},
		// Regular web: framework-specific callback paths.
		{"regular:nextjs:none", 3000,
			[]string{"http://localhost:3000/auth/callback"},
			[]string{"http://localhost:3000"}, nil, "regular_web"},
		{"regular:fastify:none", 3000,
			[]string{"http://localhost:3000/auth/callback"},
			[]string{"http://localhost:3000"}, nil, "regular_web"},
		{"regular:tanstack-start:none", 3000,
			[]string{"http://localhost:3000/auth/callback"},
			[]string{"http://localhost:3000"}, nil, "regular_web"},
		{"regular:nuxt:none", 3000,
			[]string{"http://localhost:3000/auth/callback"},
			[]string{"http://localhost:3000"}, nil, "regular_web"},
		{"regular:express:none", 3000,
			[]string{"http://localhost:3000/callback"},
			[]string{"http://localhost:3000"}, nil, "regular_web"},
		{"regular:hono:none", 3000,
			[]string{"http://localhost:3000/auth/callback"},
			[]string{"http://localhost:3000"}, nil, "regular_web"},
		{"regular:vanilla-python:none", 3000,
			[]string{"http://localhost:3000/callback"},
			[]string{"http://localhost:3000"}, nil, "regular_web"},
		// Flask detection sets port 5000 (Flask's historical default).
		{"regular:vanilla-python:none", 5000,
			[]string{"http://localhost:5000/callback"},
			[]string{"http://localhost:5000"}, nil, "regular_web"},
		{"regular:sveltekit:none", 3000,
			[]string{"http://localhost:3000/auth/callback"},
			[]string{"http://localhost:3000"}, nil, "regular_web"},
		{"regular:sveltekit:vite", 3000,
			[]string{"http://localhost:3000/auth/callback"},
			[]string{"http://localhost:3000"}, nil, "regular_web"},
		{"regular:django:none", 3000,
			[]string{"http://localhost:3000/callback"},
			[]string{"http://localhost:3000"}, nil, "regular_web"},
		// Django detection (manage.py or requirements.txt) sets port 8000 (Django dev server default).
		{"regular:django:none", 8000,
			[]string{"http://localhost:8000/callback"},
			[]string{"http://localhost:8000"}, nil, "regular_web"},
		{"regular:vanilla-go:none", 3000,
			[]string{"http://localhost:3000/callback"},
			[]string{"http://localhost:3000"}, nil, "regular_web"},
		{"regular:spring-boot:maven", 3000,
			[]string{"http://localhost:3000/login/oauth2/code/okta"},
			[]string{"http://localhost:3000"}, nil, "regular_web"},
		{"regular:spring-boot:gradle", 3000,
			[]string{"http://localhost:3000/login/oauth2/code/okta"},
			[]string{"http://localhost:3000"}, nil, "regular_web"},
		{"regular:laravel:composer", 8000,
			[]string{"http://localhost:8000/callback"},
			[]string{"http://localhost:8000"}, nil, "regular_web"},
		{"regular:rails:none", 3000,
			[]string{"http://localhost:3000/auth/auth0/callback"},
			[]string{"http://localhost:3000"}, nil, "regular_web"},
		{"regular:jhipster:none", 8080,
			[]string{"http://localhost:8080/login/oauth2/code/oidc"},
			[]string{"http://localhost:8080"}, nil, "regular_web"},
		{"regular:aspnet-mvc:none", 5000,
			[]string{"http://localhost:5000/callback"},
			[]string{"http://localhost:5000"}, nil, "regular_web"},
		{"regular:aspnet-blazor:none", 3000,
			[]string{"http://localhost:3000/callback"},
			[]string{"http://localhost:3000"}, nil, "regular_web"},
		{"regular:aspnet-owin:none", 5000,
			[]string{"http://localhost:5000/callback"},
			[]string{"http://localhost:5000"}, nil, "regular_web"},
		{"regular:vanilla-php:composer", 3000,
			[]string{"http://localhost:3000/callback"},
			[]string{"http://localhost:3000"}, nil, "regular_web"},
		{"regular:vanilla-java:maven", 8080,
			[]string{"http://localhost:8080/callback"},
			[]string{"http://localhost:8080"}, nil, "regular_web"},
		{"regular:java-ee:maven", 8080,
			[]string{"http://localhost:8080/callback"},
			[]string{"http://localhost:8080"}, nil, "regular_web"},
		// Native: static callback URLs — no DetectionSub substitution.
		// Flutter and React Native use custom URI scheme callbacks (bundle-ID-specific);
		// the bundle identifier is unknown at setup time so callbacks are empty.
		{"native:flutter:none", 0,
			[]string{},
			[]string{}, nil, "native"},
		{"native:react-native:none", 0,
			[]string{},
			[]string{}, nil, "native"},
		// Expo uses the standard Expo Go redirect URI.
		{"native:expo:none", 0,
			[]string{"exp://localhost:19000"},
			[]string{"exp://localhost:19000"}, nil, "native"},
		// Ionic (Capacitor) intercepts http://localhost redirects in the WebView.
		{"native:ionic-angular:none", 0,
			[]string{"http://localhost"},
			[]string{"http://localhost"}, nil, "native"},
		{"native:ionic-react:vite", 0,
			[]string{"http://localhost"},
			[]string{"http://localhost"}, nil, "native"},
		{"native:ionic-vue:vite", 0,
			[]string{"http://localhost"},
			[]string{"http://localhost"}, nil, "native"},
		// .NET Mobile and MAUI use custom URI scheme callbacks (bundle-ID-specific).
		{"native:dotnet-mobile:none", 0,
			[]string{},
			[]string{}, nil, "native"},
		{"native:maui:none", 0,
			[]string{},
			[]string{}, nil, "native"},
		// WPF/WinForms uses the bare loopback http://localhost per Auth0 docs.
		{"native:wpf-winforms:none", 0,
			[]string{"http://localhost"},
			[]string{"http://localhost"}, nil, "native"},
		// Android and iOS Swift: custom URI scheme callbacks; unknown at setup time.
		{"native:android:gradle", 0, []string{}, []string{}, nil, "native"},
		{"native:ios-swift:none", 0, []string{}, []string{}, nil, "native"},
		// M2M: no URLs.
		{"m2m:none:none", 0, []string{}, []string{}, nil, "non_interactive"},
		// Custom port propagates.
		{"spa:react:vite", 8080,
			[]string{"http://localhost:8080"},
			[]string{"http://localhost:8080"},
			[]string{"http://localhost:8080"}, "spa"},
		{"regular:nextjs:none", 8080,
			[]string{"http://localhost:8080/auth/callback"},
			[]string{"http://localhost:8080"}, nil, "regular_web"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.configKey, func(t *testing.T) {
			t.Parallel()
			config, ok := auth0.QuickstartConfigs[tc.configKey]
			require.True(t, ok, "config key %q not found", tc.configKey)

			got := resolveRequestParams(config.RequestParams, "TestApp", tc.port)

			assert.Equal(t, tc.wantAppType, got.AppType)
			assert.Equal(t, tc.wantCallbacks, got.Callbacks)
			assert.Equal(t, tc.wantLogoutURLs, got.AllowedLogoutURLs)
			if tc.wantWebOrigins != nil {
				assert.Equal(t, tc.wantWebOrigins, got.WebOrigins)
			}
		})
	}
}

// -- generateAndWriteQuickstartConfig with QuickstartConfigs --.

// TestGenerateAndWriteQuickstartConfig_AllQuickstartConfigs verifies the env
// file content generated for every application type in auth0.QuickstartConfigs.
func TestGenerateAndWriteQuickstartConfig_AllQuickstartConfigs(t *testing.T) {
	t.Parallel()

	const domain = "test.auth0.com"
	const cidVal = "test-client-id"
	const csecVal = "test-client-secret"
	cid, csec := cidVal, csecVal
	client := &management.Client{ClientID: &cid, ClientSecret: &csec}

	tests := []struct {
		configKey    string
		port         int
		wantFileName string
		wantKeys     []string
		wantValues   map[string]string
	}{
		// SPA.
		{"spa:react:vite", 5173, ".env",
			[]string{"VITE_AUTH0_DOMAIN", "VITE_AUTH0_CLIENT_ID"},
			map[string]string{"VITE_AUTH0_DOMAIN": domain, "VITE_AUTH0_CLIENT_ID": cidVal}},
		{"spa:vue:vite", 5173, ".env",
			[]string{"VITE_AUTH0_DOMAIN", "VITE_AUTH0_CLIENT_ID"},
			map[string]string{"VITE_AUTH0_DOMAIN": domain, "VITE_AUTH0_CLIENT_ID": cidVal}},
		{"spa:svelte:vite", 5173, ".env",
			[]string{"VITE_AUTH0_DOMAIN", "VITE_AUTH0_CLIENT_ID"},
			map[string]string{"VITE_AUTH0_DOMAIN": domain, "VITE_AUTH0_CLIENT_ID": cidVal}},
		{"spa:vanilla-javascript:vite", 5173, ".env",
			[]string{"VITE_AUTH0_DOMAIN", "VITE_AUTH0_CLIENT_ID"},
			map[string]string{"VITE_AUTH0_DOMAIN": domain, "VITE_AUTH0_CLIENT_ID": cidVal}},
		{"spa:react:webpack", 8080, ".env",
			[]string{"AUTH0_DOMAIN", "AUTH0_CLIENT_ID"},
			map[string]string{"AUTH0_DOMAIN": domain, "AUTH0_CLIENT_ID": cidVal}},
		{"spa:vanilla-javascript:webpack", 8080, ".env",
			[]string{"AUTH0_DOMAIN", "AUTH0_CLIENT_ID"},
			map[string]string{"AUTH0_DOMAIN": domain, "AUTH0_CLIENT_ID": cidVal}},
		{"spa:angular:none", 4200, "environment.ts",
			// Angular-ts wraps values under auth0:{} matching the official Angular quickstart.
			[]string{"auth0:", "domain:", "clientId:", "production:"},
			map[string]string{domain: domain, cidVal: cidVal}},
		{"spa:flutter-web:none", 3000, "auth_config.dart",
			[]string{"domain", "clientId"},
			map[string]string{"domain": domain, "clientId": cidVal}},
		// Regular web.
		{"regular:nextjs:none", 3000, ".env",
			[]string{"AUTH0_DOMAIN", "AUTH0_CLIENT_ID", "AUTH0_CLIENT_SECRET", "AUTH0_SECRET", "APP_BASE_URL"},
			map[string]string{"AUTH0_DOMAIN": domain, "AUTH0_CLIENT_ID": cidVal, "AUTH0_CLIENT_SECRET": csecVal, "APP_BASE_URL": "http://localhost:3000"}},
		{"regular:tanstack-start:none", 3000, ".env",
			[]string{"AUTH0_DOMAIN", "AUTH0_CLIENT_ID", "AUTH0_CLIENT_SECRET", "AUTH0_SECRET", "APP_BASE_URL"},
			map[string]string{"AUTH0_DOMAIN": domain, "AUTH0_CLIENT_ID": cidVal, "AUTH0_CLIENT_SECRET": csecVal, "APP_BASE_URL": "http://localhost:3000"}},
		{"regular:fastify:none", 3000, ".env",
			[]string{"AUTH0_DOMAIN", "AUTH0_CLIENT_ID", "AUTH0_CLIENT_SECRET", "SESSION_SECRET", "APP_BASE_URL"},
			map[string]string{"AUTH0_DOMAIN": domain, "AUTH0_CLIENT_ID": cidVal, "AUTH0_CLIENT_SECRET": csecVal, "APP_BASE_URL": "http://localhost:3000"}},
		{"regular:nuxt:none", 3000, ".env",
			[]string{"NUXT_AUTH0_DOMAIN", "NUXT_AUTH0_CLIENT_ID", "NUXT_AUTH0_CLIENT_SECRET", "NUXT_AUTH0_SESSION_SECRET", "NUXT_AUTH0_APP_BASE_URL"},
			map[string]string{"NUXT_AUTH0_DOMAIN": domain, "NUXT_AUTH0_CLIENT_ID": cidVal, "NUXT_AUTH0_APP_BASE_URL": "http://localhost:3000"}},
		{"regular:express:none", 3000, ".env",
			[]string{"ISSUER_BASE_URL", "CLIENT_ID", "SECRET", "BASE_URL"},
			map[string]string{"ISSUER_BASE_URL": "https://" + domain, "CLIENT_ID": cidVal, "BASE_URL": "http://localhost:3000"}},
		{"regular:hono:none", 3000, ".env",
			[]string{"AUTH0_DOMAIN", "AUTH0_CLIENT_ID", "AUTH0_CLIENT_SECRET", "AUTH0_SESSION_ENCRYPTION_KEY", "BASE_URL"},
			map[string]string{"AUTH0_DOMAIN": domain, "AUTH0_CLIENT_ID": cidVal, "BASE_URL": "http://localhost:3000"}},
		{"regular:vanilla-python:none", 3000, ".env",
			[]string{"AUTH0_DOMAIN", "AUTH0_CLIENT_ID", "AUTH0_CLIENT_SECRET", "AUTH0_SECRET", "AUTH0_REDIRECT_URI"},
			map[string]string{"AUTH0_DOMAIN": domain, "AUTH0_REDIRECT_URI": "http://localhost:3000/callback"}},
		// Spring-boot uses YAML: dot-keys are nested; verify both structure and value.
		{"regular:spring-boot:maven", 8080, "application.yml",
			[]string{"okta:", "oauth2:", "issuer: https://", "client-id:", "client-secret:"},
			nil},
		{"regular:spring-boot:gradle", 8080, "application.yml",
			[]string{"okta:", "oauth2:", "issuer: https://", "client-id:", "client-secret:"},
			nil},
		{"regular:laravel:composer", 8000, ".env",
			[]string{"AUTH0_DOMAIN", "AUTH0_CLIENT_ID", "AUTH0_CLIENT_SECRET", "AUTH0_COOKIE_SECRET", "AUTH0_BASE_URL"},
			map[string]string{"AUTH0_DOMAIN": domain, "AUTH0_CLIENT_ID": cidVal, "AUTH0_BASE_URL": "http://localhost:8000"}},
		{"regular:rails:none", 3000, "auth0.yml",
			[]string{"development:", "auth0_domain:", "auth0_client_id:", "auth0_client_secret:"},
			map[string]string{"auth0_domain": domain, "auth0_client_id": cidVal}},
		{"regular:jhipster:none", 8080, ".auth0.env",
			[]string{"SPRING_SECURITY_OAUTH2_CLIENT_PROVIDER_OIDC_ISSUER_URI", "SPRING_SECURITY_OAUTH2_CLIENT_REGISTRATION_OIDC_CLIENT_ID", "SPRING_SECURITY_OAUTH2_CLIENT_REGISTRATION_OIDC_CLIENT_SECRET", "JHIPSTER_SECURITY_OAUTH2_AUDIENCE"},
			map[string]string{
				"SPRING_SECURITY_OAUTH2_CLIENT_PROVIDER_OIDC_ISSUER_URI":        "https://" + domain + "/",
				"SPRING_SECURITY_OAUTH2_CLIENT_REGISTRATION_OIDC_CLIENT_ID":     cidVal,
				"SPRING_SECURITY_OAUTH2_CLIENT_REGISTRATION_OIDC_CLIENT_SECRET": csecVal,
				"JHIPSTER_SECURITY_OAUTH2_AUDIENCE":                             "https://" + domain + "/api/v2/",
			}},
		{"regular:aspnet-mvc:none", 3000, "appsettings.json",
			[]string{"Domain", "ClientId", "ClientSecret"}, nil},
		{"regular:aspnet-blazor:none", 3000, "appsettings.json",
			[]string{"Domain", "ClientId", "ClientSecret"}, nil},
		{"regular:aspnet-owin:none", 3000, "Web.config",
			[]string{"auth0:Domain", "auth0:ClientId"}, nil},
		{"regular:vanilla-php:composer", 3000, ".env",
			[]string{"AUTH0_DOMAIN", "AUTH0_CLIENT_ID", "AUTH0_CLIENT_SECRET", "AUTH0_COOKIE_SECRET", "AUTH0_BASE_URL"},
			map[string]string{"AUTH0_DOMAIN": domain, "AUTH0_BASE_URL": "http://localhost:3000"}},
		{"regular:vanilla-java:maven", 8080, "web.xml",
			[]string{"com.auth0.domain", "com.auth0.clientId", "com.auth0.clientSecret"},
			map[string]string{"com.auth0.domain": domain, "com.auth0.clientId": cidVal}},
		{"regular:java-ee:maven", 8080, "web.xml",
			// Javaee-webxml writes JNDI env-entry elements matching the official Java EE quickstart.
			// Keys use slash separators (auth0/domain) per the Java EE JNDI naming convention.
			[]string{"env-entry", "auth0/domain", "auth0/clientId", "auth0/clientSecret", "auth0/scope"},
			map[string]string{"auth0/domain": domain, "auth0/clientId": cidVal}},
		{"regular:sveltekit:none", 3000, ".env",
			[]string{"AUTH0_DOMAIN", "AUTH0_CLIENT_ID", "AUTH0_CLIENT_SECRET", "AUTH0_SECRET", "APP_BASE_URL"},
			map[string]string{"AUTH0_DOMAIN": domain, "AUTH0_CLIENT_ID": cidVal, "AUTH0_CLIENT_SECRET": csecVal, "APP_BASE_URL": "http://localhost:3000"}},
		// SvelteKit + Vite uses the same server-side config as sveltekit:none.
		{"regular:sveltekit:vite", 3000, ".env",
			[]string{"AUTH0_DOMAIN", "AUTH0_CLIENT_ID", "AUTH0_CLIENT_SECRET", "AUTH0_SECRET", "APP_BASE_URL"},
			map[string]string{"AUTH0_DOMAIN": domain, "AUTH0_CLIENT_ID": cidVal, "AUTH0_CLIENT_SECRET": csecVal, "APP_BASE_URL": "http://localhost:3000"}},
		{"regular:django:none", 3000, ".env",
			[]string{"AUTH0_DOMAIN", "AUTH0_CLIENT_ID", "AUTH0_CLIENT_SECRET"},
			map[string]string{"AUTH0_DOMAIN": domain, "AUTH0_CLIENT_ID": cidVal, "AUTH0_CLIENT_SECRET": csecVal}},
		{"regular:vanilla-go:none", 3000, ".env",
			[]string{"AUTH0_DOMAIN", "AUTH0_CLIENT_ID", "AUTH0_CLIENT_SECRET", "AUTH0_CALLBACK_URL"},
			map[string]string{"AUTH0_DOMAIN": domain, "AUTH0_CALLBACK_URL": "http://localhost:3000/callback"}},
		// Native.
		{"native:flutter:none", 0, "auth_config.dart",
			[]string{"domain", "clientId"},
			map[string]string{"domain": domain, "clientId": cidVal}},
		{"native:react-native:none", 0, ".env",
			[]string{"AUTH0_DOMAIN", "AUTH0_CLIENT_ID"},
			map[string]string{"AUTH0_DOMAIN": domain, "AUTH0_CLIENT_ID": cidVal}},
		{"native:expo:none", 0, ".env",
			[]string{"EXPO_PUBLIC_AUTH0_DOMAIN", "EXPO_PUBLIC_AUTH0_CLIENT_ID"},
			map[string]string{"EXPO_PUBLIC_AUTH0_DOMAIN": domain, "EXPO_PUBLIC_AUTH0_CLIENT_ID": cidVal}},
		{"native:ionic-angular:none", 0, "environment.ts",
			[]string{"domain", "clientId"}, nil},
		{"native:ionic-react:vite", 0, ".env",
			[]string{"VITE_AUTH0_DOMAIN", "VITE_AUTH0_CLIENT_ID"}, nil},
		{"native:ionic-vue:vite", 0, ".env",
			[]string{"VITE_AUTH0_DOMAIN", "VITE_AUTH0_CLIENT_ID"}, nil},
		{"native:dotnet-mobile:none", 0, "appsettings.json",
			[]string{"Domain", "ClientId"}, nil},
		{"native:maui:none", 0, "appsettings.json",
			[]string{"Domain", "ClientId"}, nil},
		// WPF/WinForms are public native clients (PKCE) — no client secret is written.
		{"native:wpf-winforms:none", 0, "appsettings.json",
			[]string{"Domain", "ClientId"}, nil},
		{"native:android:gradle", 0, "strings.xml",
			[]string{"com_auth0_domain", "com_auth0_client_id", "com_auth0_scheme"},
			map[string]string{"com_auth0_domain": domain, "com_auth0_client_id": cidVal, "com_auth0_scheme": "https"}},
		{"native:ios-swift:none", 0, "Auth0.plist",
			[]string{"ClientId", "Domain"},
			map[string]string{"ClientId": cidVal, "Domain": domain}},
		// M2M.
		{"m2m:none:none", 0, ".env",
			[]string{"AUTH0_DOMAIN", "AUTH0_CLIENT_ID", "AUTH0_CLIENT_SECRET"},
			map[string]string{"AUTH0_DOMAIN": domain, "AUTH0_CLIENT_ID": cidVal, "AUTH0_CLIENT_SECRET": csecVal}},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.configKey, func(t *testing.T) {
			t.Parallel()

			config, ok := auth0.QuickstartConfigs[tc.configKey]
			require.True(t, ok, "config key %q not found", tc.configKey)

			dir := t.TempDir()
			strategy := auth0.FileOutputStrategy{
				Path:   filepath.Join(dir, config.Strategy.Path),
				Format: config.Strategy.Format,
			}
			subDir := filepath.Dir(strategy.Path)
			if subDir != dir {
				require.NoError(t, os.MkdirAll(subDir, 0755))
			}

			err := generateAndWriteQuickstartConfig(&strategy, config.EnvValues, domain, client, tc.port)
			require.NoError(t, err)

			fileName := filepath.Base(strategy.Path)
			assert.Equal(t, tc.wantFileName, fileName)
			assert.FileExists(t, strategy.Path)

			content, err := os.ReadFile(strategy.Path)
			require.NoError(t, err)
			contentStr := string(content)

			for _, key := range tc.wantKeys {
				assert.Contains(t, contentStr, key, "key %q missing from %s", key, fileName)
			}
			for key, wantVal := range tc.wantValues {
				assert.Contains(t, contentStr, wantVal,
					"value %q for key %q missing from %s", wantVal, key, fileName)
			}
		})
	}
}

// -- generateClient with QuickstartConfigs --.

// TestGenerateClient_AllQuickstartConfigs verifies the management.Client fields
// produced by generateClient for every app type in auth0.QuickstartConfigs.
func TestGenerateClient_AllQuickstartConfigs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		configKey         string
		port              int
		wantAppType       string
		wantCallbacksLen  int
		wantCallback      string
		wantLogoutURLsLen int
		wantWebOriginsLen int
	}{
		// SPA: callback = baseURL (no /callback suffix).
		{"spa:react:vite", 5173, "spa", 1, "http://localhost:5173", 1, 1},
		{"spa:vue:vite", 5173, "spa", 1, "http://localhost:5173", 1, 1},
		{"spa:svelte:vite", 5173, "spa", 1, "http://localhost:5173", 1, 1},
		{"spa:vanilla-javascript:vite", 5173, "spa", 1, "http://localhost:5173", 1, 1},
		{"spa:react:webpack", 8080, "spa", 1, "http://localhost:8080", 1, 1},
		{"spa:vanilla-javascript:webpack", 8080, "spa", 1, "http://localhost:8080", 1, 1},
		{"spa:angular:none", 4200, "spa", 1, "http://localhost:4200", 1, 1},
		{"spa:flutter-web:none", 3000, "spa", 1, "http://localhost:3000", 1, 1},
		// Regular web: framework-specific paths.
		{"regular:nextjs:none", 3000, "regular_web", 1, "http://localhost:3000/auth/callback", 1, 0},
		{"regular:fastify:none", 3000, "regular_web", 1, "http://localhost:3000/auth/callback", 1, 0},
		{"regular:tanstack-start:none", 3000, "regular_web", 1, "http://localhost:3000/auth/callback", 1, 0},
		{"regular:nuxt:none", 3000, "regular_web", 1, "http://localhost:3000/auth/callback", 1, 0},
		{"regular:express:none", 3000, "regular_web", 1, "http://localhost:3000/callback", 1, 0},
		{"regular:hono:none", 3000, "regular_web", 1, "http://localhost:3000/auth/callback", 1, 0},
		{"regular:vanilla-python:none", 3000, "regular_web", 1, "http://localhost:3000/callback", 1, 0},
		// Flask detection sets port 5000 (Flask's historical default).
		{"regular:vanilla-python:none", 5000, "regular_web", 1, "http://localhost:5000/callback", 1, 0},
		{"regular:sveltekit:none", 3000, "regular_web", 1, "http://localhost:3000/auth/callback", 1, 0},
		{"regular:sveltekit:vite", 3000, "regular_web", 1, "http://localhost:3000/auth/callback", 1, 0},
		{"regular:django:none", 3000, "regular_web", 1, "http://localhost:3000/callback", 1, 0},
		// Django detection (manage.py or requirements.txt) sets port 8000 (Django dev server default).
		{"regular:django:none", 8000, "regular_web", 1, "http://localhost:8000/callback", 1, 0},
		{"regular:vanilla-go:none", 3000, "regular_web", 1, "http://localhost:3000/callback", 1, 0},
		{"regular:laravel:composer", 8000, "regular_web", 1, "http://localhost:8000/callback", 1, 0},
		{"regular:rails:none", 3000, "regular_web", 1, "http://localhost:3000/auth/auth0/callback", 1, 0},
		{"regular:jhipster:none", 8080, "regular_web", 1, "http://localhost:8080/login/oauth2/code/oidc", 1, 0},
		{"regular:aspnet-mvc:none", 3000, "regular_web", 1, "http://localhost:3000/callback", 1, 0},
		{"regular:aspnet-blazor:none", 3000, "regular_web", 1, "http://localhost:3000/callback", 1, 0},
		{"regular:aspnet-owin:none", 3000, "regular_web", 1, "http://localhost:3000/callback", 1, 0},
		{"regular:vanilla-php:composer", 3000, "regular_web", 1, "http://localhost:3000/callback", 1, 0},
		{"regular:vanilla-java:maven", 8080, "regular_web", 1, "http://localhost:8080/callback", 1, 0},
		{"regular:java-ee:maven", 8080, "regular_web", 1, "http://localhost:8080/callback", 1, 0},
		{"regular:spring-boot:maven", 8080, "regular_web", 1, "http://localhost:8080/login/oauth2/code/okta", 1, 0},
		{"regular:spring-boot:gradle", 8080, "regular_web", 1, "http://localhost:8080/login/oauth2/code/okta", 1, 0},
		// Native: static callback URLs appropriate per framework.
		// Flutter and React Native use bundle-ID-specific custom URI schemes; unknown at setup time.
		{"native:flutter:none", 0, "native", 0, "", 0, 0},
		{"native:react-native:none", 0, "native", 0, "", 0, 0},
		// Expo uses the standard Expo Go redirect URI.
		{"native:expo:none", 0, "native", 1, "exp://localhost:19000", 1, 0},
		// Ionic (Capacitor) intercepts http://localhost redirects.
		{"native:ionic-angular:none", 0, "native", 1, "http://localhost", 1, 0},
		{"native:ionic-react:vite", 0, "native", 1, "http://localhost", 1, 0},
		{"native:ionic-vue:vite", 0, "native", 1, "http://localhost", 1, 0},
		// .NET Mobile and MAUI use bundle-ID-specific custom URI schemes; unknown at setup time.
		{"native:dotnet-mobile:none", 0, "native", 0, "", 0, 0},
		{"native:maui:none", 0, "native", 0, "", 0, 0},
		// WPF/WinForms uses the bare loopback http://localhost per Auth0 docs.
		{"native:wpf-winforms:none", 0, "native", 1, "http://localhost", 1, 0},
		// Android and iOS Swift use bundle-ID-specific custom URI schemes; unknown at setup time.
		{"native:android:gradle", 0, "native", 0, "", 0, 0},
		{"native:ios-swift:none", 0, "native", 0, "", 0, 0},
		// M2M: no callbacks.
		{"m2m:none:none", 0, "non_interactive", 0, "", 0, 0},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.configKey, func(t *testing.T) {
			t.Parallel()

			config, ok := auth0.QuickstartConfigs[tc.configKey]
			require.True(t, ok)

			c, err := generateClient(SetupInputs{Name: "Test App", Port: tc.port}, config.RequestParams)
			require.NoError(t, err)

			assert.Equal(t, tc.wantAppType, c.GetAppType())
			assert.Len(t, c.GetCallbacks(), tc.wantCallbacksLen)
			if tc.wantCallback != "" && len(c.GetCallbacks()) > 0 {
				assert.Equal(t, tc.wantCallback, c.GetCallbacks()[0])
			}
			assert.Len(t, c.GetAllowedLogoutURLs(), tc.wantLogoutURLsLen)
			assert.Len(t, c.GetWebOrigins(), tc.wantWebOriginsLen)
			assert.True(t, c.GetOIDCConformant())
			assert.NotNil(t, c.ClientMetadata)
		})
	}
}

// -- APP_BASE_URL reflects the user-specified port --.

func TestGenerateAndWriteQuickstartConfig_PortInBaseURL(t *testing.T) {
	t.Parallel()

	for _, configKey := range []string{"regular:nextjs:none", "regular:fastify:none", "regular:express:none", "regular:tanstack-start:none"} {
		t.Run(configKey, func(t *testing.T) {
			t.Parallel()

			config := auth0.QuickstartConfigs[configKey]
			dir := t.TempDir()
			strategy := auth0.FileOutputStrategy{Path: filepath.Join(dir, ".env"), Format: "dotenv"}
			cid, csec := "cid", "csec"
			client := &management.Client{ClientID: &cid, ClientSecret: &csec}

			err := generateAndWriteQuickstartConfig(&strategy, config.EnvValues, "example.auth0.com", client, 8080)
			require.NoError(t, err)

			content, err := os.ReadFile(strategy.Path)
			require.NoError(t, err)
			assert.Contains(t, string(content), "8080",
				"%s: port 8080 should appear in the generated file", configKey)
		})
	}
}

// -- Generated secrets (AUTH0_SECRET / SESSION_SECRET) are non-empty --.

func TestGenerateAndWriteQuickstartConfig_SecretsNonEmpty(t *testing.T) {
	t.Parallel()

	cid, csec := "cid", "csec"
	client := &management.Client{ClientID: &cid, ClientSecret: &csec}

	for _, configKey := range []string{"regular:nextjs:none", "regular:fastify:none", "regular:sveltekit:none", "regular:sveltekit:vite", "regular:tanstack-start:none"} {
		t.Run(configKey, func(t *testing.T) {
			t.Parallel()

			config := auth0.QuickstartConfigs[configKey]
			dir := t.TempDir()
			strategy := auth0.FileOutputStrategy{Path: filepath.Join(dir, ".env"), Format: "dotenv"}

			err := generateAndWriteQuickstartConfig(&strategy, config.EnvValues, "example.auth0.com", client, 3000)
			require.NoError(t, err)

			content, err := os.ReadFile(strategy.Path)
			require.NoError(t, err)

			for _, line := range strings.Split(string(content), "\n") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) != 2 {
					continue
				}
				key, val := parts[0], parts[1]
				if key == "AUTH0_SECRET" || key == "SESSION_SECRET" {
					assert.NotEmpty(t, val, "key %q should be non-empty", key)
				}
			}
		})
	}
}

// TestReplaceDetectionSub_AllQuickstartConfigsCovered verifies that every env
// key used in any QuickstartConfig (including those added via init()) is handled
// by replaceDetectionSub. This test is intentionally dynamic — it iterates over
// auth0.QuickstartConfigs at runtime so that newly added configs are automatically
// covered without requiring a change to the test itself.
//
// If this test fails, a new env key was added to quickstart.go without a
// corresponding case in the replaceDetectionSub switch in quickstarts.go.
func TestReplaceDetectionSub_AllQuickstartConfigsCovered(t *testing.T) {
	t.Parallel()

	cid, csec := "cid", "csec"
	client := &management.Client{ClientID: &cid, ClientSecret: &csec}

	for configKey, config := range auth0.QuickstartConfigs {
		configKey := configKey
		config := config
		t.Run(configKey, func(t *testing.T) {
			t.Parallel()

			_, err := replaceDetectionSub(config.EnvValues, "example.auth0.com", client, 3000)
			require.NoError(t, err,
				"config %q: env key not covered by replaceDetectionSub switch — add a case for it in quickstarts.go",
				configKey,
			)
		})
	}
}

// TestNoInputWithTypeRequiresFramework verifies that getQuickstartConfigKey
// does not error on missing framework in non-interactive mode — the framework
// prompt is skipped by the Flag wrapper (shouldAsk returns false when canPrompt
// is false). The guard that prevents a missing framework from reaching this
// function lives in the command's RunE, not here.
func TestNoInputWithTypeRequiresFramework(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		appType   string
		framework string
		wantErr   bool
	}{
		{
			name:      "spa without framework skips prompt and returns no error",
			appType:   "spa",
			framework: "",
			wantErr:   false,
		},
		{
			name:      "regular without framework skips prompt and returns no error",
			appType:   "regular",
			framework: "",
			wantErr:   false,
		},
		{
			name:      "spa with framework succeeds",
			appType:   "spa",
			framework: "react",
			wantErr:   false,
		},
		{
			name:      "m2m never needs framework",
			appType:   "m2m",
			framework: "",
			wantErr:   false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			inputs := SetupInputs{
				App:       true,
				Type:      tc.appType,
				Framework: tc.framework,
			}
			_, _, err := getQuickstartConfigKey(&cobra.Command{}, &inputs)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateAPIIdentifier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		identifier string
		wantErr    bool
	}{
		{
			name:       "valid http URL",
			identifier: "http://example.com/api",
			wantErr:    false,
		},
		{
			name:       "valid https URL",
			identifier: "https://my-api.example.com",
			wantErr:    false,
		},
		{
			// ParseRequestURI succeeds for valid URLs regardless of length, so
			// the err==nil branch returns nil before the length check is reached.
			name:       "valid URL exactly 24 characters",
			identifier: "https://foo.example.com/",
			wantErr:    false,
		},
		{
			// Both conditions must be true to trigger an error: the string must
			// fail ParseRequestURI AND be exactly 24 characters long.  A plain
			// 24-character ID (no scheme) satisfies both.
			name:       "24-character non-URL identifier",
			identifier: "abc123def456ghi789jkl012",
			wantErr:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateAPIIdentifier(tc.identifier)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// -- createQuickstartApp happy-path --.

func TestCreateQuickstartApp_SPA_React(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientAPI := mock.NewMockClientAPI(ctrl)
	clientAPI.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, c *management.Client, _ ...management.RequestOption) error {
			// Simulate the management API assigning a client ID.
			id := "test-client-id"
			c.ClientID = &id
			return nil
		})

	dir := t.TempDir()
	tenantDomain := "test.auth0.com"
	testCLI := &cli{
		renderer: &display.Renderer{MessageWriter: &bytes.Buffer{}, ResultWriter: &bytes.Buffer{}},
		api:      &auth0.API{Client: clientAPI},
		Config: config.Config{
			DefaultTenant: tenantDomain,
			Tenants: map[string]config.Tenant{
				tenantDomain: {Domain: tenantDomain},
			},
		},
		tenant: tenantDomain,
	}

	inputs := SetupInputs{
		App:       true,
		Name:      "My SPA",
		Type:      "spa",
		Framework: "react",
		BuildTool: "vite",
		Port:      5173,
	}

	// Override the working directory so generateAndWriteQuickstartConfig writes to a temp dir.
	oldWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	defer func() { _ = os.Chdir(oldWD) }()

	clientID, err := createQuickstartApp(&cobra.Command{}, testCLI, inputs, "spa:react:vite", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "test-client-id", clientID)

	// Config file should have been written.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.NotEmpty(t, entries, "expected env config file to be written")
}

func TestCreateQuickstartApp_UnsupportedKey(t *testing.T) {
	t.Parallel()

	testCLI := &cli{renderer: &display.Renderer{MessageWriter: &bytes.Buffer{}, ResultWriter: &bytes.Buffer{}}}
	_, err := createQuickstartApp(&cobra.Command{}, testCLI, SetupInputs{}, "unknown:framework:none", nil, nil)
	assert.ErrorContains(t, err, "unsupported quickstart arguments")
}

// -- applyDetectionToInputs --.

// TestApplyDetectionToInputs verifies that applyDetectionToInputs correctly copies
// detection fields into inputs, preserving any fields that were already set.
func TestApplyDetectionToInputs(t *testing.T) {
	t.Parallel()

	t.Run("populates empty inputs from detection", func(t *testing.T) {
		t.Parallel()
		d := DetectionResult{
			Type: "regular", BuildTool: "vite",
			BundleID: "com.example.app",
		}
		got := SetupInputs{}
		applyDetectionToInputs(&got, d)
		assert.Equal(t, "regular", got.Type)
		assert.Equal(t, "vite", got.BuildTool)
		assert.Equal(t, "com.example.app", got.BundleID)
		assert.Empty(t, got.Framework, "framework must not be set by applyDetectionToInputs")
	})

	t.Run("preserves explicitly set fields", func(t *testing.T) {
		t.Parallel()
		d := DetectionResult{Type: "regular"}
		got := SetupInputs{Type: "spa", Port: 5173, Name: "explicit-name"}
		applyDetectionToInputs(&got, d)
		assert.Equal(t, "spa", got.Type, "explicit type should not be overwritten")
		assert.Equal(t, 5173, got.Port, "explicit port should not be overwritten")
		assert.Equal(t, "explicit-name", got.Name, "explicit name should not be overwritten")
	})
}

// TestAmbiguousDetection_NoInputMode_UsesFirstCandidate verifies that when detection
// finds multiple candidates and prompting is disabled, the first candidate is selected
// without hanging.
func TestAmbiguousDetection_NoInputMode_UsesFirstCandidate(t *testing.T) {
	t.Parallel()

	detection := DetectionResult{
		Type:                "regular",
		AmbiguousFrameworks: []string{"express", "hono"},
		Detected:            true,
	}

	var inputs SetupInputs
	applyDetectionToInputs(&inputs, detection)

	// Simulate no-input mode: pick first candidate when framework is empty.
	if inputs.Framework == "" {
		inputs.Framework = detection.AmbiguousFrameworks[0]
	}

	assert.Equal(t, "express", inputs.Framework)
	assert.Equal(t, "regular", inputs.Type)
}

// TestAmbiguousDetection_NoInput_IntegrationFlow verifies the full flow:
// real package.json with two candidates -> ambiguous detection -> no-input mode
// selects the first candidate -> createQuickstartApp succeeds with the resolved inputs.
func TestAmbiguousDetection_NoInput_IntegrationFlow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientAPI := mock.NewMockClientAPI(ctrl)
	clientAPI.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, c *management.Client, _ ...management.RequestOption) error {
			id := "test-client-id"
			c.ClientID = &id
			return nil
		})

	dir := t.TempDir()
	pkgJSON := `{"name":"my-app","dependencies":{"express":"^4","hono":"^3"}}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0600))

	// Step 1: DetectProject finds ambiguous candidates from the real filesystem.
	detection := DetectProject(dir)
	require.True(t, detection.Detected, "detection should have fired")
	require.Greater(t, len(detection.AmbiguousFrameworks), 1, "should have multiple candidates")

	// Step 2: In no-input mode, pick the first candidate (same logic as RunE).
	inputs := SetupInputs{App: true}
	applyDetectionToInputs(&inputs, detection)
	if inputs.Framework == "" {
		inputs.Framework = detection.AmbiguousFrameworks[0]
	}

	// Step 3: Resolve the config key.
	qsConfigKey, _, err := getQuickstartConfigKey(&cobra.Command{}, &inputs)
	require.NoError(t, err)

	// Step 4: Verify the resolved framework is the first ambiguous candidate.
	assert.Equal(t, detection.AmbiguousFrameworks[0], inputs.Framework)
	assert.Equal(t, "regular", inputs.Type)

	// Step 5: Create the app end-to-end using the resolved inputs.
	tenantDomain := "test.auth0.com"
	testCLI := &cli{
		renderer: &display.Renderer{MessageWriter: &bytes.Buffer{}, ResultWriter: &bytes.Buffer{}},
		api:      &auth0.API{Client: clientAPI},
		tenant:   tenantDomain,
	}

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	defer func() { _ = os.Chdir(origDir) }()

	clientID, err := createQuickstartApp(&cobra.Command{}, testCLI, inputs, qsConfigKey, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "test-client-id", clientID)
}

// -- createQuickstartAPI happy-path --.

func TestCreateQuickstartAPI_CreatesResourceServerAndGrant(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	rsAPI := mock.NewMockResourceServerAPI(ctrl)
	rsAPI.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, rs *management.ResourceServer, _ ...management.RequestOption) error {
			id := "rs-id-1"
			rs.ID = &id
			return nil
		})

	var grantBody map[string]json.RawMessage
	grantAPI := mock.NewMockClientGrantAPIV3(ctrl)
	grantAPI.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *managementv3.CreateClientGrantRequestContent, _ ...option.RequestOption) (*managementv3.CreateClientGrantResponseContent, error) {
			b, err := json.Marshal(req)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(b, &grantBody))
			return nil, nil
		})

	testCLI := &cli{
		renderer: &display.Renderer{MessageWriter: &bytes.Buffer{}, ResultWriter: &bytes.Buffer{}},
		api: &auth0.API{
			ResourceServer: rsAPI,
		},
		apiv3: &auth0.APIV3{
			ClientGrant: grantAPI,
		},
	}

	inputs := SetupInputs{
		API:           true,
		Name:          "My App",
		Identifier:    "https://my-api",
		SigningAlg:    "RS256",
		TokenLifetime: "86400",
		LinkedAppID:   "linked-app-client-id",
	}

	err := createQuickstartAPI(context.Background(), testCLI, inputs, nil)
	assert.NoError(t, err)

	// SetScope must be called explicitly so an empty scope list is still sent.
	require.Contains(t, grantBody, "scope", "grant request must always carry the scope field")
	assert.JSONEq(t, `[]`, string(grantBody["scope"]))
	assert.JSONEq(t, `"linked-app-client-id"`, string(grantBody["client_id"]))
	assert.JSONEq(t, `"https://my-api"`, string(grantBody["audience"]))
}

func TestCreateQuickstartAPI_NoLinkedApp_SkipsGrant(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	rsAPI := mock.NewMockResourceServerAPI(ctrl)
	rsAPI.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, rs *management.ResourceServer, _ ...management.RequestOption) error {
			id := "rs-id-2"
			rs.ID = &id
			return nil
		})

	// No grant creation expected when linkedAppClientID is empty.
	grantAPI := mock.NewMockClientGrantAPIV3(ctrl)

	testCLI := &cli{
		renderer: &display.Renderer{MessageWriter: &bytes.Buffer{}, ResultWriter: &bytes.Buffer{}},
		api: &auth0.API{
			ResourceServer: rsAPI,
		},
		apiv3: &auth0.APIV3{
			ClientGrant: grantAPI,
		},
	}

	inputs := SetupInputs{
		API:        true,
		Identifier: "https://my-api",
		SigningAlg: "RS256",
	}

	err := createQuickstartAPI(context.Background(), testCLI, inputs, nil)
	assert.NoError(t, err)
}

// -- qs setup test helpers --.

const qsTestTenant = "test.auth0.com"

// newQSSetupCmd builds the real `qs setup` command bound to c and parses args so
// flag-presence checks (IsSet) behave as in a real run. It is never executed.
func newQSSetupCmd(t *testing.T, c *cli, args ...string) *cobra.Command {
	t.Helper()
	cmd := setupQuickstartCmd(c)
	require.NoError(t, cmd.ParseFlags(args))
	cmd.SetContext(context.Background())
	return cmd
}

// newQSTestCLI returns a cli wired to the given mocks with buffered writers.
func newQSTestCLI(format display.OutputFormat, agentMode bool, api *auth0.API, apiv3 *auth0.APIV3) (*cli, *bytes.Buffer, *bytes.Buffer) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	return &cli{
		renderer: &display.Renderer{
			MessageWriter: stderr,
			ResultWriter:  stdout,
			Format:        format,
			AgentMode:     agentMode,
		},
		api:    api,
		apiv3:  apiv3,
		tenant: qsTestTenant,
	}, stdout, stderr
}

// expectClientCreate stubs exactly one Client.Create that assigns an ID and secret.
func expectClientCreate(clientAPI *mock.MockClientAPI, clientID, secret string) {
	clientAPI.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, c *management.Client, _ ...management.RequestOption) error {
			c.ClientID = &clientID
			c.ClientSecret = &secret
			return nil
		}).
		Times(1)
}

// decodeSingleJSON asserts out holds exactly one JSON document and returns it raw.
func decodeSingleJSON(t *testing.T, out []byte) map[string]json.RawMessage {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(out))
	var doc map[string]json.RawMessage
	require.NoError(t, dec.Decode(&doc), "stdout must be a JSON object, got: %q", string(out))
	var extra interface{}
	assert.ErrorIs(t, dec.Decode(&extra), io.EOF, "stdout must contain exactly one JSON document")
	return doc
}

func writeQSFile(t *testing.T, path, content string, perm os.FileMode) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte(content), perm))
	require.NoError(t, os.Chmod(path, perm)) // Defeat umask so mode assertions are exact.
}

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	return info.Mode().Perm()
}

// -- #9 config write: mergeDotenv --.

func TestMergeDotenv(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		existing string
		values   map[string]string
		want     string
	}{
		{
			name:     "replaces owned key in place and keeps unrelated lines, comments and blanks in order",
			existing: "# app config\nFOO=bar\n# AUTH0_DOMAIN=commented\nAUTH0_DOMAIN=old\n\nAUTH0_DOMAIN_EXTRA=keep\nBAZ=qux\n",
			values:   map[string]string{"AUTH0_DOMAIN": "new.auth0.com"},
			want:     "# app config\nFOO=bar\n# AUTH0_DOMAIN=commented\nAUTH0_DOMAIN=\"new.auth0.com\"\n\nAUTH0_DOMAIN_EXTRA=keep\nBAZ=qux\n",
		},
		{
			name:     "appends missing owned keys at the end, sorted",
			existing: "FOO=bar\n",
			values:   map[string]string{"B_KEY": "2", "A_KEY": "1"},
			want:     "FOO=bar\nA_KEY=\"1\"\nB_KEY=\"2\"\n",
		},
		{
			name:     "replaces and appends together",
			existing: "FOO=bar\nAUTH0_CLIENT_ID=old\n",
			values:   map[string]string{"AUTH0_CLIENT_ID": "cid", "AUTH0_DOMAIN": "d"},
			want:     "FOO=bar\nAUTH0_CLIENT_ID=\"cid\"\nAUTH0_DOMAIN=\"d\"\n",
		},
		{
			// D2: the export prefix is kept on replaced owned keys.
			name:     "export prefix is preserved on a replaced owned key",
			existing: "export AUTH0_DOMAIN=old\nexport FOO=bar\n",
			values:   map[string]string{"AUTH0_DOMAIN": "new"},
			want:     "export AUTH0_DOMAIN=\"new\"\nexport FOO=bar\n",
		},
		{
			name:     "later duplicates of an owned key are dropped",
			existing: "AUTH0_DOMAIN=a\nFOO=bar\nAUTH0_DOMAIN=b\nexport AUTH0_DOMAIN=c\n",
			values:   map[string]string{"AUTH0_DOMAIN": "new"},
			want:     "AUTH0_DOMAIN=\"new\"\nFOO=bar\n",
		},
		{
			name:     "duplicate unrelated keys are left untouched",
			existing: "FOO=1\nFOO=2\n",
			values:   map[string]string{"A": "x"},
			want:     "FOO=1\nFOO=2\nA=\"x\"\n",
		},
		{
			name:     "existing content without trailing newline gets a separator",
			existing: "FOO=bar",
			values:   map[string]string{"A": "1"},
			want:     "FOO=bar\nA=\"1\"\n",
		},
		{
			name:     "empty existing file",
			existing: "",
			values:   map[string]string{"B": "2", "A": "1"},
			want:     "A=\"1\"\nB=\"2\"\n",
		},
		{
			name:     "unrelated CRLF lines are preserved byte-for-byte",
			existing: "FOO=bar\r\nBAZ=qux\r\n",
			values:   map[string]string{"A": "1"},
			want:     "FOO=bar\r\nBAZ=qux\r\nA=\"1\"\n",
		},
		{
			name:     "owned key with spaces around = is matched",
			existing: "AUTH0_DOMAIN = old\nFOO=bar\n",
			values:   map[string]string{"AUTH0_DOMAIN": "new"},
			want:     "AUTH0_DOMAIN=\"new\"\nFOO=bar\n",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := mergeDotenv([]byte(tc.existing), tc.values)
			assert.Equal(t, tc.want, string(got))
		})
	}
}

// -- #9 config write: pre-flight plan --.

func TestPlanQuickstartConfigWrite(t *testing.T) {
	t.Parallel()

	t.Run("missing file is created with 0600", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), ".env")
		for _, force := range []bool{false, true} {
			for _, canPromptFlag := range []bool{false, true} {
				plan, err := planQuickstartConfigWrite(auth0.FileOutputStrategy{Path: path, Format: "dotenv"}, force, canPromptFlag)
				require.NoError(t, err)
				assert.Equal(t, qsConfigCreated, plan.Action)
				assert.Equal(t, os.FileMode(0600), plan.Perm)
				assert.False(t, plan.NeedsConfirm)
			}
		}
	})

	t.Run("missing non-dotenv file is created without refusal", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "src", "environments", "environment.ts")
		plan, err := planQuickstartConfigWrite(auth0.FileOutputStrategy{Path: path, Format: "angular-ts"}, false, false)
		require.NoError(t, err)
		assert.Equal(t, qsConfigCreated, plan.Action)
	})

	t.Run("existing dotenv is merged only when prompting is unavailable", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), ".env")
		writeQSFile(t, path, "FOO=bar\n", 0640)
		tests := []struct {
			force, canPrompt bool
			wantAction       string
			wantConfirm      bool
		}{
			{force: false, canPrompt: false, wantAction: qsConfigMerged},
			{force: true, canPrompt: false, wantAction: qsConfigMerged},
			{force: false, canPrompt: true, wantConfirm: true},
			{force: true, canPrompt: true, wantAction: qsConfigOverwritten},
		}
		for _, tt := range tests {
			plan, err := planQuickstartConfigWrite(auth0.FileOutputStrategy{Path: path, Format: "dotenv"}, tt.force, tt.canPrompt)
			require.NoError(t, err, "force=%v canPrompt=%v", tt.force, tt.canPrompt)
			assert.Equal(t, tt.wantConfirm, plan.NeedsConfirm, "force=%v canPrompt=%v", tt.force, tt.canPrompt)
			if !tt.wantConfirm {
				assert.Equal(t, tt.wantAction, plan.Action, "force=%v canPrompt=%v", tt.force, tt.canPrompt)
			}
			assert.Equal(t, os.FileMode(0640), plan.Perm)
		}
	})

	t.Run("existing non-dotenv without force and no prompt refuses with file_exists", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "environment.ts")
		writeQSFile(t, path, "old", 0644)
		_, err := planQuickstartConfigWrite(auth0.FileOutputStrategy{Path: path, Format: "angular-ts"}, false, false)
		require.Error(t, err)
		assert.Equal(t, "usage", errorClass(err))
		assert.Equal(t, "file_exists", errorReason(err))
		assert.Contains(t, err.Error(), path)
		assert.Contains(t, err.Error(), "--force")
	})

	t.Run("existing non-dotenv with force is overwritten", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "environment.ts")
		writeQSFile(t, path, "old", 0644)
		for _, canPromptFlag := range []bool{false, true} {
			plan, err := planQuickstartConfigWrite(auth0.FileOutputStrategy{Path: path, Format: "angular-ts"}, true, canPromptFlag)
			require.NoError(t, err)
			assert.Equal(t, qsConfigOverwritten, plan.Action)
			assert.False(t, plan.NeedsConfirm)
			assert.Equal(t, os.FileMode(0644), plan.Perm)
		}
	})

	t.Run("existing non-dotenv when prompting is possible needs confirmation", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "Auth0.plist")
		writeQSFile(t, path, "old", 0644)
		plan, err := planQuickstartConfigWrite(auth0.FileOutputStrategy{Path: path, Format: "plist"}, false, true)
		require.NoError(t, err)
		assert.True(t, plan.NeedsConfirm)
		assert.Equal(t, path, plan.Path)
		assert.Equal(t, "plist", plan.Format)
	})

	t.Run("target path is a directory", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), ".env")
		require.NoError(t, os.MkdirAll(path, 0755))
		_, err := planQuickstartConfigWrite(auth0.FileOutputStrategy{Path: path, Format: "dotenv"}, true, false)
		assert.Error(t, err)
	})
}

// TestConfirmQuickstartConfigOverwrite stubs the package-level prompt; not parallel.
func TestConfirmQuickstartConfigOverwrite(t *testing.T) {
	orig := qsConfirmOverwrite
	t.Cleanup(func() { qsConfirmOverwrite = orig })

	for _, tc := range []struct {
		answer     bool
		wantAction string
	}{
		{true, qsConfigOverwritten},
		{false, qsConfigSkipped},
	} {
		var gotMsg string
		qsConfirmOverwrite = func(msg string) bool {
			gotMsg = msg
			return tc.answer
		}
		plan := &qsConfigWritePlan{Path: "src/environments/environment.ts", Format: "angular-ts", NeedsConfirm: true}
		confirmQuickstartConfigOverwrite(plan)

		assert.Equal(t, tc.wantAction, plan.Action)
		assert.False(t, plan.NeedsConfirm, "NeedsConfirm must be cleared")
		assert.Contains(t, gotMsg, "environment.ts")
		assert.Contains(t, gotMsg, "backed up")
	}
}

func TestNextBackupPath(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "environment.ts")
	writeQSFile(t, path, "x", 0644)

	got, err := nextBackupPath(path)
	require.NoError(t, err)
	assert.Equal(t, path+".auth0-backup", got)

	writeQSFile(t, path+".auth0-backup", "b0", 0644)
	got, err = nextBackupPath(path)
	require.NoError(t, err)
	assert.Equal(t, path+".auth0-backup.1", got)

	writeQSFile(t, path+".auth0-backup.1", "b1", 0644)
	got, err = nextBackupPath(path)
	require.NoError(t, err)
	assert.Equal(t, path+".auth0-backup.2", got)
}

func TestBackupQuickstartConfigFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "Web.config")
	writeQSFile(t, path, "<configuration/>", 0640)

	backup, err := backupQuickstartConfigFile(path)
	require.NoError(t, err)
	assert.Equal(t, path+".auth0-backup", backup)
	content, err := os.ReadFile(backup)
	require.NoError(t, err)
	assert.Equal(t, "<configuration/>", string(content))
	assert.Equal(t, os.FileMode(0640), fileMode(t, backup), "backup keeps the source permission bits")

	backup2, err := backupQuickstartConfigFile(path)
	require.NoError(t, err)
	assert.Equal(t, path+".auth0-backup.1", backup2)
}

// -- #9 config write: writeQuickstartConfig --.

func TestWriteQuickstartConfig_DotenvMerge(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), ".env")
	writeQSFile(t, path, "DATABASE_URL=postgres://localhost/db\n# keep me\nAUTH0_DOMAIN=old.auth0.com\n", 0644)

	c, _, _ := newQSTestCLI("", false, nil, nil)
	out, err := c.writeQuickstartConfig(&cobra.Command{}, map[string]string{
		"AUTH0_DOMAIN":    "new.auth0.com",
		"AUTH0_CLIENT_ID": "cid",
	}, &auth0.FileOutputStrategy{Path: path, Format: "dotenv"}, nil)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, qsConfigMerged, out.Action)
	assert.Equal(t, path, out.Path)
	assert.Equal(t, "dotenv", out.Format)
	assert.Nil(t, out.Backup)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t,
		"DATABASE_URL=postgres://localhost/db\n# keep me\nAUTH0_DOMAIN=\"new.auth0.com\"\nAUTH0_CLIENT_ID=\"cid\"\n",
		string(content))
	assert.NotContains(t, string(content), "old.auth0.com")
	assert.Equal(t, os.FileMode(0644), fileMode(t, path), "merge must keep the existing file mode")

	// No backup is written for a dotenv merge.
	_, statErr := os.Stat(path + ".auth0-backup")
	assert.True(t, os.IsNotExist(statErr))
}

func TestWriteQuickstartConfig_NewFilePerm0600(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", ".env")
	c, _, _ := newQSTestCLI("", false, nil, nil)
	out, err := c.writeQuickstartConfig(&cobra.Command{}, map[string]string{"AUTH0_DOMAIN": "d"},
		&auth0.FileOutputStrategy{Path: path, Format: "dotenv"}, nil)
	require.NoError(t, err)
	assert.Equal(t, qsConfigCreated, out.Action)
	assert.Nil(t, out.Backup)
	assert.Equal(t, os.FileMode(0600), fileMode(t, path))

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "AUTH0_DOMAIN=\"d\"\n", string(content))
}

func TestWriteQuickstartConfig_NonDotenvForceWritesBackup(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "src", "environments", "environment.ts")
	const original = "export const environment = { production: true };\n"
	writeQSFile(t, path, original, 0644)

	c, _, _ := newQSTestCLI("", false, nil, nil)
	c.force = true
	env := map[string]string{"domain": "d.auth0.com", "clientId": "cid"}
	strategy := &auth0.FileOutputStrategy{Path: path, Format: "angular-ts"}

	out, err := c.writeQuickstartConfig(&cobra.Command{}, env, strategy, nil)
	require.NoError(t, err)
	assert.Equal(t, qsConfigOverwritten, out.Action)
	require.NotNil(t, out.Backup)
	assert.Equal(t, path+".auth0-backup", *out.Backup)

	backup, err := os.ReadFile(*out.Backup)
	require.NoError(t, err)
	assert.Equal(t, original, string(backup), "backup must hold the original content")

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(content), "clientId: 'cid'")
	assert.Contains(t, string(content), "domain: 'd.auth0.com'")

	// A second forced run never clobbers the first backup.
	out2, err := c.writeQuickstartConfig(&cobra.Command{}, env, strategy, nil)
	require.NoError(t, err)
	require.NotNil(t, out2.Backup)
	assert.Equal(t, path+".auth0-backup.1", *out2.Backup)
	first, err := os.ReadFile(path + ".auth0-backup")
	require.NoError(t, err)
	assert.Equal(t, original, string(first))
}

func TestWriteQuickstartConfig_SkippedLeavesFileUntouched(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "Auth0.plist")
	writeQSFile(t, path, "original", 0644)

	c, _, stderr := newQSTestCLI("", false, nil, nil)
	plan := &qsConfigWritePlan{Path: path, Format: "plist", Perm: 0644, Action: qsConfigSkipped}
	out, err := c.writeQuickstartConfig(&cobra.Command{}, map[string]string{"ClientId": "cid", "Domain": "d"},
		&auth0.FileOutputStrategy{Path: path, Format: "plist"}, plan)
	require.NoError(t, err)
	assert.Equal(t, qsConfigSkipped, out.Action)
	assert.Nil(t, out.Backup)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "original", string(content))
	_, statErr := os.Stat(path + ".auth0-backup")
	assert.True(t, os.IsNotExist(statErr), "skip must not write a backup")
	// Human output still dumps the content for manual creation.
	assert.Contains(t, stderr.String(), "<key>ClientId</key>")
}

// -- #9 pre-flight through the full setup flow --.

func TestExecuteSetupQuickstart_FileExistsRefusesBeforeAPICall(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeTestFile(t, dir, "angular.json", `{}`)
	envPath := filepath.Join("src", "environments", "environment.ts")
	writeQSFile(t, envPath, "original", 0644)

	ctrl := gomock.NewController(t)
	// No EXPECT: any Client/ResourceServer call fails the test.
	clientAPI := mock.NewMockClientAPI(ctrl)
	rsAPI := mock.NewMockResourceServerAPI(ctrl)
	grantAPI := mock.NewMockClientGrantAPIV3(ctrl)

	c, stdout, _ := newQSTestCLI(display.OutputFormatJSON, true,
		&auth0.API{Client: clientAPI, ResourceServer: rsAPI}, &auth0.APIV3{ClientGrant: grantAPI})
	cmd := newQSSetupCmd(t, c, "--app", "--type", "spa", "--framework", "angular", "--port", "4200")
	inputs := &SetupInputs{App: true, Type: "spa", Framework: "angular", Port: 4200}

	err := executeSetupQuickstart(cmd, c, inputs, false)
	require.Error(t, err)
	assert.Equal(t, "usage", errorClass(err))
	assert.Equal(t, "file_exists", errorReason(err))
	assert.Contains(t, err.Error(), "--force")

	content, readErr := os.ReadFile(envPath)
	require.NoError(t, readErr)
	assert.Equal(t, "original", string(content))
	assert.Empty(t, stdout.String(), "no result is printed on failure")
}

func TestExecuteSetupQuickstart_FileExistsRefusesBeforeAPIRead(t *testing.T) {
	// With --api too, the refusal must come before collectAPIInputs' ResourceServer.Read.
	dir := t.TempDir()
	t.Chdir(dir)
	writeQSFile(t, filepath.Join("src", "environments", "environment.ts"), "original", 0644)

	ctrl := gomock.NewController(t)
	c, _, _ := newQSTestCLI(display.OutputFormatJSON, true,
		&auth0.API{Client: mock.NewMockClientAPI(ctrl), ResourceServer: mock.NewMockResourceServerAPI(ctrl)},
		&auth0.APIV3{ClientGrant: mock.NewMockClientGrantAPIV3(ctrl)})
	cmd := newQSSetupCmd(t, c, "--app", "--api", "--type", "spa", "--framework", "angular", "--port", "4200",
		"--identifier", "https://my-api")
	inputs := &SetupInputs{App: true, API: true, Type: "spa", Framework: "angular", Port: 4200,
		Identifier: "https://my-api", SigningAlg: "RS256", TokenLifetime: "86400"}

	err := executeSetupQuickstart(cmd, c, inputs, false)
	require.Error(t, err)
	assert.Equal(t, "file_exists", errorReason(err))
}

func TestExecuteSetupQuickstart_ForceOverwritesWithBackup(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	envPath := filepath.Join("src", "environments", "environment.ts")
	writeQSFile(t, envPath, "original", 0644)

	ctrl := gomock.NewController(t)
	clientAPI := mock.NewMockClientAPI(ctrl)
	expectClientCreate(clientAPI, "app-cid", "")

	c, stdout, _ := newQSTestCLI(display.OutputFormatJSON, true, &auth0.API{Client: clientAPI}, nil)
	cmd := newQSSetupCmd(t, c, "--app", "--type", "spa", "--framework", "angular", "--port", "4200", "--force")
	require.True(t, c.force, "--force must bind to cli.force")
	inputs := &SetupInputs{App: true, Type: "spa", Framework: "angular", Port: 4200}

	require.NoError(t, executeSetupQuickstart(cmd, c, inputs, false))

	backup, err := os.ReadFile(envPath + ".auth0-backup")
	require.NoError(t, err)
	assert.Equal(t, "original", string(backup))
	content, err := os.ReadFile(envPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "app-cid")

	doc := decodeSingleJSON(t, stdout.Bytes())
	var cfg display.QuickstartSetupConfigFile
	require.NoError(t, json.Unmarshal(doc["config_file"], &cfg))
	assert.Equal(t, qsConfigOverwritten, cfg.Action)
	require.NotNil(t, cfg.Backup)
	assert.Equal(t, envPath+".auth0-backup", *cfg.Backup)

	var steps []string
	require.NoError(t, json.Unmarshal(doc["next_steps"], &steps))
	assert.Contains(t, strings.Join(steps, "\n"), envPath+".auth0-backup", "next_steps must point at the backup")
}

func TestExecuteSetupQuickstart_InteractiveDeclineSkips(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	envPath := filepath.Join("src", "environments", "environment.ts")
	writeQSFile(t, envPath, "original", 0644)

	orig := qsConfirmOverwrite
	t.Cleanup(func() { qsConfirmOverwrite = orig })
	asked := 0
	qsConfirmOverwrite = func(string) bool { asked++; return false }

	ctrl := gomock.NewController(t)
	clientAPI := mock.NewMockClientAPI(ctrl)
	expectClientCreate(clientAPI, "app-cid", "")

	c, stdout, _ := newQSTestCLI(display.OutputFormatJSON, false, &auth0.API{Client: clientAPI}, nil)
	cmd := newQSSetupCmd(t, c, "--app", "--name", "My App", "--type", "spa", "--framework", "angular", "--port", "4200")
	inputs := &SetupInputs{App: true, Name: "My App", Type: "spa", Framework: "angular", Port: 4200}

	require.NoError(t, executeSetupQuickstart(cmd, c, inputs, true))
	assert.Equal(t, 1, asked, "the overwrite prompt must be asked exactly once")

	content, err := os.ReadFile(envPath)
	require.NoError(t, err)
	assert.Equal(t, "original", string(content))

	doc := decodeSingleJSON(t, stdout.Bytes())
	var cfg display.QuickstartSetupConfigFile
	require.NoError(t, json.Unmarshal(doc["config_file"], &cfg))
	assert.Equal(t, qsConfigSkipped, cfg.Action)
	assert.Nil(t, cfg.Backup)
	assert.JSONEq(t, `[]`, string(doc["keys_written"]), "nothing was written")
}

func TestExecuteSetupQuickstart_DotenvMergeNoForceInNoInput(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeQSFile(t, ".env", "DATABASE_URL=postgres://x\nVITE_AUTH0_DOMAIN=old.example\n", 0644)

	ctrl := gomock.NewController(t)
	clientAPI := mock.NewMockClientAPI(ctrl)
	expectClientCreate(clientAPI, "app-cid", "")

	c, stdout, _ := newQSTestCLI(display.OutputFormatJSON, true, &auth0.API{Client: clientAPI}, nil)
	cmd := newQSSetupCmd(t, c, "--app", "--type", "spa", "--framework", "react", "--build-tool", "vite", "--port", "5173")
	inputs := &SetupInputs{App: true, Type: "spa", Framework: "react", BuildTool: "vite", Port: 5173}

	require.NoError(t, executeSetupQuickstart(cmd, c, inputs, false))

	content, err := os.ReadFile(".env")
	require.NoError(t, err)
	assert.Equal(t,
		"DATABASE_URL=postgres://x\nVITE_AUTH0_DOMAIN=\""+qsTestTenant+"\"\nVITE_AUTH0_CLIENT_ID=\"app-cid\"\n",
		string(content))
	assert.Equal(t, os.FileMode(0644), fileMode(t, ".env"))

	doc := decodeSingleJSON(t, stdout.Bytes())
	var cfg display.QuickstartSetupConfigFile
	require.NoError(t, json.Unmarshal(doc["config_file"], &cfg))
	assert.Equal(t, qsConfigMerged, cfg.Action)
	assert.Equal(t, ".env", cfg.Path)
	assert.Equal(t, "dotenv", cfg.Format)
	assert.Nil(t, cfg.Backup)

	var steps []string
	require.NoError(t, json.Unmarshal(doc["next_steps"], &steps))
	assert.NotContains(t, strings.ToLower(strings.Join(steps, "\n")), "backup", "a merge writes no backup")
}

// -- #10 JSON result --.

func TestExecuteSetupQuickstart_JSONResult_AppAndAPI(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	ctrl := gomock.NewController(t)
	clientAPI := mock.NewMockClientAPI(ctrl)
	expectClientCreate(clientAPI, "app-cid", "super-secret")

	rsAPI := mock.NewMockResourceServerAPI(ctrl)
	rsAPI.EXPECT().Read(gomock.Any(), gomock.Any()).Return(nil, errors.New("404 Not Found"))
	rsAPI.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, rs *management.ResourceServer, _ ...management.RequestOption) error {
			id := "rs-1"
			rs.ID = &id
			return nil
		})

	var grantBody map[string]json.RawMessage
	grantAPI := mock.NewMockClientGrantAPIV3(ctrl)
	grantAPI.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *managementv3.CreateClientGrantRequestContent, _ ...option.RequestOption) (*managementv3.CreateClientGrantResponseContent, error) {
			b, err := json.Marshal(req)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(b, &grantBody))
			return &managementv3.CreateClientGrantResponseContent{ID: auth0.String("cgr_1")}, nil
		})

	c, stdout, _ := newQSTestCLI(display.OutputFormatJSON, true,
		&auth0.API{Client: clientAPI, ResourceServer: rsAPI}, &auth0.APIV3{ClientGrant: grantAPI})
	cmd := newQSSetupCmd(t, c,
		"--app", "--api", "--name", "My SPA", "--type", "spa", "--framework", "react", "--build-tool", "vite",
		"--port", "5173", "--identifier", "https://my-api", "--scopes", "read:x,write:x")
	inputs := &SetupInputs{
		App: true, API: true, Name: "My SPA", Type: "spa", Framework: "react", BuildTool: "vite", Port: 5173,
		Identifier: "https://my-api", SigningAlg: "RS256", TokenLifetime: "86400", Scopes: "read:x,write:x",
	}

	require.NoError(t, executeSetupQuickstart(cmd, c, inputs, false))

	out := stdout.Bytes()
	doc := decodeSingleJSON(t, out)
	assert.NotContains(t, string(out), "super-secret", "secrets must never reach stdout")
	assert.NotContains(t, string(out), qsTestTenant, "config values must never reach stdout")

	var res display.QuickstartSetupResult
	require.NoError(t, json.Unmarshal(out, &res))

	require.NotNil(t, res.App)
	assert.Equal(t, "app-cid", res.App.ClientID)
	assert.Equal(t, "My SPA", res.App.Name)
	assert.Equal(t, "spa", res.App.Type)
	assert.Equal(t, []string{"http://localhost:5173"}, res.App.Callbacks)
	assert.Equal(t, []string{"http://localhost:5173"}, res.App.LogoutURLs)
	assert.Equal(t, []string{"http://localhost:5173"}, res.App.WebOrigins)

	require.NotNil(t, res.API)
	assert.Equal(t, "https://my-api", res.API.Identifier)
	assert.Equal(t, []string{"read:x", "write:x"}, res.API.Scopes)

	require.NotNil(t, res.ClientGrant)
	assert.Equal(t, "cgr_1", res.ClientGrant.ID)
	assert.Equal(t, "app-cid", res.ClientGrant.ClientID)
	assert.Equal(t, "https://my-api", res.ClientGrant.Audience)
	assert.Equal(t, []string{"read:x", "write:x"}, res.ClientGrant.Scopes)
	assert.JSONEq(t, `["read:x","write:x"]`, string(grantBody["scope"]), "grant scopes default to --scopes")

	require.NotNil(t, res.ConfigFile)
	assert.Equal(t, qsConfigCreated, res.ConfigFile.Action)
	assert.Equal(t, ".env", res.ConfigFile.Path)
	assert.Equal(t, "dotenv", res.ConfigFile.Format)

	var cfgRaw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(doc["config_file"], &cfgRaw))
	assert.Equal(t, "null", string(cfgRaw["backup"]))

	assert.Equal(t, []string{"VITE_AUTH0_AUDIENCE", "VITE_AUTH0_CLIENT_ID", "VITE_AUTH0_DOMAIN"}, res.KeysWritten)
	assert.JSONEq(t, `[]`, string(doc["next_steps"]), "next_steps must be [] not null")
}

func TestExecuteSetupQuickstart_HumanModeNoStdoutJSON(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	ctrl := gomock.NewController(t)
	clientAPI := mock.NewMockClientAPI(ctrl)
	expectClientCreate(clientAPI, "app-cid", "")

	c, stdout, stderr := newQSTestCLI("", false, &auth0.API{Client: clientAPI}, nil)
	cmd := newQSSetupCmd(t, c, "--app", "--type", "spa", "--framework", "react", "--build-tool", "vite", "--port", "5173")
	inputs := &SetupInputs{App: true, Type: "spa", Framework: "react", BuildTool: "vite", Port: 5173}

	require.NoError(t, executeSetupQuickstart(cmd, c, inputs, false))
	assert.Empty(t, stdout.String(), "human mode prints no JSON result")
	assert.Contains(t, stderr.String(), "file created successfully")
}

func TestPrintDetectionHeader(t *testing.T) {
	t.Parallel()

	for _, format := range []display.OutputFormat{display.OutputFormatJSON, display.OutputFormatJSONCompact} {
		c, stdout, _ := newQSTestCLI(format, false, nil, nil)
		printDetectionHeader(c)
		assert.Empty(t, stdout.String(), "format %q must keep stdout clean", format)
	}

	c, stdout, _ := newQSTestCLI("", false, nil, nil)
	printDetectionHeader(c)
	assert.Contains(t, stdout.String(), "Detected in current directory:")
}

func TestNativeGuidance(t *testing.T) {
	t.Parallel()

	const bundle = "com.example.app"
	tests := []struct {
		name       string
		framework  string
		scheme     string
		bundleID   string
		wantLen    int
		wantSubstr []string
		wantWarn   []bool
	}{
		{"expo with scheme", "expo", "myapp", "", 2, []string{"exp://localhost:19000", "myapp://"}, []bool{false, false}},
		{"expo without scheme", "expo", "", "", 2, []string{"exp://localhost:19000", "custom scheme URI"}, []bool{false, false}},
		{"flutter with bundle", "flutter", "", bundle, 3, []string{
			"Allowed Callback URLs",
			bundle + "://" + qsTestTenant + "/android/" + bundle + "/callback",
			bundle + "://" + qsTestTenant + "/ios/" + bundle + "/callback",
		}, []bool{false, false, false}},
		{"flutter without bundle", "flutter", "", "", 0, nil, nil},
		{"react-native with bundle", "react-native", "", bundle, 3, []string{"", "/android/", "/ios/"}, []bool{false, false, false}},
		{"react-native without bundle", "react-native", "", "", 0, nil, nil},
		{"maui with bundle", "maui", "", bundle, 1, []string{bundle + "://callback"}, []bool{false}},
		{"maui without bundle", "maui", "", "", 0, nil, nil},
		{"dotnet-mobile with bundle", "dotnet-mobile", "", bundle, 1, []string{bundle + "://callback"}, []bool{false}},
		{"ionic with app id", "ionic-react", "", bundle, 2, []string{"Capacitor app ID: " + bundle, "http://localhost"}, []bool{false, false}},
		{"ionic without app id warns", "ionic-angular", "", "", 2, []string{"Could not read Capacitor app ID", "http://localhost"}, []bool{true, false}},
		{"jhipster", "jhipster", "", "", 1, []string{"jhipster.tech"}, []bool{false}},
		{"react has no guidance", "react", "", "", 0, nil, nil},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			lines := nativeGuidance(tc.framework, tc.scheme, tc.bundleID, qsTestTenant)
			require.Len(t, lines, tc.wantLen)
			for i, line := range lines {
				assert.Contains(t, line.text, tc.wantSubstr[i])
				assert.Equal(t, tc.wantWarn[i], line.warn, "line %d warn flag", i)
			}
		})
	}
}

func TestPrintNativeGuidance_ReturnsTrimmedNextSteps(t *testing.T) {
	t.Parallel()

	c, stdout, stderr := newQSTestCLI("", false, nil, nil)
	steps := printNativeGuidance(c, "flutter", "", "com.example.app")
	require.Len(t, steps, 3)
	for _, s := range steps {
		assert.Equal(t, strings.TrimSpace(s), s, "next_steps entries must be trimmed")
	}
	assert.True(t, strings.HasPrefix(steps[1], "Android:"))
	assert.True(t, strings.HasPrefix(steps[2], "iOS:"))
	assert.Contains(t, stderr.String(), "  Android:", "human output keeps the indentation")
	assert.Empty(t, stdout.String())

	assert.Empty(t, printNativeGuidance(c, "react", "", ""))
}

func TestCreateQuickstartApp_FillsResult(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	ctrl := gomock.NewController(t)
	clientAPI := mock.NewMockClientAPI(ctrl)
	expectClientCreate(clientAPI, "app-cid", "super-secret")

	c, _, _ := newQSTestCLI("", false, &auth0.API{Client: clientAPI}, nil)
	inputs := SetupInputs{App: true, Name: "My App", Type: "regular", Framework: "nextjs", BuildTool: "none", Port: 3000}
	result := &display.QuickstartSetupResult{}

	clientID, err := createQuickstartApp(&cobra.Command{}, c, inputs, "regular:nextjs:none", nil, result)
	require.NoError(t, err)
	assert.Equal(t, "app-cid", clientID)

	require.NotNil(t, result.App)
	assert.Equal(t, "app-cid", result.App.ClientID)
	assert.Equal(t, "My App", result.App.Name)
	assert.Equal(t, "regular_web", result.App.Type)
	assert.Equal(t, []string{"http://localhost:3000/auth/callback"}, result.App.Callbacks)
	assert.Equal(t, []string{"http://localhost:3000"}, result.App.LogoutURLs)

	require.NotNil(t, result.ConfigFile)
	assert.Equal(t, qsConfigCreated, result.ConfigFile.Action)
	assert.Equal(t, ".env", result.ConfigFile.Path)

	assert.Equal(t, []string{"APP_BASE_URL", "AUTH0_CLIENT_ID", "AUTH0_CLIENT_SECRET", "AUTH0_DOMAIN", "AUTH0_SECRET"}, result.KeysWritten)
	for _, k := range result.KeysWritten {
		assert.NotContains(t, k, "super-secret")
		assert.NotContains(t, k, "app-cid")
		assert.NotContains(t, k, qsTestTenant)
		assert.NotContains(t, k, "=")
	}
	assert.Empty(t, result.NextSteps)
}

func TestCreateQuickstartApp_SkippedResultHasNoKeysAndANextStep(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	ctrl := gomock.NewController(t)
	clientAPI := mock.NewMockClientAPI(ctrl)
	expectClientCreate(clientAPI, "app-cid", "")

	c, _, _ := newQSTestCLI("", false, &auth0.API{Client: clientAPI}, nil)
	inputs := SetupInputs{App: true, Name: "My App", Type: "native", Framework: "ios-swift", BuildTool: "none"}
	plan := &qsConfigWritePlan{Path: "Auth0.plist", Format: "plist", Perm: 0600, Action: qsConfigSkipped}
	result := &display.QuickstartSetupResult{}

	_, err := createQuickstartApp(&cobra.Command{}, c, inputs, "native:ios-swift:none", plan, result)
	require.NoError(t, err)
	require.NotNil(t, result.ConfigFile)
	assert.Equal(t, qsConfigSkipped, result.ConfigFile.Action)
	assert.Empty(t, result.KeysWritten)
	require.NotEmpty(t, result.NextSteps)
	assert.Contains(t, strings.Join(result.NextSteps, "\n"), "Auth0.plist")
	_, statErr := os.Stat(filepath.Join(dir, "Auth0.plist"))
	assert.True(t, os.IsNotExist(statErr))
}

// -- #8 grant scopes --.

func TestValidateGrantScopes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		inputs     SetupInputs
		set        bool
		wantErr    bool
		wantSubstr string
	}{
		{"unset", SetupInputs{API: true, Scopes: "read:x"}, false, false, ""},
		{"unset without api", SetupInputs{App: true}, false, false, ""},
		{"subset", SetupInputs{API: true, Scopes: "read:x, write:x", GrantScopes: []string{"read:x"}}, true, false, ""},
		{"equal set", SetupInputs{API: true, Scopes: "read:x,write:x", GrantScopes: []string{"write:x", "read:x"}}, true, false, ""},
		{"scope not in --scopes", SetupInputs{API: true, Scopes: "read:x", GrantScopes: []string{"read:x", "delete:x"}}, true, true, "delete:x"},
		{"set without --api (D3)", SetupInputs{App: true, GrantScopes: []string{"read:x"}}, true, true, "--api"},
		{"empty --scopes with grant scopes", SetupInputs{API: true, GrantScopes: []string{"read:x"}}, true, true, "read:x"},
		{"explicit empty grant scopes", SetupInputs{API: true, Scopes: "read:x", GrantScopes: []string{}}, true, false, ""},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateGrantScopes(tc.inputs, tc.set)
			if !tc.wantErr {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Equal(t, "usage", errorClass(err))
			assert.Equal(t, "invalid_flag_value", errorReason(err))
			assert.Contains(t, err.Error(), tc.wantSubstr)
		})
	}
}

func TestParseScopeList(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{}, parseScopeList(""))
	assert.NotNil(t, parseScopeList(""), "never nil")
	assert.Equal(t, []string{}, parseScopeList(" , ,"))
	assert.Equal(t, []string{"read:x", "write:x"}, parseScopeList(" read:x, write:x ,"))
}

func TestNormalizeScopeSlice(t *testing.T) {
	t.Parallel()

	assert.Nil(t, normalizeScopeSlice(nil), "nil stays nil (flag unset)")
	got := normalizeScopeSlice([]string{})
	assert.NotNil(t, got, "explicit empty stays non-nil")
	assert.Empty(t, got)
	got = normalizeScopeSlice([]string{" "})
	assert.NotNil(t, got)
	assert.Empty(t, got)
	assert.Equal(t, []string{"read:x", "write:x"}, normalizeScopeSlice([]string{" read:x ", "", "write:x"}))
}

// runGrantScopesCase runs createQuickstartAPI with a linked app and returns the
// marshalled grant request body.
func runGrantScopesCase(t *testing.T, inputs SetupInputs, resp *managementv3.CreateClientGrantResponseContent, result *display.QuickstartSetupResult) map[string]json.RawMessage {
	t.Helper()
	ctrl := gomock.NewController(t)

	rsAPI := mock.NewMockResourceServerAPI(ctrl)
	rsAPI.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	var body map[string]json.RawMessage
	grantAPI := mock.NewMockClientGrantAPIV3(ctrl)
	grantAPI.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *managementv3.CreateClientGrantRequestContent, _ ...option.RequestOption) (*managementv3.CreateClientGrantResponseContent, error) {
			b, err := json.Marshal(req)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(b, &body))
			return resp, nil
		})

	c, _, _ := newQSTestCLI("", false, &auth0.API{ResourceServer: rsAPI}, &auth0.APIV3{ClientGrant: grantAPI})
	require.NoError(t, createQuickstartAPI(context.Background(), c, inputs, result))
	return body
}

func grantTestInputs() SetupInputs {
	return SetupInputs{
		API: true, Name: "My App", Identifier: "https://my-api", SigningAlg: "RS256",
		TokenLifetime: "86400", LinkedAppID: "linked-app-client-id",
	}
}

func TestCreateQuickstartAPI_GrantScopesDefaultToScopes(t *testing.T) {
	t.Parallel()
	in := grantTestInputs()
	in.Scopes = "read:x, write:x"
	body := runGrantScopesCase(t, in, nil, nil)
	assert.JSONEq(t, `["read:x","write:x"]`, string(body["scope"]))
}

func TestCreateQuickstartAPI_GrantScopesOverride(t *testing.T) {
	t.Parallel()
	in := grantTestInputs()
	in.Scopes = "read:x,write:x"
	in.GrantScopes = []string{"read:x"}
	body := runGrantScopesCase(t, in, nil, nil)
	assert.JSONEq(t, `["read:x"]`, string(body["scope"]))
}

func TestCreateQuickstartAPI_GrantScopesExplicitEmpty(t *testing.T) {
	t.Parallel()
	in := grantTestInputs()
	in.Scopes = "read:x"
	in.GrantScopes = []string{}
	body := runGrantScopesCase(t, in, nil, nil)
	require.Contains(t, body, "scope", "an explicit empty list must still be sent")
	assert.JSONEq(t, `[]`, string(body["scope"]))
}

func TestCreateQuickstartAPI_GrantFailureIsError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	rsAPI := mock.NewMockResourceServerAPI(ctrl)
	rsAPI.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	boom := errors.New("boom")
	grantAPI := mock.NewMockClientGrantAPIV3(ctrl)
	grantAPI.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil, boom)

	c, _, _ := newQSTestCLI("", false, &auth0.API{ResourceServer: rsAPI}, &auth0.APIV3{ClientGrant: grantAPI})
	result := &display.QuickstartSetupResult{}
	err := createQuickstartAPI(context.Background(), c, grantTestInputs(), result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "linked-app-client-id")
	assert.Contains(t, err.Error(), "https://my-api")
	assert.Contains(t, err.Error(), "boom")
	assert.ErrorIs(t, err, boom)
	assert.Nil(t, result.ClientGrant, "no grant section on failure")
}

func TestCreateQuickstartAPI_APICreateFailureNamesCreatedApp(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	run := func(t *testing.T, appCreatedInThisRun bool) error {
		ctrl := gomock.NewController(t)
		rsAPI := mock.NewMockResourceServerAPI(ctrl)
		rsAPI.EXPECT().Create(gomock.Any(), gomock.Any()).Return(boom)

		c, _, _ := newQSTestCLI("", false, &auth0.API{ResourceServer: rsAPI}, &auth0.APIV3{ClientGrant: mock.NewMockClientGrantAPIV3(ctrl)})
		inputs := grantTestInputs()
		inputs.App = appCreatedInThisRun
		return createQuickstartAPI(context.Background(), c, inputs, nil)
	}

	t.Run("app created in this run is named in the error", func(t *testing.T) {
		t.Parallel()
		err := run(t, true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "linked-app-client-id")
		assert.Contains(t, err.Error(), "already created")
		assert.ErrorIs(t, err, boom)
	})

	t.Run("linking an existing app does not claim it was created", func(t *testing.T) {
		t.Parallel()
		err := run(t, false)
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "already created")
		assert.ErrorIs(t, err, boom)
	})
}

func TestCreateQuickstartApp_ConfigWriteFailureNamesCreatedApp(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	// A directory where the config file should go: the write fails after the pre-flight passed.
	require.NoError(t, os.Mkdir(".env", 0755))

	ctrl := gomock.NewController(t)
	clientAPI := mock.NewMockClientAPI(ctrl)
	expectClientCreate(clientAPI, "app-cid", "")
	c, _, _ := newQSTestCLI("", false, &auth0.API{Client: clientAPI}, nil)

	plan := &qsConfigWritePlan{Path: ".env", Format: "dotenv", Action: qsConfigCreated, Perm: 0600}
	inputs := SetupInputs{App: true, Type: "spa", Framework: "react", BuildTool: "vite", Name: "My App", Port: 5173}
	_, err := createQuickstartApp(&cobra.Command{}, c, inputs, "spa:react:vite", plan, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "app-cid", "the error must name the application that now exists")
	assert.Contains(t, err.Error(), "failed to generate config file")
}

func TestCreateQuickstartAPI_FillsResult(t *testing.T) {
	t.Parallel()

	t.Run("nil grant response leaves ID empty", func(t *testing.T) {
		t.Parallel()
		in := grantTestInputs()
		in.Scopes = "read:x"
		result := &display.QuickstartSetupResult{}
		runGrantScopesCase(t, in, nil, result)

		require.NotNil(t, result.API)
		assert.Equal(t, "https://my-api", result.API.Identifier)
		assert.Equal(t, "My App-API", result.API.Name)
		assert.Equal(t, []string{"read:x"}, result.API.Scopes)

		require.NotNil(t, result.ClientGrant)
		assert.Equal(t, "", result.ClientGrant.ID)
		assert.Equal(t, "linked-app-client-id", result.ClientGrant.ClientID)
		assert.Equal(t, "https://my-api", result.ClientGrant.Audience)
		assert.Equal(t, []string{"read:x"}, result.ClientGrant.Scopes)
	})

	t.Run("grant ID from response", func(t *testing.T) {
		t.Parallel()
		result := &display.QuickstartSetupResult{}
		runGrantScopesCase(t, grantTestInputs(), &managementv3.CreateClientGrantResponseContent{ID: auth0.String("cgr_9")}, result)
		require.NotNil(t, result.ClientGrant)
		assert.Equal(t, "cgr_9", result.ClientGrant.ID)
		assert.NotNil(t, result.ClientGrant.Scopes)
		assert.Empty(t, result.ClientGrant.Scopes)
	})

	t.Run("no linked app leaves client_grant nil", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		rsAPI := mock.NewMockResourceServerAPI(ctrl)
		rsAPI.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
		c, _, _ := newQSTestCLI("", false, &auth0.API{ResourceServer: rsAPI},
			&auth0.APIV3{ClientGrant: mock.NewMockClientGrantAPIV3(ctrl)})
		in := grantTestInputs()
		in.LinkedAppID = ""
		result := &display.QuickstartSetupResult{}
		require.NoError(t, createQuickstartAPI(context.Background(), c, in, result))
		require.NotNil(t, result.API)
		assert.Nil(t, result.ClientGrant)
	})
}

// -- #3 multi-value URL flags --.

func TestGenerateClient_URLOverrides(t *testing.T) {
	t.Parallel()

	const def = "http://localhost:5173"
	tests := []struct {
		name                         string
		callbacks, logout, origins   []string
		wantCB, wantLogout, wantOrig []string
	}{
		{
			name:      "multi-value lists replace the defaults",
			callbacks: []string{"http://a", "http://b"},
			logout:    []string{"http://l1", "http://l2"},
			origins:   []string{"http://w"},
			wantCB:    []string{"http://a", "http://b"}, wantLogout: []string{"http://l1", "http://l2"}, wantOrig: []string{"http://w"},
		},
		{
			name:   "nil keeps the defaults",
			wantCB: []string{def}, wantLogout: []string{def}, wantOrig: []string{def},
		},
		{
			name:      "blanks are trimmed and dropped; an all-blank list keeps the default",
			callbacks: []string{" http://a ", ""},
			logout:    []string{"", " "},
			wantCB:    []string{"http://a"}, wantLogout: []string{def}, wantOrig: []string{def},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := auth0.QuickstartConfigs["spa:react:vite"]
			c, err := generateClient(SetupInputs{
				Name: "App", Port: 5173,
				CallbackURLs: tc.callbacks, LogoutURLs: tc.logout, WebOriginURLs: tc.origins,
			}, cfg.RequestParams)
			require.NoError(t, err)
			assert.Equal(t, tc.wantCB, c.GetCallbacks())
			assert.Equal(t, tc.wantLogout, c.GetAllowedLogoutURLs())
			assert.Equal(t, tc.wantOrig, c.GetWebOrigins())
		})
	}
}

func TestCleanURLList(t *testing.T) {
	t.Parallel()

	assert.Nil(t, cleanURLList(nil))
	assert.Nil(t, cleanURLList([]string{"", "  "}))
	assert.Equal(t, []string{"http://a", "http://b"}, cleanURLList([]string{" http://a", "", "http://b "}))
}

func TestSetupQuickstartCmd_URLFlagsMultiValue(t *testing.T) {
	t.Parallel()

	cmd := setupQuickstartCmd(&cli{})
	require.NoError(t, cmd.ParseFlags([]string{
		"--callback-url", "http://a,http://b", "--callback-url", "http://c",
		"--logout-url", "http://l",
		"--web-origin-url", "http://w1,http://w2",
		"--grant-scopes", "read:x,write:x", "--grant-scopes", "admin",
	}))

	got, err := cmd.Flags().GetStringSlice("callback-url")
	require.NoError(t, err)
	assert.Equal(t, []string{"http://a", "http://b", "http://c"}, got)

	got, err = cmd.Flags().GetStringSlice("logout-url")
	require.NoError(t, err)
	assert.Equal(t, []string{"http://l"}, got)

	got, err = cmd.Flags().GetStringSlice("web-origin-url")
	require.NoError(t, err)
	assert.Equal(t, []string{"http://w1", "http://w2"}, got)

	got, err = cmd.Flags().GetStringSlice("grant-scopes")
	require.NoError(t, err)
	assert.Equal(t, []string{"read:x", "write:x", "admin"}, got)
}

// -- #2 help / flags --.

func TestSetupBuildToolHelp(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Build tool used by the project (composer, gradle, maven, none, vite, webpack)", setupBuildTool.Help)
	assert.Equal(t, "Build tool used by the project ("+strings.Join(auth0.SupportedBuildTools(), ", ")+")", setupBuildTool.Help)
	assert.NotContains(t, setupBuildTool.Help, "cra")
}

func TestSetupQuickstartCmd_RegistersNewFlags(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cmd := setupQuickstartCmd(&cli{})
	for _, name := range []string{"force", "grant-scopes", "json", "json-compact", "callback-url", "logout-url", "web-origin-url"} {
		assert.NotNil(t, cmd.Flags().Lookup(name), "flag --%s must be registered", name)
	}
	assert.Equal(t, "stringSlice", cmd.Flags().Lookup("callback-url").Value.Type())
	assert.Equal(t, "stringSlice", cmd.Flags().Lookup("grant-scopes").Value.Type())
	assert.Equal(t, "bool", cmd.Flags().Lookup("force").Value.Type())

	cmd = setupQuickstartCmd(&cli{})
	cmd.SetArgs([]string{"--app", "--json", "--json-compact"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "json")
	assert.Contains(t, err.Error(), "json-compact")
	assert.Contains(t, err.Error(), "none of the others can be")
}

func TestDefaultPortForSetup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		framework, buildTool string
		want                 int
	}{
		{"react", "webpack", 8080},
		{"vanilla-javascript", "webpack", 8080},
		{"react", "vite", 5173},
		{"vanilla-javascript", "vite", 5173},
		{"tanstack-start", "none", 3000},
		{"angular", "none", 4200},
		{"nextjs", "", 3000},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, defaultPortForSetup(tc.framework, tc.buildTool), "%s/%s", tc.framework, tc.buildTool)
	}
}
