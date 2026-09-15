// Package plugins contains the (POC) building blocks for running external tools
// as Auth0 CLI plugins that reuse the CLI's tenant session without ever receiving
// the raw access token.
package plugins

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
)

// TokenProvider supplies the upstream Management API bearer token to the proxy
// and refreshes it on demand. It is implemented by the CLI so the proxy stays
// decoupled from config/keyring/auth. Token returns the current token; Refresh
// mints a fresh one when the Management API rejects the current token with 401.
type TokenProvider interface {
	// Token returns the access token to use for the next upstream request.
	Token() string
	// Refresh obtains a fresh access token and returns it. It is called at most
	// once per failing token thanks to the proxy's single-flight guard.
	Refresh(ctx context.Context) (string, error)
}

// bearerToken extracts the token from an "Authorization: Bearer <token>" header
// value. The scheme match is case-insensitive per RFC 6750.
func bearerToken(header string) (string, bool) {
	const prefix = "bearer "
	if len(header) < len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	if token == "" {
		return "", false
	}
	return token, true
}

// Proxy is a short-lived, loopback-only HTTPS reverse proxy that injects the
// active tenant's bearer token into every forwarded request. The plugin talks
// to the proxy over TLS (trusting a per-run self-signed cert) and authenticates
// by sending the per-run secret as its own bearer token, so any standard Auth0
// SDK works unchanged: point it at the proxy and set its token to the secret.
// The proxy validates that bearer, then swaps it for the real access token, so
// the plugin can reach the Management API without ever holding the token itself.
type Proxy struct {
	BaseURL    string // Base URL, e.g. https://127.0.0.1:<port>.
	Secret     string // Per-run secret the plugin must send.
	CACertPath string // Path to the per-run CA cert the plugin must trust.
	Domain     string // Active tenant domain.

	server   *http.Server
	listener net.Listener
	caFile   string
}

// StartProxy stands up the auth-injecting proxy for the given tenant domain.
// The provider supplies (and refreshes) the upstream access token. The
// allowedScopes argument bounds the plugin to the Management API operations it
// declared: when non-empty, a request whose derived scope is not in the set is
// rejected with 403 before it is forwarded. Pass nil to forward every request
// unbounded. Call Close when the plugin exits.
func StartProxy(_ context.Context, tenantDomain string, provider TokenProvider, allowedScopes []string) (*Proxy, error) {
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, fmt.Errorf("failed to generate proxy secret: %w", err)
	}
	secret := hex.EncodeToString(secretBytes)

	tlsCert, certPEM, err := selfSignedLoopbackCert()
	if err != nil {
		return nil, err
	}

	caFile, err := os.CreateTemp("", "auth0-cli-plugin-ca-*.pem")
	if err != nil {
		return nil, fmt.Errorf("failed to create CA temp file: %w", err)
	}
	if err := os.Chmod(caFile.Name(), 0o600); err != nil {
		return nil, fmt.Errorf("failed to secure CA temp file: %w", err)
	}
	if _, err := caFile.Write(certPEM); err != nil {
		return nil, fmt.Errorf("failed to write CA temp file: %w", err)
	}
	if err := caFile.Close(); err != nil {
		return nil, fmt.Errorf("failed to close CA temp file: %w", err)
	}

	target := &url.URL{Scheme: "https", Host: tenantDomain}
	reverseProxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			r.Out.Host = tenantDomain
			// The real bearer token is injected (and refreshed on 401) by the
			// authTransport, so the plugin's per-run secret never leaves here.
		},
		Transport: &authTransport{base: newUpstreamTransport(), provider: provider},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		presented, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok || subtle.ConstantTimeCompare([]byte(presented), []byte(secret)) != 1 {
			http.Error(w, "forbidden: missing or invalid proxy bearer token", http.StatusForbidden)
			return
		}

		if scope, ok := requiredScopeFor(r.Method, r.URL.Path); ok && len(allowedScopes) > 0 && !slices.Contains(allowedScopes, scope) {
			http.Error(w, fmt.Sprintf("forbidden: this plugin did not declare the %q scope required for %s %s", scope, r.Method, r.URL.Path), http.StatusForbidden)
			return
		}

		reverseProxy.ServeHTTP(w, r)
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to bind loopback listener: %w", err)
	}

	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		TLSConfig:         &tls.Config{Certificates: []tls.Certificate{tlsCert}, MinVersion: tls.VersionTLS12},
	}

	go func() {
		// ServeTLS uses the in-memory cert from TLSConfig when cert/key files are empty.
		_ = server.ServeTLS(listener, "", "")
	}()

	port := listener.Addr().(*net.TCPAddr).Port
	return &Proxy{
		BaseURL:    fmt.Sprintf("https://127.0.0.1:%d", port),
		Secret:     secret,
		CACertPath: caFile.Name(),
		Domain:     tenantDomain,
		server:     server,
		listener:   listener,
		caFile:     caFile.Name(),
	}, nil
}

