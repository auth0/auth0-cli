package proxy

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins/registry"
)

const testDomain = "example.com"

// seenRequest is a request that reached the fake Management API.
type seenRequest struct {
	method     string
	requestURI string
	host       string
	header     http.Header
	body       []byte
}

// upstream is a fake Management API that records what reaches it.
type upstream struct {
	server *httptest.Server

	// The closed count is how many connections to the fake were closed.
	closed atomic.Int32

	mu   sync.Mutex
	seen []seenRequest
}

// newUpstream starts a fake Management API that answers with handler.
func newUpstream(t *testing.T, handler http.HandlerFunc) *upstream {
	t.Helper()

	u := &upstream{}

	u.server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		u.mu.Lock()
		u.seen = append(u.seen, seenRequest{method: r.Method, requestURI: r.RequestURI, host: r.Host, header: r.Header.Clone(), body: body})
		u.mu.Unlock()

		r.Body = io.NopCloser(bytes.NewReader(body))
		handler(w, r)
	}))
	u.server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateClosed {
			u.closed.Add(1)
		}
	}
	u.server.StartTLS()
	t.Cleanup(u.server.Close)

	return u
}

// transport returns a transport that sends every request to the fake, which it
// trusts, whatever host the request is for.
func (u *upstream) transport() http.RoundTripper {
	pool := x509.NewCertPool()
	pool.AddCert(u.server.Certificate())

	return &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: pool},
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, u.server.Listener.Addr().String())
		},
	}
}

// requests returns what reached the fake so far.
func (u *upstream) requests() []seenRequest {
	u.mu.Lock()
	defer u.mu.Unlock()

	return append([]seenRequest(nil), u.seen...)
}

// fakeTokens is a token source that counts how often it was asked to refresh.
type fakeTokens struct {
	mu         sync.Mutex
	current    string
	refreshTo  string
	tokenErr   error
	refreshErr error
	refreshes  int
}

func (f *fakeTokens) Token(context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.current, f.tokenErr
}

func (f *fakeTokens) Refresh(context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.refreshes++
	if f.refreshErr != nil {
		return "", f.refreshErr
	}

	f.current = f.refreshTo

	return f.current, nil
}

func (f *fakeTokens) refreshCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.refreshes
}

// scopeSet returns the scopes as the set that a command declares.
func scopeSet(scopes ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		set[scope] = struct{}{}
	}

	return set
}

