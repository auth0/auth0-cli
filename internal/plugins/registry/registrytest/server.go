package registrytest

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/auth0/auth0-cli/internal/plugins/registry"
)

// route is what the server knows about one request path.
type route struct {
	body      []byte
	found     bool
	status    int
	truncated bool
	redirect  string
	requests  int
}

// Server serves a test registry over TLS. It serves /index.json, /index.json.sig
// and the per-plugin files, and lets a test swap what is served between
// requests, count the requests, and make requests fail.
type Server struct {
	server *httptest.Server

	mu     sync.Mutex
	routes map[string]*route
	hook   func(path string, count int)
}

// NewServer starts a server that serves nothing until a test publishes
// something. The server stops when the test ends.
func NewServer(t testing.TB) *Server {
	t.Helper()

	s := &Server{routes: make(map[string]*route)}

	s.server = httptest.NewTLSServer(http.HandlerFunc(s.serve))
	t.Cleanup(s.server.Close)

	return s
}

// update changes what the server knows about the request path.
func (s *Server) update(path string, change func(r *route)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	change(s.routeLocked(path))
}

// routeLocked returns the route of the request path, which it creates when
// there is none yet. The caller must hold the lock.
func (s *Server) routeLocked(path string) *route {
	r, ok := s.routes[path]
	if !ok {
		r = &route{}
		s.routes[path] = r
	}

	return r
}

// Publish signs the index of the builder with the key and serves it, together
// with its signature and per-plugin files.
func (s *Server) Publish(builder *IndexBuilder, keyID string, private ed25519.PrivateKey) {
	index := builder.Build()

	s.Set("/index.json", index)
	s.Set("/index.json.sig", Sign(index, keyID, private))

	for name, data := range builder.Files() {
		s.Set("/"+name, data)
	}
}

// Set serves body at the request path, for example "/index.json".
func (s *Server) Set(path string, body []byte) {
	s.update(path, func(r *route) { r.body, r.found = body, true })
}

// Fail makes the request path answer with the status code instead of its body,
// until it is called again with 0.
func (s *Server) Fail(path string, status int) {
	s.update(path, func(r *route) { r.status = status })
}

// Truncate makes the request path announce its whole body but cut the
// connection halfway through it.
func (s *Server) Truncate(path string) {
	s.update(path, func(r *route) { r.truncated = true })
}

// Redirect makes the request path redirect to location.
func (s *Server) Redirect(path, location string) {
	s.update(path, func(r *route) { r.redirect = location })
}

// OnRequest sets a function that runs for every request before it is answered,
// with the request path and how many times the path was requested, counting this
// one. It can call the other methods of the server to swap the content.
func (s *Server) OnRequest(hook func(path string, count int)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.hook = hook
}

// Requests returns how many times the request path was requested.
func (s *Server) Requests(path string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.routeLocked(path).requests
}

// URL returns the base URL of the server.
func (s *Server) URL() string {
	return s.server.URL
}

// Client returns an HTTP client that trusts the certificate of the server.
func (s *Server) Client() *http.Client {
	return s.server.Client()
}

// Registry returns the registry that the server serves.
func (s *Server) Registry() registry.Registry {
	return registry.Registry{Name: registry.DefaultRegistryName, URL: s.server.URL + "/index.json"}
}

// serve answers a request.
func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	s.mu.Lock()
	current := s.routeLocked(path)
	current.requests++
	count := current.requests
	hook := s.hook
	s.mu.Unlock()

	if hook != nil {
		hook(path, count)
	}

	// The hook may have changed the route, so it is read after it ran.
	s.mu.Lock()
	answer := *s.routeLocked(path)
	s.mu.Unlock()

	switch {
	case answer.redirect != "":
		http.Redirect(w, r, answer.redirect, http.StatusFound)
	case answer.status != 0:
		http.Error(w, http.StatusText(answer.status), answer.status)
	case !answer.found:
		http.NotFound(w, r)
	case answer.truncated:
		w.Header().Set("Content-Length", strconv.Itoa(len(answer.body)))
		_, _ = w.Write(answer.body[:len(answer.body)/2])
		panic(http.ErrAbortHandler)
	default:
		_, _ = w.Write(answer.body)
	}
}

// AnyHostClient returns an HTTP client that sends every request to this server,
// whatever host the URL names, and accepts the certificate of the server for any
// host name. A test can then use the real download hosts in artifact URLs and
// in redirects, so that the code under test applies its host rules unchanged.
func (s *Server) AnyHostClient() *http.Client {
	address := s.server.Listener.Addr().String()

	return &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, address)
		},
		// The server presents a throwaway certificate for example.com and 127.0.0.1.
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // Only for tests that talk to the test server.
	}}
}
