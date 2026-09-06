package localserver

import "sync"

type authRequest struct {
	clientID    string
	redirectURI string
	state       string
	nonce       string
	scope       string
	challenge   string
	method      string
}

type authStore struct {
	mu   sync.Mutex
	reqs map[string]authRequest
}

func newAuthStore() *authStore {
	return &authStore{reqs: make(map[string]authRequest)}
}

func (s *authStore) save(code string, req authRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reqs[code] = req
}

// pop retrieves and removes the request associated with code (one-time use).
func (s *authStore) pop(code string) (authRequest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	req, ok := s.reqs[code]
	if ok {
		delete(s.reqs, code)
	}
	return req, ok
}