// okJSON answers every request with an empty JSON object.
func okJSON(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{}`)
}

// testEnv is a running proxy, the fake it forwards to, and a client that trusts it.
type testEnv struct {
	t        *testing.T
	proxy    *Proxy
	upstream *upstream
	tokens   *fakeTokens
	pool     *x509.CertPool
	client   *http.Client
}

// startProxy starts a proxy in front of a fake Management API that answers with
// handler, for a command that declared scopes.
func startProxy(t *testing.T, scopes []string, handler http.HandlerFunc, change ...func(*Config)) *testEnv {
	t.Helper()

	u := newUpstream(t, handler)
	tokens := &fakeTokens{current: "real-token", refreshTo: "new-token"}

	cfg := Config{
		PluginID:  "auth0/checkmate",
		Domain:    testDomain,
		Tokens:    tokens,
		Policy:    testPolicy(t),
		Scopes:    scopes,
		Dir:       t.TempDir(),
		Transport: u.transport(),
	}
	for _, apply := range change {
		apply(&cfg)
	}

	p, err := Start(t.Context(), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Close() })

	// The client only trusts what the plugin is told to trust.
	pem, err := os.ReadFile(envValue(t, p.Env(), "AUTH0_PLUGIN_CA_CERT"))
	require.NoError(t, err)

	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM(pem))

	return &testEnv{
		t: t, proxy: p, upstream: u, tokens: tokens, pool: pool,
		client: &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}, Timeout: 20 * time.Second},
	}
}

// envValue returns the value of a variable of an environment.
func envValue(t *testing.T, env []string, name string) string {
	t.Helper()

	for _, entry := range env {
		if value, ok := strings.CutPrefix(entry, name+"="); ok {
			return value
		}
	}

	require.Failf(t, "variable not found", "%s", name)

	return ""
}

// do makes a call to the proxy as the plugin would.
func (e *testEnv) do(method, target string, body io.Reader, header ...string) (*http.Response, []byte) {
	e.t.Helper()

	req, err := http.NewRequest(method, "https://"+e.proxy.addr+target, body)
	require.NoError(e.t, err)

	req.Header.Set("Authorization", "Bearer "+e.proxy.secret)

	for n := 0; n+1 < len(header); n += 2 {
		req.Header.Add(header[n], header[n+1])
	}

	resp, err := e.client.Do(req)
	require.NoError(e.t, err)

	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	require.NoError(e.t, err)

	return resp, content
}

// raw sends the lines as a request, exactly as they are, and reads the answer.
func (e *testEnv) raw(lines ...string) (*http.Response, []byte) {
	e.t.Helper()

	conn, err := tls.Dial("tcp", e.proxy.addr, &tls.Config{RootCAs: e.pool})
	require.NoError(e.t, err)

	defer conn.Close()

	require.NoError(e.t, conn.SetDeadline(time.Now().Add(10*time.Second)))

	_, err = io.WriteString(conn, strings.Join(lines, "\r\n")+"\r\nConnection: close\r\n\r\n")
	require.NoError(e.t, err)

	method, _, _ := strings.Cut(lines[0], " ")

	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: method})
	require.NoError(e.t, err)

	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	require.NoError(e.t, err)

	return resp, content
}

// get makes a call to the proxy without its secret, which is enough to find out
// whether the connection works.
func get(t *testing.T, client *http.Client, addr, target string) (*http.Response, error) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://"+addr+target, nil)
	require.NoError(t, err)

	return client.Do(req)
}

// errorBody decodes the error body of the Management API.
func errorBody(t *testing.T, content []byte) map[string]any {
	t.Helper()

	var body map[string]any
	require.NoError(t, json.Unmarshal(content, &body), string(content))

	return body
}

func TestCheckBearer(t *testing.T) {
	const secret = "0123456789abcdef"

	header := func(values ...string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/api/v2/clients", nil)
		r.Header["Authorization"] = values

		return r
	}

	tests := []struct {
		name string
		r    *http.Request
		want bool
	}{
		{name: "the secret", r: header("Bearer " + secret), want: true},
		{name: "no header", r: header()},
		{name: "a wrong secret", r: header("Bearer 0123456789abcdee")},
		{name: "a prefix of the secret", r: header("Bearer 0123456789abcde")},
		{name: "the secret and more", r: header("Bearer " + secret + "0")},
		{name: "no secret", r: header("Bearer ")},
		{name: "another scheme", r: header("Basic " + secret)},
		{name: "a lowercase scheme", r: header("bearer " + secret)},
		{name: "no scheme", r: header(secret)},
		{name: "the secret twice", r: header("Bearer "+secret, "Bearer "+secret)},
		{name: "a wrong secret first", r: header("Bearer wrong", "Bearer "+secret)},
		{name: "the secret in the query", r: httptest.NewRequest(http.MethodGet, "/api/v2/clients?access_token="+secret, nil)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rej := checkBearer(test.r, secret)

			if test.want {
				assert.Nil(t, rej)
				return
			}

			require.NotNil(t, rej)
			assert.Equal(t, http.StatusUnauthorized, rej.status)
			assert.Equal(t, codeInvalidProxyToken, rej.code)
			assert.NotContains(t, rej.message, secret)
		})
	}
}

func TestCheckRequestLine(t *testing.T) {
	const addr = "127.0.0.1:4000"

	tests := []struct {
		name       string
		method     string
		requestURI string
		host       string
		wantStatus int
	}{
		{name: "an ordinary call", method: "GET", requestURI: "/api/v2/clients", host: addr},
		{name: "every forwarded method", method: "PATCH", requestURI: "/api/v2/clients/a", host: addr},
		{name: "CONNECT", method: "CONNECT", requestURI: "example.com:443", host: addr, wantStatus: 400},
		{name: "an absolute URL to the tenant", method: "GET", requestURI: "https://example.com/api/v2/clients", host: addr, wantStatus: 400},
		{name: "an absolute URL to another host", method: "GET", requestURI: "http://evil.test/api/v2/clients", host: addr, wantStatus: 400},
		{name: "the tenant as Host", method: "GET", requestURI: "/api/v2/clients", host: "example.com", wantStatus: 400},
		{name: "another host as Host", method: "GET", requestURI: "/api/v2/clients", host: "evil.test", wantStatus: 400},
		{name: "localhost as Host", method: "GET", requestURI: "/api/v2/clients", host: "localhost:4000", wantStatus: 400},
		{name: "another port as Host", method: "GET", requestURI: "/api/v2/clients", host: "127.0.0.1:4001", wantStatus: 400},
		{name: "no Host", method: "GET", requestURI: "/api/v2/clients", wantStatus: 400},
		{name: "TRACE", method: "TRACE", requestURI: "/api/v2/clients", host: addr, wantStatus: 405},
		{name: "OPTIONS", method: "OPTIONS", requestURI: "/api/v2/clients", host: addr, wantStatus: 405},
		{name: "HEAD", method: "HEAD", requestURI: "/api/v2/clients", host: addr, wantStatus: 405},
		{name: "an unknown method", method: "PROPFIND", requestURI: "/api/v2/clients", host: addr, wantStatus: 405},
		{name: "a method in lowercase", method: "get", requestURI: "/api/v2/clients", host: addr, wantStatus: 405},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := &http.Request{Method: test.method, RequestURI: test.requestURI, Host: test.host}

			rej := checkRequestLine(r, addr)

			if test.wantStatus == 0 {
				assert.Nil(t, rej)
				return
			}

			require.NotNil(t, rej)
			assert.Equal(t, test.wantStatus, rej.status)

			if test.wantStatus == http.StatusMethodNotAllowed {
				assert.Equal(t, "GET, POST, PUT, PATCH, DELETE", rej.header.Get("Allow"))
			}
		})
	}
}

func TestCheckScope(t *testing.T) {
	policy := testPolicy(t)

	tests := []struct {
		name        string
		declared    map[string]struct{}
		method      string
		target      string
		wantStatus  int
		wantCode    errorCode
		wantMessage string
	}{
		{name: "a declared scope", declared: scopeSet("read:users"), method: "GET", target: "/api/v2/users"},
		{name: "a declared scope and others", declared: scopeSet("read:clients", "read:users"), method: "GET", target: "/api/v2/users/abc"},
		{name: "the other alternative", declared: scopeSet("read:client_keys"), method: "GET", target: "/api/v2/clients/abc"},
		{name: "only the summary scope", declared: scopeSet("read:client_summary"), method: "GET", target: "/api/v2/clients/abc"},
		{name: "a parameter in camel case", declared: scopeSet("read:actions"), method: "GET", target: "/api/v2/actions/actions/act_123/versions/v1"},
		{name: "an identifier in mixed case", declared: scopeSet("read:clients"), method: "GET", target: "/api/v2/clients/AbC123xYz"},
		{name: "a user identifier in mixed case", declared: scopeSet("read:users"), method: "GET", target: "/api/v2/users/auth0%7CABC123"},
		{
			name: "an undeclared scope", declared: scopeSet("read:clients"), method: "GET", target: "/api/v2/users",
			wantStatus: 403, wantCode: codeInsufficientScope,
			wantMessage: `Plugin auth0/checkmate did not declare scope "read:users" required for GET /api/v2/users. Add it to permissions.scopes in plugin.json.`,
		},
		{
			name: "an undeclared scope with alternatives", declared: scopeSet("read:users"), method: "GET", target: "/api/v2/clients/abc",
			wantStatus: 403, wantCode: codeInsufficientScope,
			wantMessage: `Plugin auth0/checkmate did not declare scope "read:clients" (or: read:client_keys, read:client_credentials, read:client_summary) required for GET /api/v2/clients/abc. Add it to permissions.scopes in plugin.json.`,
		},
		{name: "no scopes at all", declared: scopeSet(), method: "GET", target: "/api/v2/users", wantStatus: 403, wantCode: codeInsufficientScope},
		{name: "a nil set of scopes", method: "GET", target: "/api/v2/users", wantStatus: 403, wantCode: codeInsufficientScope},
		{name: "an endpoint that is not in the table", declared: scopeSet("read:users", "read:clients"), method: "GET", target: "/api/v2/actions", wantStatus: 403, wantCode: codeNoScopeRule},
		{name: "a method the rule does not list", declared: scopeSet("read:users", "delete:users"), method: "DELETE", target: "/api/v2/users/abc", wantStatus: 403, wantCode: codeNoScopeRule},
		{name: "an extra segment", declared: scopeSet("read:users"), method: "GET", target: "/api/v2/users/abc/def/ghi", wantStatus: 403, wantCode: codeNoScopeRule},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, rej := parseTarget(test.target)
			require.Nil(t, rej)

			rej = checkScope(policy, test.declared, "auth0/checkmate", test.method, parsed)

			if test.wantStatus == 0 {
				assert.Nil(t, rej)
				return
			}

			require.NotNil(t, rej)
			assert.Equal(t, test.wantStatus, rej.status)
			assert.Equal(t, test.wantCode, rej.code)

			if test.wantMessage != "" {
				assert.Equal(t, test.wantMessage, rej.message)
			}

			if test.wantCode == codeInsufficientScope {
				assert.NotEmpty(t, rej.scope, "the summary names the missing scope")
			}
		})
	}
}

// organizationsPolicy is a policy with two rules that overlap when letter case is
// ignored: the members of an organization by identifier, and an organization by
// name.
func organizationsPolicy(t *testing.T) *Policy {
	t.Helper()

	policy, err := NewPolicy(indexWith([]registry.ScopeRule{
		{Method: "GET", Path: "/api/v2/organizations/{id}/members", AnyOf: []string{"read:organization_members"}},
		{Method: "GET", Path: "/api/v2/organizations/name/{name}", AnyOf: []string{"read:organizations"}},
	}, "client_secret"))
	require.NoError(t, err)

	return policy
}

func TestCheckScope_RejectsACallThatCaseCouldRouteToAnotherRule(t *testing.T) {
	policy := organizationsPolicy(t)
	both := scopeSet("read:organization_members", "read:organizations")

	tests := []struct {
		name     string
		declared map[string]struct{}
		target   string
		wantCode errorCode
	}{
		{name: "the literal in capitals", declared: both, target: "/api/v2/organizations/NAME/members", wantCode: codeBadRequest},
		{name: "the literal with one capital", declared: both, target: "/api/v2/organizations/Name/members", wantCode: codeBadRequest},
		{name: "no scope declared does not hide it", declared: scopeSet(), target: "/api/v2/organizations/NAME/members", wantCode: codeBadRequest},
		{name: "only the scope of the matching rule", declared: scopeSet("read:organization_members"), target: "/api/v2/organizations/NAME/members", wantCode: codeBadRequest},
		{name: "an identifier that matches only one rule", declared: both, target: "/api/v2/organizations/org_ABC/members"},
		{name: "the literal as the identifier", declared: both, target: "/api/v2/organizations/name/members"},
		{name: "the rule by name", declared: both, target: "/api/v2/organizations/name/Acme"},
		{name: "a call that no rule matches", declared: both, target: "/api/v2/organizations/NAME/other", wantCode: codeNoScopeRule},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, rej := parseTarget(test.target)
			require.Nil(t, rej)

			rej = checkScope(policy, test.declared, "auth0/checkmate", "GET", parsed)

			if test.wantCode == "" {
				assert.Nil(t, rej)
				return
			}

			require.NotNil(t, rej)
			assert.Equal(t, test.wantCode, rej.code)

			if test.wantCode == codeBadRequest {
				assert.Equal(t, http.StatusBadRequest, rej.status)
				assert.Contains(t, rej.message, "ambiguous")
			}
		})
	}
}

func TestProxy_DeniesAnAmbiguousCall(t *testing.T) {
	e := startProxy(t, []string{"read:organization_members", "read:organizations"}, okJSON, func(c *Config) { c.Policy = organizationsPolicy(t) })

	resp, content := e.do(http.MethodGet, "/api/v2/organizations/NAME/members", nil)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "bad_request", errorBody(t, content)["errorCode"])
	assert.Contains(t, errorBody(t, content)["message"], "ambiguous")
	assert.Empty(t, e.upstream.requests())
	assert.Equal(t, "Plugin auth0/checkmate: 1 API call was denied (GET /api/v2/organizations/NAME/members). Review permissions.scopes in plugin.json.",
		e.proxy.Summary())

	resp, _ = e.do(http.MethodGet, "/api/v2/organizations/org_ABC/members", nil)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Len(t, e.upstream.requests(), 1)
}

func TestReadRequestBody(t *testing.T) {
	tests := []struct {
		name       string
		body       io.Reader
		length     int64
		wantStatus int
	}{
		{name: "no body", body: http.NoBody},
		{name: "a body of the limit", body: strings.NewReader(strings.Repeat("a", maxRequestBody)), length: maxRequestBody},
		{name: "a length above the limit", body: strings.NewReader("a"), length: maxRequestBody + 1, wantStatus: 413},
		{name: "a body above the limit with no length", body: strings.NewReader(strings.Repeat("a", maxRequestBody+1)), length: -1, wantStatus: 413},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/v2/clients", test.body)
			r.ContentLength = test.length

			body, rej := readRequestBody(httptest.NewRecorder(), r)

			if test.wantStatus != 0 {
				require.NotNil(t, rej)
				assert.Equal(t, test.wantStatus, rej.status)

				return
			}

			require.Nil(t, rej)
			assert.Len(t, body, int(test.length))
		})
	}
}

func TestCheckDomain(t *testing.T) {
	tests := []struct {
		domain string
		want   string
	}{
		{domain: "example.us.auth0.com", want: "example.us.auth0.com"},
		{domain: "Example.EU.auth0.com", want: "example.eu.auth0.com"},
		{domain: "tenant.example.com", want: "tenant.example.com"},
		{domain: "a-b.example.com", want: "a-b.example.com"},
		{domain: ""},
		{domain: "localhost"},
		{domain: "https://example.com"},
		{domain: "example.com:443"},
		{domain: "example.com/api"},
		{domain: "user@example.com"},
		{domain: "user:pass@example.com"},
		{domain: "example.com?x=1"},
		{domain: "example.com#x"},
		{domain: "example.com."},
		{domain: ".example.com"},
		{domain: "a..example.com"},
		{domain: "-a.example.com"},
		{domain: "a-.example.com"},
		{domain: "exa mple.com"},
		{domain: "exa_mple.com"},
		{domain: "example.com\n"},
		{domain: "exämple.com"},
		{domain: "127.0.0.1"},
		{domain: "10.0.0.1"},
		{domain: "[::1]"},
		{domain: strings.Repeat("a", 64) + ".com"},
		{domain: strings.Repeat("a.", 130) + "com"},
	}

	for _, test := range tests {
		t.Run(test.domain, func(t *testing.T) {
			got, err := checkDomain(test.domain)

			if test.want == "" {
				require.ErrorIs(t, err, ErrInvalidDomain)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestStart_RejectsAnUnsafeConfig(t *testing.T) {
	good := func() Config {
		return Config{PluginID: "auth0/checkmate", Domain: testDomain, Tokens: &fakeTokens{}, Policy: testPolicy(t), Dir: t.TempDir()}
	}

	tests := []struct {
		name    string
		change  func(c *Config)
		wantErr error
	}{
		{name: "a bad domain", change: func(c *Config) { c.Domain = "https://example.com" }, wantErr: ErrInvalidDomain},
		{name: "no domain", change: func(c *Config) { c.Domain = "" }, wantErr: ErrInvalidDomain},
		{name: "no policy", change: func(c *Config) { c.Policy = nil }, wantErr: ErrNoScopeTable},
		{name: "an empty policy", change: func(c *Config) { c.Policy = &Policy{} }, wantErr: ErrNoScopeTable},
		{name: "a policy without secret fields", change: func(c *Config) { c.Policy = &Policy{rules: testPolicy(t).rules} }, wantErr: ErrNoSecretFields},
		{name: "no token source", change: func(c *Config) { c.Tokens = nil }},
		{name: "no plugin ID", change: func(c *Config) { c.PluginID = "" }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := good()
			test.change(&cfg)

			p, err := Start(t.Context(), cfg)

			assert.Nil(t, p)
			require.Error(t, err)

			if test.wantErr != nil {
				assert.ErrorIs(t, err, test.wantErr)
			}

			entries, readErr := os.ReadDir(cfg.Dir)
			require.NoError(t, readErr)
			assert.Empty(t, entries, "nothing is left behind")
		})
	}
}

func TestProxy_RejectsEveryBypass(t *testing.T) {
	e := startProxy(t, []string{"read:clients", "read:users", "delete:clients", "update:clients", "create:clients"}, okJSON)

	tests := []struct {
		name       string
		request    string
		host       string
		wantStatus int
		wantCode   string
	}{
		{name: "dot dot", request: "GET /api/v2/../oauth/token HTTP/1.1", wantStatus: 400},
		{name: "a dot dot between resources", request: "GET /api/v2/clients/../users HTTP/1.1", wantStatus: 400},
		{name: "escaped dot dot", request: "GET /api/v2/%2e%2e/users HTTP/1.1", wantStatus: 400},
		{name: "escaped dot dot in capitals", request: "GET /api/v2/%2E%2E/users HTTP/1.1", wantStatus: 400},
		{name: "an escaped slash", request: "GET /api/v2/clients%2fusers HTTP/1.1", wantStatus: 400},
		{name: "an escaped backslash", request: "GET /api/v2/clients%5Cusers HTTP/1.1", wantStatus: 400},
		{name: "a null byte", request: "GET /api/v2/clients%00 HTTP/1.1", wantStatus: 400},
		{name: "an escaped percent sign", request: "GET /api/v2/%252e%252e/users HTTP/1.1", wantStatus: 400},
		{name: "a double slash", request: "GET /api/v2//clients HTTP/1.1", wantStatus: 400},
		{name: "a trailing slash", request: "GET /api/v2/clients/ HTTP/1.1", wantStatus: 400},
		{name: "path parameters", request: "GET /api/v2/clients;x=1 HTTP/1.1", wantStatus: 400},
		{name: "the prefix in capitals", request: "GET /API/V2/clients HTTP/1.1", wantStatus: 403, wantCode: "forbidden_path"},
		{name: "the prefix with a capital", request: "GET /api/V2/clients HTTP/1.1", wantStatus: 403, wantCode: "forbidden_path"},
		{name: "a look-alike prefix", request: "GET /api/v2x/clients HTTP/1.1", wantStatus: 403, wantCode: "forbidden_path"},
		{name: "the prefix alone", request: "GET /api/v2 HTTP/1.1", wantStatus: 403, wantCode: "forbidden_path"},
		{name: "the token endpoint", request: "POST /oauth/token HTTP/1.1", wantStatus: 403, wantCode: "forbidden_path"},
		{name: "the userinfo endpoint", request: "GET /userinfo HTTP/1.1", wantStatus: 403, wantCode: "forbidden_path"},
		{name: "an endpoint that is not in the table", request: "GET /api/v2/actions HTTP/1.1", wantStatus: 403, wantCode: "no_scope_rule"},
		{name: "an extra segment", request: "GET /api/v2/users/a/b/c HTTP/1.1", wantStatus: 403, wantCode: "no_scope_rule"},
		{name: "a method the rule does not list", request: "PUT /api/v2/clients/a HTTP/1.1", wantStatus: 403, wantCode: "no_scope_rule"},
		{name: "an absolute URL to the tenant", request: "GET https://example.com/api/v2/clients HTTP/1.1", wantStatus: 400},
		{name: "an absolute URL to another host", request: "GET http://evil.test/api/v2/clients HTTP/1.1", wantStatus: 400},
		{name: "CONNECT", request: "CONNECT example.com:443 HTTP/1.1", wantStatus: 400},
		{name: "the tenant as Host", request: "GET /api/v2/clients HTTP/1.1", host: testDomain, wantStatus: 400},
		{name: "another host as Host", request: "GET /api/v2/clients HTTP/1.1", host: "evil.test", wantStatus: 400},
		{name: "TRACE", request: "TRACE /api/v2/clients HTTP/1.1", wantStatus: 405},
		{name: "OPTIONS", request: "OPTIONS /api/v2/clients HTTP/1.1", wantStatus: 405},
		{name: "HEAD", request: "HEAD /api/v2/clients HTTP/1.1", wantStatus: 405},
		{name: "an unknown method", request: "PROPFIND /api/v2/clients HTTP/1.1", wantStatus: 405},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			host := test.host
			if host == "" {
				host = e.proxy.addr
			}

			resp, content := e.raw(test.request, "Host: "+host, "Authorization: Bearer "+e.proxy.secret)

			assert.Equal(t, test.wantStatus, resp.StatusCode, string(content))
			assert.Empty(t, e.upstream.requests(), "nothing reaches the Management API")

			if test.wantStatus == http.StatusMethodNotAllowed {
				assert.NotEmpty(t, resp.Header.Get("Allow"))
			}

			if test.wantCode != "" {
				assert.Equal(t, test.wantCode, errorBody(t, content)["errorCode"])
			}
		})
	}
}

func TestProxy_RejectsAWrongProxyToken(t *testing.T) {
	e := startProxy(t, []string{"read:clients"}, okJSON)
	secret := e.proxy.secret

	tests := []struct {
		name  string
		lines []string
	}{
		{name: "no token", lines: []string{"GET /api/v2/clients HTTP/1.1", "Host: " + e.proxy.addr}},
		{name: "a wrong token", lines: []string{"GET /api/v2/clients HTTP/1.1", "Host: " + e.proxy.addr, "Authorization: Bearer " + strings.Repeat("0", 64)}},
		{name: "the token in the query", lines: []string{"GET /api/v2/clients?access_token=" + secret + " HTTP/1.1", "Host: " + e.proxy.addr}},
		{name: "the token in another header", lines: []string{"GET /api/v2/clients HTTP/1.1", "Host: " + e.proxy.addr, "X-Token: " + secret}},
		{name: "a wrong token before a bad request", lines: []string{"GET /api/v2/../x HTTP/1.1", "Host: evil.test", "Authorization: Bearer wrong"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resp, content := e.raw(test.lines...)

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
			assert.Equal(t, "invalid_proxy_token", errorBody(t, content)["errorCode"])
			assert.NotContains(t, string(content), secret)
		})
	}

	assert.Empty(t, e.upstream.requests())
	assert.Empty(t, e.proxy.Summary(), "a request without the token is not the plugin, so it is not a denial")
}

func TestProxy_Forwards(t *testing.T) {
	e := startProxy(t, []string{"read:clients", "update:clients"}, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	})

	resp, content := e.do(http.MethodPatch, "/api/v2/clients/auth0%7C123?fields=a%2Cb&include_fields=true", strings.NewReader(`{"name":"x"}`),
		"Content-Type", "application/json",
		"Accept", "application/json",
		"User-Agent", "checkmate/1.0",
		"Auth0-Client", "eyJuYW1lIjoiY2hlY2ttYXRlIn0",
		"Cookie", "session=abc",
		"X-Forwarded-For", "10.0.0.1",
		"X-HTTP-Method-Override", "DELETE",
		"X-Method-Override", "DELETE",
		"Accept-Encoding", "br",
		"Proxy-Authorization", "Basic abc",
		"X-Custom", "yes",
	)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.JSONEq(t, `{"ok":true}`, string(content))

	seen := e.upstream.requests()
	require.Len(t, seen, 1)
	got := seen[0]

	assert.Equal(t, "PATCH", got.method)
	assert.Equal(t, "/api/v2/clients/auth0%7C123?fields=a%2Cb&include_fields=true", got.requestURI)
	assert.Equal(t, testDomain, got.host)
	assert.JSONEq(t, `{"name":"x"}`, string(got.body))

	assert.Equal(t, "Bearer real-token", got.header.Get("Authorization"), "the real token replaces the secret")
	assert.NotContains(t, strings.Join(got.header.Values("Authorization"), " "), e.proxy.secret)
	assert.Equal(t, "application/json", got.header.Get("Content-Type"))
	assert.Equal(t, "application/json", got.header.Get("Accept"))
	assert.Equal(t, "checkmate/1.0", got.header.Get("User-Agent"))
	assert.Equal(t, "eyJuYW1lIjoiY2hlY2ttYXRlIn0", got.header.Get("Auth0-Client"))

	for _, dropped := range []string{"Cookie", "X-Forwarded-For", "X-Http-Method-Override", "X-Method-Override", "Proxy-Authorization", "X-Custom"} {
		assert.Empty(t, got.header.Values(dropped), dropped)
	}

	assert.NotEqual(t, "br", got.header.Get("Accept-Encoding"), "the transport asks for what it can decode")
	assert.Empty(t, e.proxy.Summary())
}

func TestProxy_EnforcesScopes(t *testing.T) {
	tests := []struct {
		name       string
		scopes     []string
		method     string
		target     string
		wantStatus int
		wantCode   string
	}{
		{name: "a declared scope", scopes: []string{"read:users"}, method: "GET", target: "/api/v2/users", wantStatus: 200},
		{name: "one alternative of two", scopes: []string{"read:client_keys"}, method: "GET", target: "/api/v2/clients/abc", wantStatus: 200},
		{name: "only the summary scope", scopes: []string{"read:client_summary"}, method: "GET", target: "/api/v2/clients/abc", wantStatus: 200},
		{name: "a parameter in camel case", scopes: []string{"read:actions"}, method: "GET", target: "/api/v2/actions/actions/act_123/versions/v1", wantStatus: 200},
		{name: "an undeclared scope", scopes: []string{"read:clients"}, method: "GET", target: "/api/v2/users", wantStatus: 403, wantCode: "insufficient_scope"},
		{name: "a read scope does not allow a delete", scopes: []string{"read:clients"}, method: "DELETE", target: "/api/v2/clients/abc", wantStatus: 403, wantCode: "insufficient_scope"},
		{name: "no scopes declared", method: "GET", target: "/api/v2/users", wantStatus: 403, wantCode: "insufficient_scope"},
		{name: "an endpoint that is not in the table", scopes: []string{"read:users"}, method: "GET", target: "/api/v2/actions", wantStatus: 403, wantCode: "no_scope_rule"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := startProxy(t, test.scopes, okJSON)

			resp, content := e.do(test.method, test.target, nil)

			assert.Equal(t, test.wantStatus, resp.StatusCode, string(content))

			if test.wantStatus == http.StatusOK {
				assert.Len(t, e.upstream.requests(), 1)
				return
			}

			assert.Empty(t, e.upstream.requests())

			body := errorBody(t, content)
			assert.Equal(t, test.wantCode, body["errorCode"])
			assert.Equal(t, "Forbidden", body["error"])
			assert.EqualValues(t, 403, body["statusCode"])
			assert.Contains(t, body["message"], "auth0/checkmate")
		})
	}
}

func TestProxy_Summary(t *testing.T) {
	e := startProxy(t, []string{"read:clients"}, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"statusCode":403,"error":"Forbidden","message":"Insufficient scope"}`)
	})

	assert.Empty(t, e.proxy.Summary())

	e.do(http.MethodGet, "/api/v2/users", nil)
	e.do(http.MethodGet, "/api/v2/users", nil)
	e.do(http.MethodGet, "/oauth/token", nil)

	resp, _ := e.do(http.MethodGet, "/api/v2/clients", nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "Auth0 refused and the proxy passes that on")

	assert.Equal(t, "Plugin auth0/checkmate: 3 API calls were denied (read:users for GET /api/v2/users, GET /oauth/token) and Auth0 refused 1 call because your session lacks a scope. Review permissions.scopes in plugin.json.",
		e.proxy.Summary())
}

