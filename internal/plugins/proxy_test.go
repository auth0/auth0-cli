package plugins

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequiredScopeFor(t *testing.T) {
	tests := []struct {
		name      string
		method    string
		path      string
		wantScope string
		wantOK    bool
	}{
		{"get maps to read", http.MethodGet, "/api/v2/clients", "read:clients", true},
		{"get sub-resource uses first segment", http.MethodGet, "/api/v2/clients/abc123", "read:clients", true},
		{"post maps to create", http.MethodPost, "/api/v2/roles", "create:roles", true},
		{"patch maps to update", http.MethodPatch, "/api/v2/roles/r1", "update:roles", true},
		{"put maps to update", http.MethodPut, "/api/v2/rules/r1", "update:rules", true},
		{"delete maps to delete", http.MethodDelete, "/api/v2/roles/r1", "delete:roles", true},
		{"hyphens normalized to underscores", http.MethodGet, "/api/v2/attack-protection/brute-force-protection", "read:attack_protection", true},
		{"tenant settings read override", http.MethodGet, "/api/v2/tenants/settings", "read:tenant_settings", true},
		{"tenant settings update override", http.MethodPatch, "/api/v2/tenants/settings", "update:tenant_settings", true},
		{"email provider read override", http.MethodGet, "/api/v2/emails/provider", "read:email_provider", true},
		{"email provider create override", http.MethodPost, "/api/v2/emails/provider", "create:email_provider", true},
		{"email provider update override", http.MethodPatch, "/api/v2/emails/provider", "update:email_provider", true},
		{"email provider delete override", http.MethodDelete, "/api/v2/emails/provider", "delete:email_provider", true},
		{"guardian factors read override", http.MethodGet, "/api/v2/guardian/factors", "read:guardian_factors", true},
		{"guardian factors update override", http.MethodPut, "/api/v2/guardian/factors/sms", "update:guardian_factors", true},
		{"guardian factors nested path override", http.MethodGet, "/api/v2/guardian/factors/sms/providers/twilio", "read:guardian_factors", true},
		{"guardian policies read override", http.MethodGet, "/api/v2/guardian/policies", "read:mfa_policies", true},
		{"guardian policies update override", http.MethodPut, "/api/v2/guardian/policies", "update:mfa_policies", true},
		{"override tolerates a trailing slash", http.MethodGet, "/api/v2/tenants/settings/", "read:tenant_settings", true},
		{"tenants without settings falls back to first segment", http.MethodGet, "/api/v2/tenants", "read:tenants", true},
		{"emails without provider falls back to first segment", http.MethodGet, "/api/v2/emails/templates/welcome", "read:emails", true},
		{"guardian elsewhere falls back to first segment", http.MethodGet, "/api/v2/guardian/enrollments/abc", "read:guardian", true},
		{"override prefix must end on a segment boundary", http.MethodGet, "/api/v2/guardian/factorsX", "read:guardian", true},
		{"override prefix boundary for nested names", http.MethodGet, "/api/v2/tenants/settings-extra", "read:tenants", true},
		{"anomaly captchas override", http.MethodGet, "/api/v2/anomaly/captchas", "read:attack_protection", true},
		{"anomaly captchas update override", http.MethodPatch, "/api/v2/anomaly/captchas", "update:attack_protection", true},
		{"anomaly elsewhere falls back to first segment", http.MethodGet, "/api/v2/anomaly/blocks/ip/1.2.3.4", "read:anomaly", true},
		{"management prefix is case-insensitive", http.MethodGet, "/API/V2/clients", "read:clients", true},
		{"bare management root has no resource", http.MethodGet, "/api/v2", "", false},
		{"non management path not enforced", http.MethodGet, "/authorize", "", false},
		{"empty resource not enforced", http.MethodGet, "/api/v2/", "", false},
		{"unknown method not enforced", "OPTIONS", "/api/v2/clients", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scope, ok := requiredScopeFor(tt.method, tt.path)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantScope, scope)
		})
	}
}

// stubProvider is a TokenProvider whose token rotates once Refresh is called.
// It counts refreshes so tests can assert single-flight behavior.
type stubProvider struct {
	mu           sync.Mutex
	token        string
	refreshCount int32
	refreshErr   error
}

func (s *stubProvider) Token() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.token
}

