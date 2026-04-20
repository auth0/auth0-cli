package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/auth0"
)

// ── DetectionSubBase ──────────────────────────────────────────────────────────.

// TestResolveRequestParams_DetectionSubBase verifies that DetectionSubBase in
// callbacks resolves to baseURL with no path suffix (unlike DetectionSub which
// appends "/callback").
func TestResolveRequestParams_DetectionSubBase(t *testing.T) {
	t.Parallel()

	t.Run("callback resolves to baseURL only", func(t *testing.T) {
		t.Parallel()
		req := auth0.RequestParams{
			AppType:           "spa",
			Callbacks:         []string{auth0.DetectionSubBase},
			AllowedLogoutURLs: []string{auth0.DetectionSub},
			WebOrigins:        []string{auth0.DetectionSub},
			Name:              auth0.DetectionSub,
		}
		got := resolveRequestParams(req, "MyApp", 5173)
		assert.Equal(t, []string{"http://localhost:5173"}, got.Callbacks, "callback should be baseURL with no path")
		assert.Equal(t, []string{"http://localhost:5173"}, got.AllowedLogoutURLs)
		assert.Equal(t, []string{"http://localhost:5173"}, got.WebOrigins)
	})

	t.Run("DetectionSubBase in logoutURLs resolves to baseURL", func(t *testing.T) {
		t.Parallel()
		req := auth0.RequestParams{
			AllowedLogoutURLs: []string{auth0.DetectionSubBase},
		}
		got := resolveRequestParams(req, "App", 3000)
		assert.Equal(t, []string{"http://localhost:3000"}, got.AllowedLogoutURLs)
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

// ── resolveRequestParams with QuickstartConfigs ───────────────────────────────.

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
			[]string{"http://localhost:3000/api/auth/callback"},
			[]string{"http://localhost:3000"}, nil, "regular_web"},
		{"regular:fastify:none", 3000,
			[]string{"http://localhost:3000/auth/callback"},
			[]string{"http://localhost:3000"}, nil, "regular_web"},
		{"regular:nuxt:none", 3000,
			[]string{"http://localhost:3000/callback"},
			[]string{"http://localhost:3000"}, nil, "regular_web"},
		{"regular:express:none", 3000,
			[]string{"http://localhost:3000/callback"},
			[]string{"http://localhost:3000"}, nil, "regular_web"},
		{"regular:hono:none", 3000,
			[]string{"http://localhost:3000/callback"},
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
		{"regular:spring-boot:maven", 8080,
			[]string{"http://localhost:8080/login/oauth2/code/oidc"},
			[]string{"http://localhost:8080"}, nil, "regular_web"},
		{"regular:spring-boot:gradle", 8080,
			[]string{"http://localhost:8080/login/oauth2/code/oidc"},
			[]string{"http://localhost:8080"}, nil, "regular_web"},
		{"regular:laravel:composer", 8000,
			[]string{"http://localhost:8000/callback"},
			[]string{"http://localhost:8000"}, nil, "regular_web"},
		{"regular:rails:none", 3000,
			[]string{"http://localhost:3000/callback"},
			[]string{"http://localhost:3000"}, nil, "regular_web"},
		{"regular:aspnet-mvc:none", 3000,
			[]string{"http://localhost:3000/callback"},
			[]string{"http://localhost:3000"}, nil, "regular_web"},
		{"regular:aspnet-blazor:none", 3000,
			[]string{"http://localhost:3000/callback"},
			[]string{"http://localhost:3000"}, nil, "regular_web"},
		{"regular:aspnet-owin:none", 3000,
			[]string{"http://localhost:3000/callback"},
			[]string{"http://localhost:3000"}, nil, "regular_web"},
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
		// M2M: no URLs.
		{"m2m:none:none", 0, []string{}, []string{}, nil, "non_interactive"},
		// Custom port propagates.
		{"spa:react:vite", 8080,
			[]string{"http://localhost:8080"},
			[]string{"http://localhost:8080"},
			[]string{"http://localhost:8080"}, "spa"},
		{"regular:nextjs:none", 8080,
			[]string{"http://localhost:8080/api/auth/callback"},
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

// ── GenerateAndWriteQuickstartConfig with QuickstartConfigs ──────────────────.

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
		{"spa:angular:none", 4200, "environment.ts",
			[]string{"domain", "clientId"},
			map[string]string{"domain": domain, "clientId": cidVal}},
		{"spa:flutter-web:none", 3000, "auth_config.dart",
			[]string{"domain", "clientId"},
			map[string]string{"domain": domain, "clientId": cidVal}},
		// Regular web.
		{"regular:nextjs:none", 3000, ".env",
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
			[]string{"AUTH0_DOMAIN", "AUTH0_CLIENT_ID", "AUTH0_CLIENT_SECRET", "AUTH0_COOKIE_SECRET"},
			map[string]string{"AUTH0_DOMAIN": domain, "AUTH0_CLIENT_ID": cidVal}},
		{"regular:rails:none", 3000, ".env",
			[]string{"auth0_domain", "auth0_client_id", "auth0_client_secret"},
			map[string]string{"auth0_domain": domain, "auth0_client_id": cidVal}},
		{"regular:aspnet-mvc:none", 3000, "appsettings.json",
			[]string{"Domain", "ClientId", "ClientSecret"}, nil},
		{"regular:aspnet-blazor:none", 3000, "appsettings.json",
			[]string{"Domain", "ClientId"}, nil},
		{"regular:aspnet-owin:none", 3000, "Web.config",
			[]string{"auth0:Domain", "auth0:ClientId"}, nil},
		{"regular:vanilla-php:composer", 3000, ".env",
			[]string{"AUTH0_DOMAIN", "AUTH0_CLIENT_ID", "AUTH0_CLIENT_SECRET", "AUTH0_COOKIE_SECRET"},
			map[string]string{"AUTH0_DOMAIN": domain}},
		{"regular:vanilla-java:maven", 8080, "application.properties",
			[]string{"auth0.domain", "auth0.clientId", "auth0.clientSecret"},
			map[string]string{"auth0.domain": domain, "auth0.clientId": cidVal}},
		{"regular:java-ee:maven", 8080, "microprofile-config.properties",
			[]string{"auth0.domain", "auth0.clientId", "auth0.clientSecret"},
			map[string]string{"auth0.domain": domain, "auth0.clientId": cidVal}},
		{"regular:sveltekit:none", 3000, ".env",
			[]string{"AUTH0_DOMAIN", "AUTH0_CLIENT_ID", "AUTH0_CLIENT_SECRET", "AUTH0_SECRET", "APP_BASE_URL"},
			map[string]string{"AUTH0_DOMAIN": domain, "AUTH0_CLIENT_ID": cidVal, "AUTH0_CLIENT_SECRET": csecVal, "APP_BASE_URL": "http://localhost:3000"}},
		// SvelteKit + Vite uses the same server-side config as sveltekit:none.
		{"regular:sveltekit:vite", 3000, ".env",
			[]string{"AUTH0_DOMAIN", "AUTH0_CLIENT_ID", "AUTH0_CLIENT_SECRET", "AUTH0_SECRET", "APP_BASE_URL"},
			map[string]string{"AUTH0_DOMAIN": domain, "AUTH0_CLIENT_ID": cidVal, "AUTH0_CLIENT_SECRET": csecVal, "APP_BASE_URL": "http://localhost:3000"}},
		{"regular:django:none", 3000, ".env",
			[]string{"AUTH0_DOMAIN", "AUTH0_CLIENT_ID", "AUTH0_CLIENT_SECRET"},
			map[string]string{"AUTH0_DOMAIN": domain, "AUTH0_CLIENT_ID": cidVal}},
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

			fileName, filePath, err := GenerateAndWriteQuickstartConfig(&strategy, config.EnvValues, domain, client, tc.port)
			require.NoError(t, err)

			assert.Equal(t, tc.wantFileName, fileName)
			assert.FileExists(t, filePath)

			content, err := os.ReadFile(filePath)
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

// ── generateClient with QuickstartConfigs ────────────────────────────────────.

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
		{"spa:angular:none", 4200, "spa", 1, "http://localhost:4200", 1, 1},
		{"spa:flutter-web:none", 3000, "spa", 1, "http://localhost:3000", 1, 1},
		// Regular web: framework-specific paths.
		{"regular:nextjs:none", 3000, "regular_web", 1, "http://localhost:3000/api/auth/callback", 1, 0},
		{"regular:fastify:none", 3000, "regular_web", 1, "http://localhost:3000/auth/callback", 1, 0},
		{"regular:nuxt:none", 3000, "regular_web", 1, "http://localhost:3000/callback", 1, 0},
		{"regular:express:none", 3000, "regular_web", 1, "http://localhost:3000/callback", 1, 0},
		{"regular:hono:none", 3000, "regular_web", 1, "http://localhost:3000/callback", 1, 0},
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
		{"regular:rails:none", 3000, "regular_web", 1, "http://localhost:3000/callback", 1, 0},
		{"regular:aspnet-mvc:none", 3000, "regular_web", 1, "http://localhost:3000/callback", 1, 0},
		{"regular:aspnet-blazor:none", 3000, "regular_web", 1, "http://localhost:3000/callback", 1, 0},
		{"regular:aspnet-owin:none", 3000, "regular_web", 1, "http://localhost:3000/callback", 1, 0},
		{"regular:vanilla-php:composer", 3000, "regular_web", 1, "http://localhost:3000/callback", 1, 0},
		{"regular:vanilla-java:maven", 8080, "regular_web", 1, "http://localhost:8080/callback", 1, 0},
		{"regular:java-ee:maven", 8080, "regular_web", 1, "http://localhost:8080/callback", 1, 0},
		{"regular:spring-boot:maven", 8080, "regular_web", 1, "http://localhost:8080/login/oauth2/code/oidc", 1, 0},
		{"regular:spring-boot:gradle", 8080, "regular_web", 1, "http://localhost:8080/login/oauth2/code/oidc", 1, 0},
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

// ── APP_BASE_URL reflects the user-specified port ────────────────────────────.

func TestGenerateAndWriteQuickstartConfig_PortInBaseURL(t *testing.T) {
	t.Parallel()

	for _, configKey := range []string{"regular:nextjs:none", "regular:fastify:none", "regular:express:none"} {
		t.Run(configKey, func(t *testing.T) {
			t.Parallel()

			config := auth0.QuickstartConfigs[configKey]
			dir := t.TempDir()
			strategy := auth0.FileOutputStrategy{Path: filepath.Join(dir, ".env"), Format: "dotenv"}
			cid, csec := "cid", "csec"
			client := &management.Client{ClientID: &cid, ClientSecret: &csec}

			_, _, err := GenerateAndWriteQuickstartConfig(&strategy, config.EnvValues, "example.auth0.com", client, 8080)
			require.NoError(t, err)

			content, err := os.ReadFile(strategy.Path)
			require.NoError(t, err)
			assert.Contains(t, string(content), "8080",
				"%s: port 8080 should appear in the generated file", configKey)
		})
	}
}

// ── Generated secrets (AUTH0_SECRET / SESSION_SECRET) are non-empty ──────────.

func TestGenerateAndWriteQuickstartConfig_SecretsNonEmpty(t *testing.T) {
	t.Parallel()

	cid, csec := "cid", "csec"
	client := &management.Client{ClientID: &cid, ClientSecret: &csec}

	for _, configKey := range []string{"regular:nextjs:none", "regular:fastify:none", "regular:sveltekit:none", "regular:sveltekit:vite"} {
		t.Run(configKey, func(t *testing.T) {
			t.Parallel()

			config := auth0.QuickstartConfigs[configKey]
			dir := t.TempDir()
			strategy := auth0.FileOutputStrategy{Path: filepath.Join(dir, ".env"), Format: "dotenv"}

			_, _, err := GenerateAndWriteQuickstartConfig(&strategy, config.EnvValues, "example.auth0.com", client, 3000)
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
			name:       "bare http scheme no host",
			identifier: "http://",
			wantErr:    true,
		},
		{
			name:       "bare https scheme no host",
			identifier: "https://",
			wantErr:    true,
		},
		{
			name:       "no scheme",
			identifier: "example.com/api",
			wantErr:    true,
		},
		{
			name:       "wrong scheme",
			identifier: "ftp://example.com/api",
			wantErr:    true,
		},
		{
			name:       "empty string",
			identifier: "",
			wantErr:    true,
		},
		{
			name:       "plain string no URL",
			identifier: "not-a-url",
			wantErr:    true,
		},
		{
			name:       "URL with userinfo credentials",
			identifier: "http://user:pass@host.com",
			wantErr:    true,
		},
	}

	for _, tc := range tests {
		tc := tc
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