func TestProxy_ResponseHandling(t *testing.T) {
	gzipped := func(s string) []byte {
		var buf bytes.Buffer
		w := gzip.NewWriter(&buf)
		_, _ = w.Write([]byte(s))
		_ = w.Close()

		return buf.Bytes()
	}

	tests := []struct {
		name        string
		handler     http.HandlerFunc
		wantStatus  int
		wantBody    string
		wantCode    string
		wantHeaders map[string]string
	}{
		{
			name: "secrets are removed",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				_, _ = io.WriteString(w, `{"client_id":"a","client_secret":"s","jwt_configuration":{"secret_encoded":true},"signing_keys":[{"key":"k"}],"app":{"options":{"Client_Secret":"x","scope":"openid"}}}`)
			},
			wantStatus: 200,
			wantBody:   `{"client_id":"a","jwt_configuration":{"secret_encoded":true},"app":{"options":{"scope":"openid"}}}`,
		},
		{
			name: "a content type that ends in +json",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/problem+json")
				_, _ = io.WriteString(w, `{"private_key":"k","title":"t"}`)
			},
			wantStatus: 200,
			wantBody:   `{"title":"t"}`,
		},
		{
			name: "an error body is cleaned too",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"statusCode":400,"message":"bad","client_secret":"s"}`)
			},
			wantStatus: 400,
			wantBody:   `{"statusCode":400,"message":"bad"}`,
		},
		{
			name: "a gzip body is decoded and cleaned",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Content-Encoding", "gzip")
				_, _ = w.Write(gzipped(`{"client_secret":"s","a":1}`))
			},
			wantStatus:  200,
			wantBody:    `{"a":1}`,
			wantHeaders: map[string]string{"Content-Encoding": ""},
		},
		{
			name: "cookies and hop by hop headers are dropped, the length is recomputed",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Set-Cookie", "session=abc")
				w.Header().Set("Keep-Alive", "timeout=5")
				w.Header().Set("Connection", "X-Hop")
				w.Header().Set("X-Hop", "1")
				w.Header().Set("X-Ratelimit-Remaining", "9")
				_, _ = io.WriteString(w, `{"a":1,"client_secret":"a long secret value"}`)
			},
			wantStatus: 200,
			wantBody:   `{"a":1}`,
			wantHeaders: map[string]string{
				"Set-Cookie": "", "Keep-Alive": "", "X-Hop": "", "Content-Length": "7", "X-Ratelimit-Remaining": "9",
			},
		},
		{
			name:       "no content",
			handler:    func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) },
			wantStatus: 204,
		},
		{
			name:        "an empty body",
			handler:     func(w http.ResponseWriter, _ *http.Request) { w.Header().Set("Content-Type", "text/plain") },
			wantStatus:  200,
			wantHeaders: map[string]string{"Content-Length": "0"},
		},
		{
			name: "a body that is not JSON",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				_, _ = io.WriteString(w, `<html>client_secret=abc</html>`)
			},
			wantStatus: 502,
			wantCode:   "unsupported_response",
		},
		{
			name: "a body with no content type",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header()["Content-Type"] = nil
				_, _ = io.WriteString(w, `{"client_secret":"s"}`)
			},
			wantStatus: 502,
			wantCode:   "unsupported_response",
		},
		{
			name: "a binary body that says it is JSON",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte{0x00, 0x01, 0xff})
			},
			wantStatus: 502,
			wantCode:   "unsupported_response",
		},
		{
			name: "an encoding that cannot be decoded",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Content-Encoding", "br")
				_, _ = io.WriteString(w, `{"client_secret":"s"}`)
			},
			wantStatus: 502,
			wantCode:   "unsupported_response",
		},
		{
			name: "a body above the limit",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				// Valid JSON in the first bytes, so that only the size can reject it.
				_, _ = io.WriteString(w, "{}"+strings.Repeat(" ", maxResponseBody))
			},
			wantStatus: 502,
			wantCode:   "unsupported_response",
		},
		{
			name: "a redirect",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "https://evil.test/steal", http.StatusFound)
			},
			wantStatus: 502,
			wantCode:   "upstream_redirect",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := startProxy(t, []string{"read:clients"}, test.handler)

			resp, content := e.do(http.MethodGet, "/api/v2/clients", nil)

			assert.Equal(t, test.wantStatus, resp.StatusCode, string(content))

			switch {
			case test.wantCode != "":
				assert.Equal(t, test.wantCode, errorBody(t, content)["errorCode"])
				assert.NotContains(t, string(content), "client_secret")
				assert.NotContains(t, resp.Header.Get("Location"), "evil")
			case test.wantBody != "":
				assert.Equal(t, test.wantBody, string(content))
			default:
				assert.Empty(t, content)
			}

			for name, want := range test.wantHeaders {
				if want == "" {
					assert.Empty(t, resp.Header.Values(name), name)
				} else {
					assert.Equal(t, want, resp.Header.Get(name), name)
				}
			}

			if test.name == "a redirect" {
				assert.Len(t, e.upstream.requests(), 1, "the redirect is not followed")
			}
		})
	}
}

func TestProxy_KeepsLargeNumbersExact(t *testing.T) {
	e := startProxy(t, []string{"read:clients"}, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":9007199254740993,"ratio":1.10}`)
	})

	_, content := e.do(http.MethodGet, "/api/v2/clients", nil)

	assert.Equal(t, `{"id":9007199254740993,"ratio":1.10}`, string(content))
}

