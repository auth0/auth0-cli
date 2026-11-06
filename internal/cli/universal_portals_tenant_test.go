package cli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/auth0/go-auth0/management"
	managementv3 "github.com/auth0/go-auth0/v3/management/client"
	"github.com/auth0/go-auth0/v3/management/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/portals"
)

const testAPIPrefix = "/api/v2/"

// receivedRequest is a request the test server got.
type receivedRequest struct {
	Method string
	// Path is the escaped path below the API prefix.
	Path  string
	Query map[string][]string
	Body  string
}

func (r receivedRequest) json(t *testing.T) map[string]any {
	t.Helper()

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(r.Body), &body), "body: %q", r.Body)

	return body
}

// portalsServer is a Management API stand-in that answers the routes a test registers and records
// every request. A request to a route nobody registered fails the test.
type portalsServer struct {
	t        *testing.T
	mu       sync.Mutex
	routes   map[string]func(receivedRequest) (int, string)
	requests []receivedRequest
}

// newPortalsServer starts the server and returns a tenant that talks to it through the real v1 and
// v3 clients, so the tests see the requests the SDK really sends.
func newPortalsServer(t *testing.T) (*portalsServer, *portalsTenant) {
	t.Helper()

	server := &portalsServer{t: t, routes: map[string]func(receivedRequest) (int, string){}}

	httpServer := httptest.NewServer(http.HandlerFunc(server.serve))
	t.Cleanup(httpServer.Close)

	v1, err := management.New(
		strings.TrimPrefix(httpServer.URL, "http://"),
		management.WithStaticToken("token"),
		management.WithInsecure(),
		management.WithNoRetries(),
	)
	require.NoError(t, err)

	v3 := managementv3.NewWithOptions(
		option.WithBaseURL(httpServer.URL+"/api/v2"),
		option.WithToken("token"),
		option.WithMaxAttempts(1),
	)

	return server, newPortalsTenant(auth0.NewAPI(v1), auth0.NewAPIV3(v3))
}

func (s *portalsServer) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	request := receivedRequest{
		Method: r.Method,
		Path:   strings.TrimPrefix(r.URL.EscapedPath(), testAPIPrefix),
		Query:  r.URL.Query(),
		Body:   string(body),
	}

	s.mu.Lock()
	s.requests = append(s.requests, request)
	route, found := s.routes[request.Method+" "+request.Path]
	s.mu.Unlock()

	if !found {
		s.t.Errorf("unexpected request %s %s", request.Method, request.Path)
		http.Error(w, `{"statusCode":404,"message":"no route"}`, http.StatusNotFound)

		return
	}

	status, response := route(request)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, response)
}

// reply registers a fixed answer for "METHOD path".
func (s *portalsServer) reply(method, path string, status int, body string) {
	s.handle(method, path, func(receivedRequest) (int, string) { return status, body })
}

// handle registers a function that answers "METHOD path".
func (s *portalsServer) handle(method, path string, route func(receivedRequest) (int, string)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.routes[method+" "+path] = route
}

// received returns the requests to "METHOD path", in order.
func (s *portalsServer) received(method, path string) []receivedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	var matching []receivedRequest
	for _, request := range s.requests {
		if request.Method == method && request.Path == path {
			matching = append(matching, request)
		}
	}

	return matching
}

func TestAsPortalsError(t *testing.T) {
	t.Parallel()

	const scopeError = `{"statusCode":403,"error":"Forbidden","message":"Insufficient scope, expected any of: read:resource_servers","errorCode":"insufficient_scope"}`

	t.Run("a failed v1 request keeps the status, the code and the message", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodGet, "resource-servers", http.StatusForbidden, scopeError)

		_, err := tenant.ListResourceServers(context.Background())

		var apiErr *portals.APIError
		require.ErrorAs(t, err, &apiErr)
		assert.Equal(t, 403, apiErr.Status)
		assert.Equal(t, "insufficient_scope", apiErr.Code)
		assert.Equal(t, "Insufficient scope, expected any of: read:resource_servers", apiErr.Message)
	})

	t.Run("a failed v3 request keeps the status, the code and the message", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodGet, "client-grants", http.StatusForbidden, scopeError)

		_, err := tenant.ListClientGrants(context.Background(), "client")

		var apiErr *portals.APIError
		require.ErrorAs(t, err, &apiErr)
		assert.Equal(t, 403, apiErr.Status)
		assert.Equal(t, "insufficient_scope", apiErr.Code)
		assert.Equal(t, "Insufficient scope, expected any of: read:resource_servers", apiErr.Message)
	})

	t.Run("a failed raw request keeps the status, the code and the message", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodGet, "portals/up_1", http.StatusNotFound, `{"statusCode":404,"message":"not found","errorCode":"inexistent_portal"}`)

		_, err := tenant.ReadPortal(context.Background(), "up_1")

		var apiErr *portals.APIError
		require.ErrorAs(t, err, &apiErr)
		assert.Equal(t, 404, apiErr.Status)
		assert.Equal(t, "inexistent_portal", apiErr.Code)
		assert.Equal(t, "not found", apiErr.Message)
	})

	t.Run("a body that is not JSON becomes the message", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodGet, "portals/up_1", http.StatusBadGateway, "upstream unavailable")

		_, err := tenant.ReadPortal(context.Background(), "up_1")

		var apiErr *portals.APIError
		require.ErrorAs(t, err, &apiErr)
		assert.Equal(t, 502, apiErr.Status)
		assert.Equal(t, "upstream unavailable", apiErr.Message)
	})

	t.Run("an error that is not from the API is returned as it is", func(t *testing.T) {
		t.Parallel()

		failure := errors.New("network down")

		assert.Same(t, failure, asPortalsError(failure))
		assert.NoError(t, asPortalsError(nil))
	})
}