func (s *stubProvider) Refresh(_ context.Context) (string, error) {
	atomic.AddInt32(&s.refreshCount, 1)
	if s.refreshErr != nil {
		return "", s.refreshErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.token = "new"
	return s.token, nil
}

// roundTripFunc adapts a function to an http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// okOn returns a base transport that answers 200 only when the request carries
// "Bearer <goodToken>", and 401 otherwise.
func okOn(goodToken string, hits *int32) http.RoundTripper {
	return roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if hits != nil {
			atomic.AddInt32(hits, 1)
		}
		status := http.StatusUnauthorized
		if r.Header.Get("Authorization") == "Bearer "+goodToken {
			status = http.StatusOK
		}
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(nil),
			Header:     make(http.Header),
		}, nil
	})
}

func TestAuthTransportRefreshesOn401(t *testing.T) {
	provider := &stubProvider{token: "old"}
	transport := &authTransport{base: okOn("new", nil), provider: provider}

	req, err := http.NewRequest(http.MethodGet, "https://example.test/api/v2/clients", nil)
	require.NoError(t, err)

	resp, err := transport.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(1), atomic.LoadInt32(&provider.refreshCount))
}

func TestAuthTransportSurfaces401WhenRefreshFails(t *testing.T) {
	provider := &stubProvider{token: "old", refreshErr: assert.AnError}
	transport := &authTransport{base: okOn("new", nil), provider: provider}

	req, err := http.NewRequest(http.MethodGet, "https://example.test/api/v2/clients", nil)
	require.NoError(t, err)

	resp, err := transport.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAuthTransportSingleFlight(t *testing.T) {
	provider := &stubProvider{token: "old"}
	transport := &authTransport{base: okOn("new", nil), provider: provider}

	const concurrent = 12
	var wg sync.WaitGroup
	results := make([]int, concurrent)
	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req, _ := http.NewRequest(http.MethodGet, "https://example.test/api/v2/clients", nil)
			resp, err := transport.RoundTrip(req)
			if err == nil {
				results[idx] = resp.StatusCode
			}
		}(i)
	}
	wg.Wait()

	for _, code := range results {
		assert.Equal(t, http.StatusOK, code)
	}
	// All the concurrent 401s must collapse into a single refresh.
	assert.Equal(t, int32(1), atomic.LoadInt32(&provider.refreshCount))
}

func TestProxyScopeEnforcement(t *testing.T) {
	// Point upstream at an unroutable loopback port: allowed requests pass the
	// scope gate and then fail to connect (non-403), while disallowed requests
	// are rejected at the gate with 403 before any forwarding.
	provider := &stubProvider{token: "tok"}
	proxy, err := StartProxy(context.Background(), "127.0.0.1:1", provider, []string{"read:clients"})
	require.NoError(t, err)
	defer proxy.Close()

	client := loopbackClient(t, proxy.CACertPath)

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int // 0 means "any status other than 403".
	}{
		{"declared read scope passes the gate", http.MethodGet, "/api/v2/clients", 0},
		{"undeclared read scope is blocked", http.MethodGet, "/api/v2/logs", http.StatusForbidden},
		{"undeclared write scope is blocked", http.MethodPost, "/api/v2/clients", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, proxy.BaseURL+tt.path, nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer "+proxy.Secret)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			if tt.wantStatus == 0 {
				assert.NotEqual(t, http.StatusForbidden, resp.StatusCode)
			} else {
				assert.Equal(t, tt.wantStatus, resp.StatusCode)
			}
		})
	}
}

func TestProxyScopeEnforcementOverrides(t *testing.T) {
	provider := &stubProvider{token: "tok"}
	proxy, err := StartProxy(context.Background(), "127.0.0.1:1", provider, []string{"read:tenant_settings"})
	require.NoError(t, err)
	defer proxy.Close()

	client := loopbackClient(t, proxy.CACertPath)

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int // 0 means "any status other than 403".
	}{
		{"declared override scope passes the gate", http.MethodGet, "/api/v2/tenants/settings", 0},
		{"first-segment scope is still blocked", http.MethodGet, "/api/v2/tenants", http.StatusForbidden},
		{"undeclared write on the override is blocked", http.MethodPatch, "/api/v2/tenants/settings", http.StatusForbidden},
		{"other override scopes are blocked", http.MethodGet, "/api/v2/emails/provider", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, proxy.BaseURL+tt.path, nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer "+proxy.Secret)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			if tt.wantStatus == 0 {
				assert.NotEqual(t, http.StatusForbidden, resp.StatusCode)
			} else {
				assert.Equal(t, tt.wantStatus, resp.StatusCode)
			}
		})
	}
}