func TestProxy_RefreshesTheTokenOnce(t *testing.T) {
	const callers = 20

	t.Run("one refresh for many calls that failed with the same token", func(t *testing.T) {
		// Only maxInFlight calls are in flight at once, and these are the ones that
		// fail with the first token. The others find the new token when they run.
		var arrived sync.WaitGroup

		arrived.Add(maxInFlight)

		release := make(chan struct{})

		e := startProxy(t, []string{"read:clients"}, func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == "Bearer real-token" {
				arrived.Done()
				<-release
				w.WriteHeader(http.StatusUnauthorized)

				return
			}

			okJSON(w, r)
		})

		statuses := make(chan int, callers)

		var wg sync.WaitGroup

		for range callers {
			wg.Go(func() {
				resp, _ := e.do(http.MethodGet, "/api/v2/clients", nil)
				statuses <- resp.StatusCode
			})
		}

		everyoneWaiting := make(chan struct{})
		go func() { arrived.Wait(); close(everyoneWaiting) }()

		select {
		case <-everyoneWaiting:
		case <-time.After(15 * time.Second):
			t.Fatal("the calls did not all reach the Management API")
		}

		close(release)
		wg.Wait()
		close(statuses)

		for status := range statuses {
			assert.Equal(t, http.StatusOK, status)
		}

		assert.Equal(t, 1, e.tokens.refreshCount())
		assert.Len(t, e.upstream.requests(), callers+maxInFlight, "every call that failed was sent again once")
	})

	t.Run("a second 401 is not tried again", func(t *testing.T) {
		e := startProxy(t, []string{"read:clients"}, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})

		resp, content := e.do(http.MethodGet, "/api/v2/clients", nil)

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		assert.Equal(t, "session_expired", errorBody(t, content)["errorCode"])
		assert.Equal(t, 1, e.tokens.refreshCount())
		assert.Len(t, e.upstream.requests(), 2)
	})

	tests := []struct {
		name   string
		change func(f *fakeTokens)
	}{
		{name: "a session that cannot be renewed", change: func(f *fakeTokens) { f.refreshErr = ErrReloginRequired }},
		{name: "a refresh that fails", change: func(f *fakeTokens) { f.refreshErr = errors.New("the tenant is down") }},
		{name: "a refresh that gives the same token", change: func(f *fakeTokens) { f.refreshTo = "real-token" }},
	}

	for _, test := range tests {
		t.Run(test.name+" gives session_expired without a replay", func(t *testing.T) {
			e := startProxy(t, []string{"read:clients"}, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			})
			test.change(e.tokens)

			resp, content := e.do(http.MethodGet, "/api/v2/clients", nil)

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

			body := errorBody(t, content)
			assert.Equal(t, "session_expired", body["errorCode"])
			assert.Contains(t, body["message"], "auth0 login")
			assert.NotContains(t, string(content), "real-token")
			assert.Len(t, e.upstream.requests(), 1)
		})
	}

	t.Run("a token that cannot be read gives session_expired", func(t *testing.T) {
		e := startProxy(t, []string{"read:clients"}, okJSON)
		e.tokens.tokenErr = errors.New("keyring is locked")

		resp, content := e.do(http.MethodGet, "/api/v2/clients", nil)

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		assert.Equal(t, "session_expired", errorBody(t, content)["errorCode"])
		assert.Empty(t, e.upstream.requests())
	})
}

