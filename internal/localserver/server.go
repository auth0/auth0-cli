// Package localserver implements a local OIDC identity server for agent-driven
// development. It exposes Auth0-compatible endpoints so existing SDKs can
// complete a full login round trip without a real Auth0 tenant.
//
// Every login is auto-approved as a single local test user. Tokens carry
// iss=http://localhost:<port>/ and are signed with an ephemeral key that has
// no relation to any real Auth0 tenant — they will fail production validation
// by design, making it safe for one to end up in a log or committed .env file.
package localserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
)

// Server is a local OIDC identity server.
type Server struct {
	port     int
	issuer   string
	httpSrv  *http.Server
	keys     *keySet
	store    *authStore
	mu       sync.Mutex
	lastCode *capturedCode
}

// capturedCode holds a simulated OTP/verification code for agent retrieval.
type capturedCode struct {
	Code      string `json:"code"`
	SentTo    string `json:"sent_to"`
	ExpiresIn int    `json:"expires_in"`
}

// New creates a Server bound to the given port.
func New(port int) (*Server, error) {
	ks, err := newKeySet()
	if err != nil {
		return nil, fmt.Errorf("generating local key pair: %w", err)
	}

	s := &Server{
		port:   port,
		issuer: fmt.Sprintf("http://localhost:%d", port),
		keys:   ks,
		store:  newAuthStore(),
	}
	s.httpSrv = &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", port),
		Handler: s.buildMux(),
	}
	return s, nil
}

// Start binds the TCP listener and begins serving in a background goroutine.
// It returns as soon as the port is bound; call Stop to shut down.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.httpSrv.Addr)
	if err != nil {
		return fmt.Errorf("binding to %s: %w", s.httpSrv.Addr, err)
	}
	go func() { _ = s.httpSrv.Serve(ln) }()
	return nil
}

// Stop gracefully shuts down the server within the given context deadline.
func (s *Server) Stop(ctx context.Context) error {
	return s.httpSrv.Shutdown(ctx)
}

// Addr returns the base URL of the server, e.g. "http://localhost:6789".
func (s *Server) Addr() string {
	return s.issuer
}