func TestProxyRejectsBadSecret(t *testing.T) {
	provider := &stubProvider{token: "tok"}
	proxy, err := StartProxy(context.Background(), "127.0.0.1:1", provider, nil)
	require.NoError(t, err)
	defer proxy.Close()

	client := loopbackClient(t, proxy.CACertPath)

	req, err := http.NewRequest(http.MethodGet, proxy.BaseURL+"/api/v2/clients", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer not-the-secret")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// loopbackClient returns an HTTP client that trusts the proxy's per-run CA.
func loopbackClient(t *testing.T, caPath string) *http.Client {
	t.Helper()
	caPEM, err := os.ReadFile(caPath)
	require.NoError(t, err)
	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM(caPEM))
	return &http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}},
	}
}

// upstreamRecorder stands in for the Management API and records what it
// receives, so tests can assert both what was forwarded and what never was.
type upstreamRecorder struct {
	mu       sync.Mutex
	requests []recordedRequest
}

type recordedRequest struct {
	method string
	uri    string
	header http.Header
}

func (u *upstreamRecorder) snapshot() []recordedRequest {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]recordedRequest(nil), u.requests...)
}

// startProxyWithUpstream starts the proxy in front of a recording TLS upstream.
func startProxyWithUpstream(t *testing.T, allowedScopes []string) (*Proxy, *upstreamRecorder) {
	t.Helper()

	recorder := &upstreamRecorder{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder.mu.Lock()
		recorder.requests = append(recorder.requests, recordedRequest{method: r.Method, uri: r.RequestURI, header: r.Header.Clone()})
		recorder.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	proxy, err := startProxy(strings.TrimPrefix(server.URL, "https://"), &stubProvider{token: "tok"}, allowedScopes, server.Client().Transport)
	require.NoError(t, err)
	t.Cleanup(proxy.Close)

	return proxy, recorder
}

// rawRoundTrip sends a hand-built HTTP/1.1 request so the request target and
// method reach the proxy byte for byte, bypassing the Go client's path cleaning.
func rawRoundTrip(t *testing.T, proxy *Proxy, method, target string, extraHeaders ...string) int {
	t.Helper()

	caPEM, err := os.ReadFile(proxy.CACertPath)
	require.NoError(t, err)
	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM(caPEM))

	addr := strings.TrimPrefix(proxy.BaseURL, "https://")
	conn, err := tls.Dial("tcp", addr, &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12})
	require.NoError(t, err)
	defer conn.Close()

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s %s HTTP/1.1\r\nHost: %s\r\nAuthorization: Bearer %s\r\nConnection: close\r\n", method, target, addr, proxy.Secret)
	for _, h := range extraHeaders {
		sb.WriteString(h + "\r\n")
	}
	sb.WriteString("\r\n")

	_, err = io.WriteString(conn, sb.String())
	require.NoError(t, err)

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	return resp.StatusCode
}

func TestProxyRejectsPathBypasses(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		target     string
		wantStatus int
	}{
		{"dot segments climbing out of a permitted path", http.MethodGet, "/api/v2/tenants/settings/../../clients", http.StatusBadRequest},
		{"encoded dot segments", http.MethodGet, "/api/v2/tenants/settings/%2e%2e/%2e%2e/clients", http.StatusBadRequest},
		{"upper-case encoded dot segments", http.MethodGet, "/api/v2/tenants/settings/%2E%2E/clients", http.StatusBadRequest},
		{"encoded slash in a segment", http.MethodGet, "/api/v2/tenants%2Fsettings", http.StatusBadRequest},
		{"lower-case encoded slash", http.MethodGet, "/api/v2/tenants%2fsettings", http.StatusBadRequest},
		{"encoded backslash", http.MethodGet, "/api/v2/tenants%5Csettings", http.StatusBadRequest},
		{"double slash after the prefix", http.MethodGet, "/api/v2//clients", http.StatusBadRequest},
		{"leading double slash", http.MethodGet, "//api/v2/clients", http.StatusBadRequest},
		{"dot segment into the management path from outside", http.MethodGet, "/foo/../api/v2/clients", http.StatusBadRequest},
		{"trailing dot segment", http.MethodGet, "/api/v2/tenants/settings/..", http.StatusBadRequest},
		{"upper-case prefix is still enforced", http.MethodGet, "/API/v2/clients", http.StatusForbidden},
		{"mixed-case prefix is still enforced", http.MethodGet, "/api/V2/clients", http.StatusForbidden},
		{"upper-case resource is still enforced", http.MethodGet, "/api/v2/Clients", http.StatusForbidden},
		{"empty resource fails closed", http.MethodGet, "/api/v2/", http.StatusForbidden},
		{"bare management root fails closed", http.MethodGet, "/api/v2", http.StatusForbidden},
		{"lower-case method fails closed", "get", "/api/v2/clients", http.StatusForbidden},
		{"unknown method fails closed", "OPTIONS", "/api/v2/tenants/settings", http.StatusForbidden},
		{"trace fails closed", "TRACE", "/api/v2/tenants/settings", http.StatusForbidden},
		{"path parameter after the prefix is rejected", http.MethodGet, "/api/v2;x/users", http.StatusBadRequest},
		{"path parameter inside a path is rejected", http.MethodGet, "/api/v2/tenants/settings;x", http.StatusBadRequest},
		{"encoded path parameter is rejected", http.MethodGet, "/api/v2/tenants/settings%3bx", http.StatusBadRequest},
		{"look-alike prefix fails closed", http.MethodGet, "/api/v2x/users", http.StatusForbidden},
		{"non management path fails closed when scopes are declared", http.MethodGet, "/oauth/token", http.StatusForbidden},
		{"authorize fails closed when scopes are declared", http.MethodGet, "/authorize", http.StatusForbidden},
		{"encoded null byte", http.MethodGet, "/api/v2/tenants/settings%00", http.StatusBadRequest},
		{"double-encoded dot segments", http.MethodGet, "/api/v2/tenants/settings/%252e%252e/clients", http.StatusBadRequest},
		{"encoded percent sign", http.MethodGet, "/api/v2/tenants/settings%25", http.StatusBadRequest},
		{"undeclared scope on a permitted-looking path", http.MethodGet, "/api/v2/tenants", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy, upstream := startProxyWithUpstream(t, []string{"read:tenant_settings"})

			assert.Equal(t, tt.wantStatus, rawRoundTrip(t, proxy, tt.method, tt.target))
			assert.Empty(t, upstream.snapshot(), "the upstream must never receive a rejected request")
		})
	}
}