func TestProxy_ServesOnlyOverTheTrustedCertificate(t *testing.T) {
	e := startProxy(t, []string{"read:clients"}, okJSON)

	t.Run("a client that trusts the certificate", func(t *testing.T) {
		resp, _ := e.do(http.MethodGet, "/api/v2/clients", nil)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "HTTP/1.1", resp.Proto)
	})

	t.Run("a client that does not", func(t *testing.T) {
		client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: x509.NewCertPool()}}}

		_, err := get(t, client, e.proxy.addr, "/api/v2/clients")

		var unknown x509.UnknownAuthorityError
		require.ErrorAs(t, err, &unknown)
	})

	t.Run("a client that asks for HTTP/2 still gets HTTP/1.1", func(t *testing.T) {
		client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: e.pool}, ForceAttemptHTTP2: true}}

		resp, err := get(t, client, e.proxy.addr, "/userinfo")
		require.NoError(t, err)

		defer resp.Body.Close()

		assert.Equal(t, 1, resp.ProtoMajor)
	})

	t.Run("a client that only speaks TLS 1.1", func(t *testing.T) {
		conn, err := tls.Dial("tcp", e.proxy.addr, &tls.Config{RootCAs: e.pool, MaxVersion: tls.VersionTLS11})
		if err == nil {
			_ = conn.Close()
		}

		assert.Error(t, err)
	})
}

