package doctor

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testDomain = "shop.example"
	testIssuer = "https://auth.shop.example/"
)

var testHosts = []string{testDomain, "auth.shop.example", "api.shop.example"}

// site is a synthetic Poppy site. A test mutates its documents or routes before serving it.
type site struct {
	poppy    map[string]any
	resource map[string]any
	authz    map[string]any

	// BrowserHeaders and browserBody describe the browser join response.
	browserHeaders http.Header
	browserBody    string
	browserStatus  int

	openapi     string
	openapiType string

	// Routes override the default handling of "<host><path>".
	routes map[string]http.HandlerFunc
}

// validSite is a site that follows protocol 0.4.
func validSite() *site {
	return &site{
		poppy: map[string]any{
			"protocol_version": "0.4",
			"organization": map[string]any{
				"name":     "Shop",
				"domain":   testDomain,
				"logo_uri": "https://shop.example/logo.png",
			},
			"auth": map[string]any{
				"issuer": testIssuer,
				"scopes": map[string]any{
					"orders:read":           "See your orders, returns, and bookings",
					"orders:write":          "Exchange or return items, and change bookings",
					"addresses":             "See and change your saved shipping addresses",
					"poppy:browser-sign-in": "Use the website signed in, with the other access you approve",
				},
				"scope_hints": map[string]any{
					"orders:read":           map[string]any{"changes": "none", "personal_data": true},
					"orders:write":          map[string]any{"changes": "irreversible", "personal_data": true},
					"addresses":             map[string]any{"changes": "reversible", "personal_data": true},
					"poppy:browser-sign-in": map[string]any{"changes": "irreversible", "personal_data": true},
				},
			},
			"agent":          map[string]any{"protocols": []any{map[string]any{"type": "poppy", "endpoint": "https://api.shop.example/poppy/conversations"}}},
			"web":            map[string]any{"browser_session_endpoint": "https://shop.example/poppy/browser-session"},
			"feedback":       map[string]any{"endpoint": "https://api.shop.example/poppy/feedback"},
			"excluded_tasks": map[string]any{"url": "https://shop.example/poppy/excluded-tasks.json"},
			"apis": []any{
				map[string]any{"type": "openapi", "url": "https://api.shop.example/openapi.json"},
				map[string]any{"type": "mcp", "url": "https://mcp.shop.example/mcp"},
				map[string]any{"type": "graphql", "url": "https://api.shop.example/graphql"},
			},
			"extensions": map[string]any{"unknown": true},
		},
		resource: map[string]any{
			"resource":                 "https://shop.example",
			"authorization_servers":    []any{testIssuer},
			"bearer_methods_supported": []any{"header"},
		},
		authz: map[string]any{
			"issuer":                        testIssuer,
			"protected_resources":           []any{"https://shop.example", "https://mcp.shop.example"},
			"authorization_endpoint":        "https://auth.shop.example/authorize",
			"token_endpoint":                "https://auth.shop.example/oauth/token",
			"revocation_endpoint":           "https://auth.shop.example/oauth/revoke",
			"device_authorization_endpoint": "https://auth.shop.example/oauth/device/code",
			"scopes_supported":              []any{"orders:read", "orders:write", "addresses", "poppy:browser-sign-in", "openid"},
			"grant_types_supported":         []any{"authorization_code", "refresh_token", "client_credentials", "urn:ietf:params:oauth:grant-type:device_code"},
			"token_endpoint_auth_signing_alg_values_supported": []any{"RS256", "ES256", "PS384"},
			"authorization_response_iss_parameter_supported":   true,
		},
		browserStatus: http.StatusOK,
		browserBody:   `<html><body><form method="post"><input type="hidden" name="code" value="abc"><button>Join</button></form></body></html>`,
		browserHeaders: http.Header{
			"Set-Cookie":    {"__Host-poppy_join=abc; Path=/; Secure; HttpOnly; SameSite=Strict; Max-Age=60"},
			"Cache-Control": {"no-store"},
		},
		openapi:     `{"openapi":"3.1.0","info":{"title":"Shop","version":"1"}}`,
		openapiType: "application/json",
		routes:      map[string]http.HandlerFunc{},
	}
}

// valid03Site is a site that follows protocol 0.3: no 0.4 members, and none of the
// reserved scope.
func valid03Site() *site {
	s := validSite()
	s.poppy["protocol_version"] = "0.3"
	delete(obj(obj(s.poppy, "auth"), "scopes"), "poppy:browser-sign-in")
	delete(obj(obj(s.poppy, "auth"), "scope_hints"), "poppy:browser-sign-in")
	s.authz["scopes_supported"] = []any{"orders:read", "orders:write", "addresses", "openid"}

	return s
}

func writeJSON(w http.ResponseWriter, doc map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(doc)
}

func (s *site) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}

	key := host + r.URL.Path
	if h, ok := s.routes[key]; ok {
		h(w, r)
		return
	}

	switch key {
	case testDomain + "/.well-known/poppy.json":
		writeJSON(w, s.poppy)
	case testDomain + "/.well-known/oauth-protected-resource":
		writeJSON(w, s.resource)
	case "auth.shop.example/.well-known/oauth-authorization-server":
		writeJSON(w, s.authz)
	case testDomain + "/poppy/browser-session":
		for name, values := range s.browserHeaders {
			for _, v := range values {
				w.Header().Add(name, v)
			}
		}
		w.WriteHeader(s.browserStatus)
		_, _ = w.Write([]byte(s.browserBody))
	case "api.shop.example/openapi.json":
		w.Header().Set("Content-Type", s.openapiType)
		_, _ = w.Write([]byte(s.openapi))
	default:
		http.NotFound(w, r)
	}
}

// run serves the site over TLS, maps every host name to the test server, and
// runs the checks that need no login (D1 to D7).
func (s *site) run(t *testing.T) Report {
	t.Helper()

	report := s.runWith(t, Options{})
	report.Results = slices.DeleteFunc(report.Results, func(res Result) bool { return res.Level == Skipped && res.Message == noLogin })

	return report
}

// runWith is run with more options, and keeps every result.
func (s *site) runWith(t *testing.T, opts Options) Report {
	t.Helper()

	cert, pool := testCertificate(t)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Values("Authorization"), "request to %s carried credentials", r.URL)
		assert.Empty(t, r.Header.Values("Cookie"), "request to %s carried cookies", r.URL)
		s.ServeHTTP(w, r)
	}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
		},
	}}

	opts.HTTPClient = client

	return Run(t.Context(), testDomain, opts)
}

// testCertificate returns a self-signed certificate for the test hosts and a pool that trusts it.
func testCertificate(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "poppy doctor test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              testHosts,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	parsed, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AddCert(parsed)

	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, pool
}
