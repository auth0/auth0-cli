// Package proxy runs the loopback HTTPS proxy that a plugin process uses to call
// the Auth0 Management API.
//
// The plugin never receives the access token of the user or machine session. It
// gets a per-run secret instead, and the proxy swaps the secret for the real
// token on the calls it forwards. The proxy only forwards the calls that the
// invoked command declared scopes for, removes secret fields from every response,
// and denies everything else, so a plugin cannot reach more than it asked the
// user to approve.
package proxy

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The limits on what the proxy accepts, buffers and waits for.
const (
	maxRequestBody    = 10 << 20
	maxResponseBody   = 32 << 20
	upstreamTimeout   = 60 * time.Second
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 60 * time.Second
	idleTimeout       = 60 * time.Second
	maxHeaderBytes    = 64 << 10
	shutdownTimeout   = 2 * time.Second

	// The in-flight calls are how many calls the proxy works on at once. The
	// others wait, so that a plugin cannot use up the memory of the CLI with
	// buffered bodies.
	maxInFlight = 16

	// The refreshes are how often the proxy asks the token source for a new
	// token in one run.
	maxRefreshes = 3
)

// requestHeaders are the headers of a request that are forwarded. Every other
// header is dropped, which removes method override headers, cookies, forwarded
// addresses, and the Authorization header of the plugin.
var requestHeaders = []string{"Accept", "Content-Type", "User-Agent", "Auth0-Client"}

// droppedResponseHeaders are the headers of a response that are never passed on.
// Content-Length and Content-Encoding are replaced because the body is rewritten.
var droppedResponseHeaders = []string{
	"Set-Cookie", "Set-Cookie2", "Content-Length", "Content-Encoding",
	"Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization",
	"Proxy-Connection", "Te", "Trailer", "Transfer-Encoding", "Upgrade",
}

// TokenSource supplies the access token of the session that the proxy calls the
// Management API with.
type TokenSource interface {
	// Token returns the current access token.
	Token(ctx context.Context) (string, error)

	// Refresh gets a new access token after the current one was rejected. It
	// returns ErrReloginRequired when the session cannot be renewed without the
	// user.
	Refresh(ctx context.Context) (string, error)
}

// Config is what Start needs to run a proxy.
type Config struct {
	// PluginID names the plugin in messages.
	PluginID string

	// Domain is the tenant domain. It must be a bare host name.
	Domain string

	// Tokens supplies the access token that replaces the secret of the plugin.
	Tokens TokenSource

	// Policy says which calls a scope allows and which fields are secret.
	Policy *Policy

	// Scopes are the scopes that the invoked command declared. A call is only
	// forwarded when one of them allows it, so no scopes means no calls.
	Scopes []string

	// Dir is the directory in which the proxy makes its own directory for the
	// certificate. An empty Dir means the temporary directory of the system.
	Dir string

	// Transport reaches the tenant. It is meant for tests. A nil Transport is a
	// copy of the default transport that only speaks HTTP/1.1.
	Transport http.RoundTripper
}

// Proxy is a running proxy. Stop it with Close.
type Proxy struct {
	pluginID  string
	domain    string
	tokens    TokenSource
	policy    *Policy
	scopes    map[string]struct{}
	transport http.RoundTripper

	// The owned transport is the transport the proxy built itself, which is the
	// only one it closes. It is nil when the caller supplied the transport.
	owned *http.Transport

	// The slots hold one entry for every call in flight.
	slots chan struct{}

	// The timeout is how long one request to the tenant, or one call to the token
	// source, may take. It is the upstream timeout, which tests shorten.
	timeout time.Duration

	// The secret is what the plugin sends as its bearer token.
	secret string

	// The address is that of the listener, which is also the Host that requests
	// must carry.
	addr   string
	server *http.Server
	dir    string
	caPath string

	denials denialLog

	// The refresh mutex makes the calls that failed with the same token share one
	// refresh. It also guards the count of refreshes.
	refreshMu sync.Mutex
	refreshes int

	done      chan struct{}
	closeOnce sync.Once
	closeErr  error
}