func TestProxy_Env(t *testing.T) {
	e := startProxy(t, nil, okJSON)
	env := e.proxy.Env()

	host, port, err := net.SplitHostPort(envValue(t, env, "AUTH0_PLUGIN_DOMAIN"))
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", host)
	assert.NotEmpty(t, port)

	secret := envValue(t, env, "AUTH0_PLUGIN_MANAGEMENT_TOKEN")
	assert.Regexp(t, `^[0-9a-f]{64}$`, secret)
	assert.NotEqual(t, "real-token", secret)

	ca := envValue(t, env, "AUTH0_PLUGIN_CA_CERT")
	assert.Equal(t, ca, envValue(t, env, "SSL_CERT_FILE"))
	assert.Equal(t, ca, envValue(t, env, "NODE_EXTRA_CA_CERTS"))
	assert.Len(t, env, 5)

	other := startProxy(t, nil, okJSON)
	assert.NotEqual(t, secret, envValue(t, other.proxy.Env(), "AUTH0_PLUGIN_MANAGEMENT_TOKEN"), "every run has its own secret")
}

func TestProxy_Close(t *testing.T) {
	t.Run("removes the certificate and stops serving", func(t *testing.T) {
		e := startProxy(t, []string{"read:clients"}, okJSON)
		ca := envValue(t, e.proxy.Env(), "AUTH0_PLUGIN_CA_CERT")

		require.FileExists(t, ca)
		require.NoError(t, e.proxy.Close())
		assert.NoFileExists(t, ca)
		assert.NoDirExists(t, filepath.Dir(ca))
		assert.NoError(t, e.proxy.Close(), "closing twice is fine")

		_, err := get(t, e.client, e.proxy.addr, "/api/v2/clients")
		assert.Error(t, err)
	})

	t.Run("leaves the directory it was given", func(t *testing.T) {
		parent := t.TempDir()
		e := startProxy(t, nil, okJSON, func(c *Config) { c.Dir = parent })

		ca := envValue(t, e.proxy.Env(), "AUTH0_PLUGIN_CA_CERT")
		assert.Equal(t, parent, filepath.Dir(filepath.Dir(ca)))

		require.NoError(t, e.proxy.Close())
		assert.DirExists(t, parent)

		entries, err := os.ReadDir(parent)
		require.NoError(t, err)
		assert.Empty(t, entries)
	})

	t.Run("happens when the context ends", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		p, err := Start(ctx, Config{
			PluginID:  "auth0/checkmate",
			Domain:    testDomain,
			Tokens:    &fakeTokens{},
			Policy:    testPolicy(t),
			Dir:       t.TempDir(),
			Transport: newUpstream(t, okJSON).transport(),
		})
		require.NoError(t, err)

		ca := envValue(t, p.Env(), "AUTH0_PLUGIN_CA_CERT")
		require.FileExists(t, ca)

		cancel()

		assert.Eventually(t, func() bool {
			_, err := os.Stat(ca)

			return errors.Is(err, os.ErrNotExist)
		}, 5*time.Second, 10*time.Millisecond)

		assert.NoError(t, p.Close())
	})
}