func TestListPages(t *testing.T) {
	t.Parallel()

	t.Run("it reads every page", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.handle(http.MethodGet, "resource-servers", func(r receivedRequest) (int, string) {
			if r.Query["page"][0] == "0" {
				return 200, `{"start":0,"limit":100,"total":150,"resource_servers":` + serversJSON(0, 100) + `}`
			}

			return 200, `{"start":100,"limit":100,"total":150,"resource_servers":` + serversJSON(100, 150) + `}`
		})

		servers, err := tenant.ListResourceServers(context.Background())

		require.NoError(t, err)
		assert.Len(t, servers, 150)
		assert.Equal(t, "https://api/0", servers[0].Identifier)
		assert.Equal(t, "https://api/149", servers[149].Identifier)

		requests := server.received(http.MethodGet, "resource-servers")
		require.Len(t, requests, 2)
		assert.Equal(t, []string{"100"}, requests[0].Query["per_page"])
		assert.Equal(t, []string{"1"}, requests[1].Query["page"])
	})

	t.Run("it stops a listing that never ends", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodGet, "resource-servers", 200, `{"start":0,"limit":100,"total":1000000,"resource_servers":[]}`)

		_, err := tenant.ListResourceServers(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "did not stop paging")
		assert.Len(t, server.received(http.MethodGet, "resource-servers"), portalsMaxPages)
	})

	t.Run("it stops when the context ends", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodGet, "resource-servers", 200, `{"start":0,"limit":100,"total":1000000,"resource_servers":[]}`)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := tenant.ListResourceServers(ctx)

		assert.ErrorIs(t, err, context.Canceled)
		assert.Empty(t, server.received(http.MethodGet, "resource-servers"))
	})
}

// serversJSON returns resource servers numbered from first up to, and not including, last.
func serversJSON(first, last int) string {
	items := make([]string, 0, last-first)
	for number := first; number < last; number++ {
		id := itoa(number)
		items = append(items, `{"id":"rs_`+id+`","name":"API `+id+`","identifier":"https://api/`+id+`"}`)
	}

	return "[" + strings.Join(items, ",") + "]"
}

func itoa(number int) string {
	data, _ := json.Marshal(number)

	return string(data)
}

func TestPortalsTenant_ResourceServers(t *testing.T) {
	t.Parallel()

	server, tenant := newPortalsServer(t)
	server.handle(http.MethodPost, "resource-servers", func(receivedRequest) (int, string) {
		return 201, `{"id":"rs_1","name":"Auth0 My Account API","identifier":"https://acme.us.auth0.com/me/"}`
	})

	created, err := tenant.CreateResourceServer(context.Background(), portals.ResourceServerSpec{
		Name:       "Auth0 My Account API",
		Identifier: "https://acme.us.auth0.com/me/",
		SkipConsentForVerifiableFirstPartyClients: true,
		TokenDialect: "rfc9068_profile",
	})

	require.NoError(t, err)
	assert.Equal(t, portals.ResourceServer{ID: "rs_1", Identifier: "https://acme.us.auth0.com/me/", Name: "Auth0 My Account API"}, created)
	assert.Equal(t, map[string]any{
		"name":       "Auth0 My Account API",
		"identifier": "https://acme.us.auth0.com/me/",
		"skip_consent_for_verifiable_first_party_clients": true,
		"token_dialect": "rfc9068_profile",
	}, server.received(http.MethodPost, "resource-servers")[0].json(t))
}