// Start starts a proxy on a loopback port and returns it. The proxy stops when
// ctx ends or when Close is called. It returns ErrNoScopeTable or
// ErrNoSecretFields when the policy cannot keep a plugin within its scopes, and
// ErrInvalidDomain when the domain is not a host name.
func Start(ctx context.Context, cfg Config) (*Proxy, error) {
	domain, err := checkDomain(cfg.Domain)
	if err != nil {
		return nil, err
	}

	if err := cfg.Policy.usable(); err != nil {
		return nil, err
	}

	if cfg.Tokens == nil || cfg.PluginID == "" {
		return nil, errors.New("the proxy needs a plugin ID and a token source")
	}

	secret, err := newSecret()
	if err != nil {
		return nil, err
	}

	cert, certPEM, err := newCertificate(time.Now())
	if err != nil {
		return nil, err
	}

	dir, err := os.MkdirTemp(cfg.Dir, "auth0-plugin-")
	if err != nil {
		return nil, fmt.Errorf("failed to create the directory for the proxy certificate: %w", err)
	}

	caPath := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(caPath, certPEM, 0o600); err != nil {
		_ = os.RemoveAll(dir)

		return nil, fmt.Errorf("failed to write the proxy certificate: %w", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = os.RemoveAll(dir)

		return nil, fmt.Errorf("failed to listen on the loopback address: %w", err)
	}

	p := &Proxy{
		pluginID:  cfg.PluginID,
		domain:    domain,
		tokens:    cfg.Tokens,
		policy:    cfg.Policy,
		scopes:    make(map[string]struct{}, len(cfg.Scopes)),
		transport: cfg.Transport,
		slots:     make(chan struct{}, maxInFlight),
		timeout:   upstreamTimeout,
		secret:    secret,
		addr:      listener.Addr().String(),
		dir:       dir,
		caPath:    caPath,
		done:      make(chan struct{}),
	}

	for _, scope := range cfg.Scopes {
		p.scopes[scope] = struct{}{}
	}

	if p.transport == nil {
		p.owned = newTransport()
		p.transport = p.owned
	}

	p.server = &http.Server{
		Handler:           http.HandlerFunc(p.serve),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
		ErrorLog:          log.New(io.Discard, "", 0),

		// An empty map turns HTTP/2 off, so that one parser sees every request.
		TLSNextProto: map[string]func(*http.Server, *tls.Conn, http.Handler){},
	}

	tlsListener := tls.NewListener(listener, &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		NextProtos:   []string{"http/1.1"},
	})

	go func() { _ = p.server.Serve(tlsListener) }()

	go func() {
		select {
		case <-ctx.Done():
			_ = p.Close()
		case <-p.done:
		}
	}()

	return p, nil
}

// Env returns the environment variables that point a plugin at the proxy: the
// address and the secret to call it with, and the certificate to trust, which
// is passed in the variables that Go, Node and Bun read their extra roots from.
func (p *Proxy) Env() []string {
	return []string{
		"AUTH0_PLUGIN_DOMAIN=" + p.addr,
		"AUTH0_PLUGIN_MANAGEMENT_TOKEN=" + p.secret,
		"SSL_CERT_FILE=" + p.caPath,
		"NODE_EXTRA_CA_CERTS=" + p.caPath,
		"AUTH0_PLUGIN_CA_CERT=" + p.caPath,
	}
}

// Summary returns one line that says which calls were denied, or an empty string
// when none was. It counts the calls the proxy denied and, apart from them, the
// calls that Auth0 refused because the session lacks a scope.
func (p *Proxy) Summary() string {
	return p.denials.summary(p.pluginID)
}

// Close stops the proxy, closes the idle connections of the transport it built
// and removes its certificate. It is safe to call twice.
func (p *Proxy) Close() error {
	p.closeOnce.Do(func() {
		close(p.done)

		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if err := p.server.Shutdown(ctx); err != nil {
			_ = p.server.Close()
		}

		// A transport that the caller supplied is the caller's to close.
		if p.owned != nil {
			p.owned.CloseIdleConnections()
		}

		if err := os.RemoveAll(p.dir); err != nil {
			p.closeErr = fmt.Errorf("failed to remove the proxy certificate: %w", err)
		}
	})

	return p.closeErr
}

// serve answers one request, going through the checks in order.
func (p *Proxy) serve(w http.ResponseWriter, r *http.Request) {
	// A request with the wrong secret is not the plugin, so it is not counted as
	// something the plugin was denied.
	if rej := checkBearer(r, p.secret); rej != nil {
		rej.write(w)

		return
	}

	rej := checkRequestLine(r, p.addr)

	var t target
	if rej == nil {
		t, rej = parseTarget(r.RequestURI)
	}

	if rej == nil {
		rej = checkScope(p.policy, p.scopes, p.pluginID, r.Method, t)
	}

	if rej != nil {
		path, _, _ := strings.Cut(r.RequestURI, "?")
		p.denials.deny(rej.scope, r.Method, path)
		rej.write(w)

		return
	}

	p.forward(w, r, t)
}