func TestProxyForwardsCanonicalPaths(t *testing.T) {
	tests := []struct {
		name          string
		allowedScopes []string
		method        string
		target        string
		headers       []string
		wantURI       string
	}{
		{"declared path is forwarded", []string{"read:tenant_settings"}, http.MethodGet, "/api/v2/tenants/settings", nil, "/api/v2/tenants/settings"},
		{"trailing slash is preserved", []string{"read:tenant_settings"}, http.MethodGet, "/api/v2/tenants/settings/", nil, "/api/v2/tenants/settings/"},
		{"query string is preserved", []string{"read:tenant_settings"}, http.MethodGet, "/api/v2/tenants/settings?fields=a%2eb&x=1", nil, "/api/v2/tenants/settings?fields=a%2eb&x=1"},
		{"prefix case is normalized before forwarding", []string{"read:tenant_settings"}, http.MethodGet, "/API/v2/tenants/settings", nil, "/api/v2/tenants/settings"},
		{"non management path is forwarded when unbounded", nil, http.MethodGet, "/oauth/token", nil, "/oauth/token"},
		{"look-alike prefix is forwarded when unbounded", nil, http.MethodGet, "/api/v2x/users", nil, "/api/v2x/users"},
		{"unbounded proxy forwards the management root", nil, http.MethodGet, "/api/v2/", nil, "/api/v2/"},
		{"unbounded proxy forwards any resource", nil, http.MethodGet, "/api/v2/clients", nil, "/api/v2/clients"},
		{
			name:          "method override headers are stripped",
			allowedScopes: []string{"read:tenant_settings"},
			method:        http.MethodGet,
			target:        "/api/v2/tenants/settings",
			headers: []string{
				"X-HTTP-Method-Override: DELETE", "X-HTTP-Method: DELETE", "X-Method-Override: DELETE",
				"X_HTTP_Method_Override: DELETE", "x_method_override: DELETE", "X_Http_Method: DELETE",
			},
			wantURI: "/api/v2/tenants/settings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy, upstream := startProxyWithUpstream(t, tt.allowedScopes)

			assert.Equal(t, http.StatusOK, rawRoundTrip(t, proxy, tt.method, tt.target, tt.headers...))

			got := upstream.snapshot()
			require.Len(t, got, 1)
			assert.Equal(t, tt.method, got[0].method)
			assert.Equal(t, tt.wantURI, got[0].uri)
			assert.Equal(t, "Bearer tok", got[0].header.Get("Authorization"), "the real token replaces the proxy secret")
			for name := range got[0].header {
				normalized := strings.ReplaceAll(strings.ToLower(name), "_", "-")
				assert.NotContains(t, []string{"x-http-method-override", "x-http-method", "x-method-override"}, normalized,
					"%s must not reach the upstream", name)
			}
		})
	}
}