func TestProxy_CertificateFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes do not apply on Windows")
	}

	e := startProxy(t, nil, okJSON)
	ca := envValue(t, e.proxy.Env(), "AUTH0_PLUGIN_CA_CERT")

	file, err := os.Stat(ca)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), file.Mode().Perm())

	dir, err := os.Stat(filepath.Dir(ca))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), dir.Mode().Perm())
}

func TestNewTransport(t *testing.T) {
	transport := newTransport()

	assert.False(t, transport.ForceAttemptHTTP2)
	assert.NotNil(t, transport.TLSNextProto)
	assert.Empty(t, transport.TLSNextProto)
	assert.False(t, transport.DisableCompression)
}

func TestNewTransport_OffersHTTP1OnlyOverTLS(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	t.Cleanup(server.Close)

	pool := x509.NewCertPool()
	pool.AddCert(server.Certificate())

	// A default transport that has been used already offers h2 in its TLS config.
	warmed := &http.Transport{
		ForceAttemptHTTP2: true,
		TLSClientConfig:   &tls.Config{RootCAs: pool, NextProtos: []string{"h2", "http/1.1"}},
	}

	original := http.DefaultTransport
	http.DefaultTransport = warmed
	t.Cleanup(func() { http.DefaultTransport = original })

	transport := newTransport()
	t.Cleanup(transport.CloseIdleConnections)

	require.NotNil(t, transport.TLSClientConfig)
	assert.Equal(t, []string{"http/1.1"}, transport.TLSClientConfig.NextProtos)

	resp, err := (&http.Client{Transport: transport}).Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "ok", string(body))
	assert.Equal(t, 1, resp.ProtoMajor)

	// The shared default transport keeps what it offered.
	assert.Contains(t, warmed.TLSClientConfig.NextProtos, "h2")
}

func TestProxy_LimitsTheCallsInFlight(t *testing.T) {
	const callers = 3 * maxInFlight

	var (
		mu               sync.Mutex
		inFlight, peaked int
	)

	e := startProxy(t, []string{"read:clients"}, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		inFlight++
		peaked = max(peaked, inFlight)
		mu.Unlock()

		time.Sleep(200 * time.Millisecond)

		mu.Lock()
		inFlight--
		mu.Unlock()

		okJSON(w, r)
	})

	var wg sync.WaitGroup

	for range callers {
		wg.Go(func() {
			resp, _ := e.do(http.MethodGet, "/api/v2/clients", nil)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}

	wg.Wait()

	assert.Len(t, e.upstream.requests(), callers, "every call is answered in the end")
	mu.Lock()
	defer mu.Unlock()

	assert.LessOrEqual(t, peaked, maxInFlight)
	assert.Empty(t, e.proxy.slots, "every slot was given back")
}

func TestProxy_ServerTimeouts(t *testing.T) {
	e := startProxy(t, nil, okJSON)

	assert.Equal(t, 60*time.Second, e.proxy.server.ReadTimeout)
	assert.Equal(t, 60*time.Second, e.proxy.server.IdleTimeout)
}

func TestProxy_forward_WaitsForASlotBeforeReadingTheBody(t *testing.T) {
	e := startProxy(t, []string{"read:clients"}, okJSON)

	// Every slot is taken, so the next call has to wait.
	for range maxInFlight {
		e.proxy.slots <- struct{}{}
	}

	ctx, cancel := context.WithCancel(t.Context())
	body := &countingBody{}

	r := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v2/clients", body)
	target, rej := parseTarget("/api/v2/clients")
	require.Nil(t, rej)

	done := make(chan struct{})

	go func() {
		defer close(done)

		e.proxy.forward(httptest.NewRecorder(), r, target)
	}()

	select {
	case <-done:
		t.Fatal("the call did not wait for a slot")
	case <-time.After(200 * time.Millisecond):
	}

	assert.Zero(t, body.reads.Load(), "the body is not read while the call waits")

	cancel()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("a call that waits for a slot did not stop when its request was cancelled")
	}

	assert.Zero(t, body.reads.Load())
	assert.Len(t, e.proxy.slots, maxInFlight, "the call that gave up took no slot and gave none back")
	assert.Empty(t, e.upstream.requests())
}

// countingBody is a request body that counts how often it is read.
type countingBody struct {
	reads atomic.Int32
}

func (b *countingBody) Read([]byte) (int, error) {
	b.reads.Add(1)

	return 0, io.EOF
}