// checkBearer checks that the request carries the secret of the run as its
// bearer token. The secret is only read from the header, never from the query.
func checkBearer(r *http.Request, secret string) *rejection {
	var presented string

	ok := false
	if values := r.Header.Values("Authorization"); len(values) == 1 {
		presented, ok = strings.CutPrefix(values[0], "Bearer ")
	}

	if ok && subtle.ConstantTimeCompare([]byte(presented), []byte(secret)) == 1 {
		return nil
	}

	rej := reject(http.StatusUnauthorized, codeInvalidProxyToken, "The proxy token is missing or wrong.")
	rej.header = http.Header{}
	rej.header.Set("WWW-Authenticate", "Bearer")

	return rej
}

// checkRequestLine checks the parts of a request that are about where it goes
// and how: it must be an ordinary request to the listener, with one of the
// methods the proxy forwards.
func checkRequestLine(r *http.Request, addr string) *rejection {
	switch {
	case r.Method == http.MethodConnect:
		return reject(http.StatusBadRequest, codeBadRequest, "The proxy does not tunnel connections.")
	case !strings.HasPrefix(r.RequestURI, "/"):
		return reject(http.StatusBadRequest, codeBadRequest,
			"The request target "+quoted(r.RequestURI)+" must be a path, not a URL.")
	case r.Host != addr:
		return reject(http.StatusBadRequest, codeBadRequest,
			"The Host header "+quoted(r.Host)+" must be the address of the proxy, "+addr+".")
	case !slices.Contains(allowedMethods, r.Method):
		rej := reject(http.StatusMethodNotAllowed, codeMethodNotAllowed,
			"The method "+quoted(r.Method)+" is not allowed. Use "+strings.Join(allowedMethods, ", ")+".")
		rej.header = http.Header{}
		rej.header.Set("Allow", strings.Join(allowedMethods, ", "))

		return rej
	}

	return nil
}

// checkScope checks that the call is in the scope table, that no other rule
// could be meant by it, and that the command declared a scope that allows it.
func checkScope(policy *Policy, declared map[string]struct{}, pluginID, method string, t target) *rejection {
	matched, ok := policy.match(method, t.segments)
	if !ok {
		return reject(http.StatusForbidden, codeNoScopeRule, fmt.Sprintf(
			"Plugin %s cannot call %s %s because this call is not in the registry's scope table.",
			safeText(pluginID), method, safeText(t.path)))
	}

	if policy.ambiguous(method, t.segments, matched) {
		return reject(http.StatusBadRequest, codeBadRequest, fmt.Sprintf(
			"Plugin %s cannot call %s %s because the path is ambiguous: it also matches another endpoint when letter case is ignored.",
			safeText(pluginID), method, safeText(t.path)))
	}

	if matched.allowedBy(declared) {
		return nil
	}

	needed := safeText(matched.anyOf[0])

	var others string

	if len(matched.anyOf) > 1 {
		alternatives := make([]string, 0, len(matched.anyOf)-1)
		for _, scope := range matched.anyOf[1:] {
			alternatives = append(alternatives, safeText(scope))
		}

		others = " (or: " + strings.Join(alternatives, ", ") + ")"
	}

	rej := reject(http.StatusForbidden, codeInsufficientScope, fmt.Sprintf(
		"Plugin %s did not declare scope %q%s required for %s %s. Add it to permissions.scopes in plugin.json.",
		safeText(pluginID), needed, others, method, safeText(t.path)))
	rej.scope = needed

	return rej
}

// outgoing is a request to the tenant, ready to be sent more than once.
type outgoing struct {
	method string
	target target
	header http.Header
	body   []byte
}

// upstreamResponse is a response of the tenant, read in full.
type upstreamResponse struct {
	status int
	header http.Header
	body   []byte
}

// forward sends the checked request to the tenant and answers the plugin with
// the response, after the secrets were removed from it.
func (p *Proxy) forward(w http.ResponseWriter, r *http.Request, t target) {
	// A slot is taken before the body is read, so that the calls that wait do not
	// hold a buffered body. A call whose client went away stops waiting.
	select {
	case p.slots <- struct{}{}:
		defer func() { <-p.slots }()
	case <-r.Context().Done():
		return
	}

	body, rej := readRequestBody(w, r)
	if rej != nil {
		rej.write(w)

		return
	}

	request := outgoing{method: r.Method, target: t, header: make(http.Header), body: body}

	for _, name := range requestHeaders {
		if values := r.Header.Values(name); len(values) > 0 {
			request.header[name] = values
		}
	}

	response, rej := p.call(r.Context(), request)
	if rej != nil {
		rej.write(w)

		return
	}

	if response.status == http.StatusForbidden {
		p.denials.refuse()
	}

	cleaned, rej := p.inspect(response)
	if rej != nil {
		rej.write(w)

		return
	}

	writeResponse(w, response, cleaned)
}

