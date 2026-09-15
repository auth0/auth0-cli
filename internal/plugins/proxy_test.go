package plugins

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"os"
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