func TestProxy_RefreshesAtMostThreeTimes(t *testing.T) {
	e := startProxy(t, []string{"read:clients"}, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	// Every refresh gives a token that the tenant rejects again.
	for n := range 3 * maxRefreshes {
		e.tokens.mu.Lock()
		e.tokens.refreshTo = fmt.Sprintf("token-%d", n)
		e.tokens.mu.Unlock()

		resp, content := e.do(http.MethodGet, "/api/v2/clients", nil)

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		assert.Equal(t, "session_expired", errorBody(t, content)["errorCode"])
	}

	assert.Equal(t, maxRefreshes, e.tokens.refreshCount())

	requests := len(e.upstream.requests())
	e.do(http.MethodGet, "/api/v2/clients", nil)
	assert.Equal(t, requests+1, len(e.upstream.requests()), "after the limit a 401 is answered without a replay")
}

// idleClosingTransport is a transport that counts how often it is told to close
// its idle connections.
type idleClosingTransport struct {
	http.RoundTripper
	closes atomic.Int32
}

func (t *idleClosingTransport) CloseIdleConnections() {
	t.closes.Add(1)
}

func TestProxy_Close_ClosesOnlyTheTransportItBuilt(t *testing.T) {
	t.Run("one the caller supplied is left alone", func(t *testing.T) {
		given := &idleClosingTransport{}
		e := startProxy(t, []string{"read:clients"}, okJSON, func(c *Config) { c.Transport = given })

		require.NoError(t, e.proxy.Close())
		assert.Zero(t, given.closes.Load())
	})

	t.Run("its own is closed", func(t *testing.T) {
		e := startProxy(t, []string{"read:clients"}, okJSON, func(c *Config) { c.Transport = nil })

		// Aim the transport that the proxy built at the fake.
		owned := e.proxy.owned
		require.NotNil(t, owned)
		require.Same(t, owned, e.proxy.transport)

		given := e.upstream.transport().(*http.Transport)
		owned.Proxy = nil
		owned.DialContext = given.DialContext
		owned.TLSClientConfig = given.TLSClientConfig

		resp, _ := e.do(http.MethodGet, "/api/v2/clients", nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Zero(t, e.upstream.closed.Load(), "the connection to the fake is idle and open")

		require.NoError(t, e.proxy.Close())

		assert.Eventually(t, func() bool { return e.upstream.closed.Load() > 0 }, 5*time.Second, 10*time.Millisecond,
			"the idle connection to the tenant is closed")
	})
}

// funcTokens is a token source whose answers come from functions.
type funcTokens struct {
	token   func(ctx context.Context) (string, error)
	refresh func(ctx context.Context) (string, error)
}

func (f *funcTokens) Token(ctx context.Context) (string, error) { return f.token(ctx) }

func (f *funcTokens) Refresh(ctx context.Context) (string, error) { return f.refresh(ctx) }

// untilDone blocks until ctx ends, as a token source that waits on something
// that never happens would.
func untilDone(ctx context.Context) (string, error) {
	<-ctx.Done()

	return "", ctx.Err()
}

// rejectEverything answers every request with 401.
func rejectEverything(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusUnauthorized)
}

func TestProxy_SharesOneRefreshThatTakesTime(t *testing.T) {
	const callers = 10

	// The calls are held at the tenant until all of them were rejected with the
	// first token, so that they all reach the refresh while the first one is in it.
	var arrived sync.WaitGroup

	arrived.Add(callers)

	release := make(chan struct{})

	source := &fakeTokens{current: "real-token", refreshTo: "new-token"}
	tokens := &funcTokens{
		token: source.Token,
		refresh: func(ctx context.Context) (string, error) {
			select {
			case <-time.After(300 * time.Millisecond):
			case <-ctx.Done():
				return "", ctx.Err()
			}

			return source.Refresh(ctx)
		},
	}

	e := startProxy(t, []string{"read:clients"}, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer real-token" {
			arrived.Done()
			<-release
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		okJSON(w, r)
	}, func(c *Config) { c.Tokens = tokens })

	statuses := make(chan int, callers)

	var wg sync.WaitGroup

	for range callers {
		wg.Go(func() {
			resp, _ := e.do(http.MethodGet, "/api/v2/clients", nil)
			statuses <- resp.StatusCode
		})
	}

	everyoneWaiting := make(chan struct{})
	go func() { arrived.Wait(); close(everyoneWaiting) }()

	select {
	case <-everyoneWaiting:
	case <-time.After(15 * time.Second):
		t.Fatal("the calls did not all reach the Management API")
	}

	close(release)
	wg.Wait()
	close(statuses)

	for status := range statuses {
		assert.Equal(t, http.StatusOK, status)
	}

	assert.Equal(t, 1, source.refreshCount(), "the calls shared the one refresh")
}

// hangingTransport is a transport that never answers until the request ends.
type hangingTransport struct{}

func (hangingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	<-r.Context().Done()

	return nil, r.Context().Err()
}

func TestProxy_StopsWaitingForATenantThatHangs(t *testing.T) {
	e := startProxy(t, []string{"read:clients"}, okJSON, func(c *Config) { c.Transport = hangingTransport{} })
	e.proxy.timeout = 50 * time.Millisecond
	e.client.Timeout = 5 * time.Second

	resp, content := e.do(http.MethodGet, "/api/v2/clients", nil)

	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
	assert.Equal(t, "upstream_failed", errorBody(t, content)["errorCode"])
	assert.Eventually(t, func() bool { return len(e.proxy.slots) == 0 }, 5*time.Second, 10*time.Millisecond,
		"the slot of the call was given back")
}

func TestProxy_StopsWaitingForATokenSourceThatHangs(t *testing.T) {
	var asked atomic.Int32

	tests := []struct {
		name         string
		tokens       *funcTokens
		wantRequests int
	}{
		{
			name:   "when it is asked for the token",
			tokens: &funcTokens{token: untilDone},
		},
		{
			name: "when it is asked for the token again during a refresh",
			tokens: &funcTokens{token: func(ctx context.Context) (string, error) {
				if asked.Add(1) == 1 {
					return "real-token", nil
				}

				return untilDone(ctx)
			}},
			wantRequests: 1,
		},
		{
			name:         "when it is asked to refresh",
			tokens:       &funcTokens{token: func(context.Context) (string, error) { return "real-token", nil }, refresh: untilDone},
			wantRequests: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := startProxy(t, []string{"read:clients"}, rejectEverything, func(c *Config) { c.Tokens = test.tokens })
			e.proxy.timeout = 50 * time.Millisecond
			e.client.Timeout = 5 * time.Second

			resp, content := e.do(http.MethodGet, "/api/v2/clients", nil)

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
			assert.Equal(t, "session_expired", errorBody(t, content)["errorCode"])
			assert.Len(t, e.upstream.requests(), test.wantRequests)

			require.True(t, e.proxy.refreshMu.TryLock(), "the refresh mutex was given back")
			e.proxy.refreshMu.Unlock()

			assert.Eventually(t, func() bool { return len(e.proxy.slots) == 0 }, 5*time.Second, 10*time.Millisecond,
				"the slot of the call was given back")
		})
	}
}

func TestProxy_RejectsARefreshThatReturnsATokenWithAnError(t *testing.T) {
	tokens := &funcTokens{
		token:   func(context.Context) (string, error) { return "real-token", nil },
		refresh: func(context.Context) (string, error) { return "new-token", errors.New("the refresh failed halfway") },
	}

	e := startProxy(t, []string{"read:clients"}, rejectEverything, func(c *Config) { c.Tokens = tokens })

	resp, content := e.do(http.MethodGet, "/api/v2/clients", nil)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, "session_expired", errorBody(t, content)["errorCode"])
	assert.Len(t, e.upstream.requests(), 1, "the request was not sent again with the token that came with the error")
}

// endlessTransport is a transport whose responses are far larger than the proxy
// reads, and which counts the bytes the proxy read.
type endlessTransport struct {
	read atomic.Int64
}

func (e *endlessTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       endlessBody{read: &e.read},
	}, nil
}

// endlessBody is a response body of blanks that ends after three times the limit
// of the proxy, so that a proxy without a limit finishes instead of running out
// of memory.
type endlessBody struct {
	read *atomic.Int64
}

func (b endlessBody) Read(p []byte) (int, error) {
	remaining := 3*maxResponseBody - b.read.Load()
	if remaining <= 0 {
		return 0, io.EOF
	}

	n := int(min(int64(len(p)), remaining))
	for i := range p[:n] {
		p[i] = ' '
	}

	b.read.Add(int64(n))

	return n, nil
}

func (endlessBody) Close() error { return nil }

func TestProxy_StopsReadingAResponseThatIsTooLarge(t *testing.T) {
	transport := &endlessTransport{}
	e := startProxy(t, []string{"read:clients"}, okJSON, func(c *Config) { c.Transport = transport })

	resp, content := e.do(http.MethodGet, "/api/v2/clients", nil)

	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
	assert.Equal(t, "unsupported_response", errorBody(t, content)["errorCode"])

	const slack = 64 << 10

	read := transport.read.Load()
	assert.Greater(t, read, int64(maxResponseBody), "the proxy read past the limit to find out that the response is too large")
	assert.LessOrEqual(t, read, int64(maxResponseBody+slack), "the proxy stopped reading near the limit")
}