// readRequestBody reads the body of the request, up to the limit.
func readRequestBody(w http.ResponseWriter, r *http.Request) ([]byte, *rejection) {
	if r.ContentLength > maxRequestBody {
		return nil, requestTooLarge()
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBody))
	if err != nil {
		var limit *http.MaxBytesError
		if errors.As(err, &limit) {
			return nil, requestTooLarge()
		}

		return nil, reject(http.StatusBadRequest, codeBadRequest, "The request body could not be read.")
	}

	return body, nil
}

// requestTooLarge is what the plugin gets when its request body is over the limit.
func requestTooLarge() *rejection {
	return reject(http.StatusRequestEntityTooLarge, codePayloadTooLarge, "The request body is too large.")
}

// sessionExpired is what the plugin gets when the session was rejected and could
// not be renewed.
func sessionExpired() *rejection {
	return reject(http.StatusUnauthorized, codeSessionExpired,
		"Your Auth0 session has expired or was rejected. Run `auth0 login` and try again.")
}

// call sends the request with the token of the session. When the tenant answers
// 401 it asks for a new token once and sends the request again once. A second 401
// is not tried again.
func (p *Proxy) call(ctx context.Context, request outgoing) (upstreamResponse, *rejection) {
	token, err := p.token(ctx, p.tokens.Token)
	if err != nil || token == "" {
		return upstreamResponse{}, sessionExpired()
	}

	response, rej := p.send(ctx, request, token)
	if rej != nil || response.status != http.StatusUnauthorized {
		return response, rej
	}

	renewed, err := p.refresh(ctx, token)
	if err != nil || renewed == "" || renewed == token {
		return upstreamResponse{}, sessionExpired()
	}

	response, rej = p.send(ctx, request, renewed)
	if rej == nil && response.status == http.StatusUnauthorized {
		return upstreamResponse{}, sessionExpired()
	}

	return response, rej
}

// refresh gets a new token after failed was rejected. Calls that were rejected
// with the same token queue up here, and only the first one asks the source:
// the others find the token already replaced. The source is asked at most
// maxRefreshes times in a run, so that a tenant that keeps answering 401 cannot
// make the proxy hammer it.
func (p *Proxy) refresh(ctx context.Context, failed string) (string, error) {
	p.refreshMu.Lock()
	defer p.refreshMu.Unlock()

	current, err := p.token(ctx, p.tokens.Token)
	if err != nil {
		return "", err
	}

	if current != failed {
		return current, nil
	}

	if p.refreshes >= maxRefreshes {
		return "", errRefreshLimit
	}

	p.refreshes++

	return p.token(ctx, p.tokens.Refresh)
}

// token asks the token source with get, giving it the upstream timeout. A source
// that hangs would otherwise leave every other call waiting for the refresh
// mutex, each holding a slot.
func (p *Proxy) token(ctx context.Context, get func(context.Context) (string, error)) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	return get(ctx)
}

// send makes one request to the tenant and reads the whole response. It does not
// follow redirects: the token only goes where the proxy chose to send it.
func (p *Proxy) send(ctx context.Context, request outgoing, token string) (upstreamResponse, *rejection) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	var body io.Reader
	if len(request.body) > 0 {
		body = bytes.NewReader(request.body)
	}

	upstream, err := http.NewRequestWithContext(ctx, request.method, "https://"+p.domain, body)
	if err != nil {
		return upstreamResponse{}, reject(http.StatusBadGateway, codeUpstreamFailed, "The request to Auth0 could not be made.")
	}

	// Forward the decoded path that the scope check saw. The URL type escapes it
	// the one way, so nothing is interpreted a second time.
	upstream.URL.Path = request.target.path
	upstream.URL.RawQuery = request.target.query
	upstream.Header = request.header.Clone()
	upstream.Header.Set("Authorization", "Bearer "+token)

	resp, err := p.transport.RoundTrip(upstream)
	if err != nil {
		return upstreamResponse{}, reject(http.StatusBadGateway, codeUpstreamFailed, "Auth0 could not be reached.")
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 && resp.StatusCode != http.StatusNotModified {
		return upstreamResponse{}, reject(http.StatusBadGateway, codeUpstreamRedirect, "Auth0 answered with a redirect, which the proxy does not follow.")
	}

	content, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody+1))
	if err != nil {
		return upstreamResponse{}, reject(http.StatusBadGateway, codeUpstreamFailed, "The response of Auth0 could not be read.")
	}

	if len(content) > maxResponseBody {
		return upstreamResponse{}, reject(http.StatusBadGateway, codeUnsupportedResponse, "The response of Auth0 is too large for the proxy to check.")
	}

	return upstreamResponse{status: resp.StatusCode, header: resp.Header, body: content}, nil
}