// Env returns the standard proxy env contract to hand to the plugin process.
func (p *Proxy) Env() []string {
	return []string{
		"AUTH0_CLI_PROXY_URL=" + p.BaseURL,
		"AUTH0_CLI_PROXY_TOKEN=" + p.Secret,
		"AUTH0_CLI_TENANT_DOMAIN=" + p.Domain,
		"AUTH0_CLI_PROXY_CA=" + p.CACertPath,
		"NODE_EXTRA_CA_CERTS=" + p.CACertPath,
	}
}

// Close shuts down the proxy and removes the temp CA file.
func (p *Proxy) Close() {
	if p.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = p.server.Shutdown(ctx)
	}
	if p.caFile != "" {
		_ = os.Remove(p.caFile)
	}
}

// authTransport injects the upstream bearer token into each forwarded request
// and, when the Management API rejects it with 401, refreshes the token once and
// retries. Concurrent 401s collapse into a single refresh (single-flight): the
// first goroutine refreshes, the rest observe the already-rotated token and skip
// straight to the retry.
type authTransport struct {
	base     http.RoundTripper
	provider TokenProvider
	mu       sync.Mutex
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Buffer the body so the request can be replayed on retry.
	var body []byte
	if req.Body != nil {
		var err error
		body, err = io.ReadAll(req.Body)
		_ = req.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to buffer request body: %w", err)
		}
	}

	attempt := func(token string) (*http.Response, error) {
		out := req.Clone(req.Context())
		if body != nil {
			out.Body = io.NopCloser(bytes.NewReader(body))
			out.ContentLength = int64(len(body))
		}
		out.Header.Set("Authorization", "Bearer "+token)
		return t.base.RoundTrip(out)
	}

	token := t.provider.Token()
	resp, err := attempt(token)
	if err != nil || resp.StatusCode != http.StatusUnauthorized {
		return resp, err
	}

	newToken, refreshErr := t.refresh(req.Context(), token)
	if refreshErr != nil || newToken == token {
		// Refresh failed or the token did not change; surface the original 401.
		return resp, nil
	}

	// Discard the 401 body before replaying with the refreshed token.
	_ = resp.Body.Close()
	return attempt(newToken)
}

// refresh rotates the token at most once per failing value. If another goroutine
// already refreshed (the current token differs from the one that failed), it
// returns the current token without calling the provider again.
func (t *authTransport) refresh(ctx context.Context, failed string) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if current := t.provider.Token(); current != failed {
		return current, nil
	}
	return t.provider.Refresh(ctx)
}

// newUpstreamTransport returns the transport used to reach the tenant's real
// Management API. It clones the default transport so it honors the system trust
// store and standard proxy/timeout settings.
func newUpstreamTransport() http.RoundTripper {
	if base, ok := http.DefaultTransport.(*http.Transport); ok {
		return base.Clone()
	}
	return http.DefaultTransport
}

// requiredScopeFor derives the Management API scope a request needs from its
// method and path, e.g. GET /api/v2/clients -> "read:clients", DELETE
// /api/v2/roles/{id} -> "delete:roles". It reports false for paths outside the
// Management API v2 surface (which are not scope-enforced). The mapping is
// deliberately coarse: the resource is the first path segment (hyphens
// normalized to underscores, matching Auth0's scope naming), and the verb comes
// from the HTTP method.
func requiredScopeFor(method, path string) (string, bool) {
	const prefix = "/api/v2/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}

	resource := strings.TrimPrefix(path, prefix)
	if i := strings.IndexByte(resource, '/'); i >= 0 {
		resource = resource[:i]
	}
	resource = strings.ReplaceAll(resource, "-", "_")
	if resource == "" {
		return "", false
	}

	var verb string
	switch method {
	case http.MethodGet, http.MethodHead:
		verb = "read"
	case http.MethodPost:
		verb = "create"
	case http.MethodPut, http.MethodPatch:
		verb = "update"
	case http.MethodDelete:
		verb = "delete"
	default:
		return "", false
	}

	return verb + ":" + resource, true
}

// selfSignedLoopbackCert generates a per-run self-signed cert valid for 127.0.0.1
// and localhost. Because it is self-signed with IsCA set, the same PEM serves as
// both the server cert and the CA the plugin trusts.
func selfSignedLoopbackCert() (tls.Certificate, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("failed to generate key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("failed to generate serial: %w", err)
	}

	template := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "Auth0 CLI Plugin Proxy"},
		NotBefore:             time.Now().Add(-1 * time.Minute),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("failed to marshal key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("failed to build key pair: %w", err)
	}

	return tlsCert, certPEM, nil
}