// inspect returns the body of the response without its secret fields. A body
// that is not JSON cannot be checked, so it is not passed on.
func (p *Proxy) inspect(response upstreamResponse) ([]byte, *rejection) {
	if len(response.body) == 0 {
		return nil, nil
	}

	if encoding := response.header.Get("Content-Encoding"); encoding != "" && !strings.EqualFold(encoding, "identity") {
		return nil, unsupportedResponse()
	}

	if !isJSON(response.header.Get("Content-Type")) {
		return nil, unsupportedResponse()
	}

	cleaned, err := stripSecrets(response.body, p.policy.isSecret)
	if err != nil {
		return nil, unsupportedResponse()
	}

	return cleaned, nil
}

// unsupportedResponse is what the plugin gets when a response cannot be checked
// for secrets.
func unsupportedResponse() *rejection {
	return reject(http.StatusBadGateway, codeUnsupportedResponse,
		"The response of Auth0 is not JSON that the proxy can check, so it was not passed on.")
}

// isJSON reports whether a Content-Type header names JSON.
func isJSON(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)

	return err == nil && (mediaType == "application/json" || strings.HasSuffix(mediaType, "+json"))
}

// writeResponse answers the plugin with the status and the headers of the
// response and the cleaned body.
func writeResponse(w http.ResponseWriter, response upstreamResponse, body []byte) {
	// The headers that the Connection header names are hop by hop too.
	var named []string

	for _, value := range response.header.Values("Connection") {
		for name := range strings.SplitSeq(value, ",") {
			named = append(named, http.CanonicalHeaderKey(strings.TrimSpace(name)))
		}
	}

	for name, values := range response.header {
		if !slices.Contains(droppedResponseHeaders, name) && !slices.Contains(named, name) {
			w.Header()[name] = values
		}
	}

	if response.status != http.StatusNoContent && response.status != http.StatusNotModified {
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	}

	w.WriteHeader(response.status)
	_, _ = w.Write(body)
}

// checkDomain checks that the tenant domain is a bare host name with a dot in
// it, and returns it in lowercase. A scheme, port, path, user info or an IP
// address are not host names, so none of them is accepted.
func checkDomain(domain string) (string, error) {
	lower := strings.ToLower(domain)
	labels := strings.Split(lower, ".")

	valid := len(lower) <= 253 && len(labels) >= 2
	for _, label := range labels {
		valid = valid && len(label) >= 1 && len(label) <= 63 &&
			!strings.HasPrefix(label, "-") && !strings.HasSuffix(label, "-") &&
			strings.Trim(label, "abcdefghijklmnopqrstuvwxyz0123456789-") == ""
	}

	// A top level domain is never all digits, which also rules out IP addresses.
	valid = valid && strings.Trim(labels[len(labels)-1], "0123456789") != ""

	if !valid {
		return "", fmt.Errorf("%w (got %q)", ErrInvalidDomain, domain)
	}

	return lower, nil
}

// newSecret returns the secret the plugin calls the proxy with.
func newSecret() (string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("failed to generate the proxy secret: %w", err)
	}

	return hex.EncodeToString(random), nil
}

// newTransport returns the transport that reaches the tenant: a copy of the
// default transport that speaks HTTP/1.1 only. Once the default transport has
// been used, its TLS config lists h2 in the protocols it offers, and the clone
// carries that over. The server would then pick h2, which the clone cannot
// speak, so the offer is narrowed to HTTP/1.1.
func newTransport() *http.Transport {
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment, TLSHandshakeTimeout: 10 * time.Second}
	if base, ok := http.DefaultTransport.(*http.Transport); ok {
		transport = base.Clone()
	}

	transport.ForceAttemptHTTP2 = false
	transport.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}

	if transport.TLSClientConfig != nil {
		transport.TLSClientConfig.NextProtos = []string{"http/1.1"}
	}

	return transport
}
