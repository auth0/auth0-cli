package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/portals"
)

// dryRunClientSecret stands in for every client secret in the plan.
const dryRunClientSecret = "<client secret>"

// dryRunRedacted stands in for any other credential in the plan.
const dryRunRedacted = "<redacted>"

// credentialKeys are the fields of a request that hold a credential, apart from client_secret.
var credentialKeys = []string{"secret", "private_key", "password", "token", "access_token", "refresh_token"}

// plannedRequest is a request a real run would send.
type plannedRequest struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	// Body has every client secret replaced.
	Body any `json:"body,omitempty"`
}

// portalsDryRun is an HTTP transport that sends the reads to the tenant and keeps every other
// request instead of sending it. Setup runs as it does for real, so the created, reused and updated
// decisions and the payloads all come from the same code. The IDs of the resources a real run would
// create are placeholders, and a read about one of them finds nothing, because that resource does
// not exist yet.
type portalsDryRun struct {
	base http.RoundTripper

	mu       sync.Mutex
	requests []plannedRequest
	counter  int
	// The placeholders given to the resources that a real run would create.
	placeholders []string
	// The connections enabled for an organization that would be created, from the requests kept.
	enabledConnections map[string][]string
}

var _ http.RoundTripper = (*portalsDryRun)(nil)

func newPortalsDryRun(base http.RoundTripper) *portalsDryRun {
	return &portalsDryRun{base: base, enabledConnections: map[string][]string{}}
}

// planned returns the requests that were kept, in order.
func (d *portalsDryRun) planned() []plannedRequest {
	d.mu.Lock()
	defer d.mu.Unlock()

	return slices.Clone(d.requests)
}

// RoundTrip implements http.RoundTripper.
func (d *portalsDryRun) RoundTrip(request *http.Request) (*http.Response, error) {
	body, err := readAndRestoreBody(request)
	if err != nil {
		return nil, fmt.Errorf("read the request to %s: %w", request.URL.Path, err)
	}

	if request.Method == http.MethodGet || request.Method == http.MethodHead {
		return d.read(request)
	}

	return d.keep(request, body), nil
}

func readAndRestoreBody(request *http.Request) ([]byte, error) {
	if request.Body == nil {
		return nil, nil
	}

	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}

	_ = request.Body.Close()
	request.Body = io.NopCloser(bytes.NewReader(body))

	return body, nil
}

// apiPath is the path of a request below the Management API.
func apiPath(request *http.Request) string {
	path := request.URL.Path

	if index := strings.Index(path, "/api/v2/"); index >= 0 {
		return path[index+len("/api/v2/"):]
	}

	return strings.TrimPrefix(path, "/")
}

// read sends a read to the tenant, unless it is about a resource that does not exist yet.
func (d *portalsDryRun) read(request *http.Request) (*http.Response, error) {
	path := apiPath(request)

	d.mu.Lock()
	about := d.placeholderIn(path + "?" + request.URL.RawQuery)
	connections := d.enabledConnections[about]
	d.mu.Unlock()

	if about == "" {
		return d.base.RoundTrip(request)
	}

	// The connections of an organization that would be created are the ones the plan enables on it.
	if strings.HasSuffix(path, "/enabled_connections") && len(connections) > 0 {
		entries := make([]map[string]string, 0, len(connections))
		for _, id := range connections {
			entries = append(entries, map[string]string{"connection_id": id})
		}

		return jsonResponse(request, map[string]any{"enabled_connections": entries, "total": len(entries), "limit": 100})
	}

	// An empty object reads as an empty list for every kind of listing.
	return jsonResponse(request, map[string]any{})
}

// placeholderIn returns the placeholder the text is about, or "".
func (d *portalsDryRun) placeholderIn(text string) string {
	decoded, err := url.QueryUnescape(text)
	if err != nil {
		decoded = text
	}

	for _, placeholder := range d.placeholders {
		if strings.Contains(decoded, placeholder) {
			return placeholder
		}
	}

	return ""
}

// keep records a request that would change the tenant and answers it the way the API would.
func (d *portalsDryRun) keep(request *http.Request, body []byte) *http.Response {
	var decoded any
	if len(body) > 0 {
		if err := json.Unmarshal(body, &decoded); err != nil {
			decoded = string(body)
		}
	}

	path := apiPath(request)

	d.mu.Lock()
	defer d.mu.Unlock()

	d.requests = append(d.requests, plannedRequest{Method: request.Method, Path: path, Body: redactSecrets(decoded)})

	if request.Method != http.MethodPost {
		response, _ := jsonResponse(request, map[string]any{})

		return response
	}

	d.counter++
	id := fmt.Sprintf("<new:%s:%d>", strings.SplitN(path, "/", 2)[0], d.counter)
	d.placeholders = append(d.placeholders, id)

	if segments := strings.Split(path, "/"); len(segments) == 3 && segments[0] == "organizations" && segments[2] == "enabled_connections" {
		if fields, isObject := decoded.(map[string]any); isObject {
			connection, _ := fields["connection_id"].(string)
			d.enabledConnections[segments[1]] = append(d.enabledConnections[segments[1]], connection)
		}
	}

	// The answer is built from the redacted body, so a secret that was sent is never in what comes back.
	response, _ := jsonResponse(request, createdResponse(path, redactSecrets(decoded), id))

	return response
}

// createdResponse is the part of the answer to a create that setup reads back: the ID, and for an
// application the secret.
func createdResponse(path string, body any, id string) map[string]any {
	fields, _ := body.(map[string]any)
	created := make(map[string]any, len(fields)+2)

	for key, value := range fields {
		created[key] = value
	}

	switch path {
	case "clients":
		created["client_id"] = id
		created["client_secret"] = dryRunClientSecret
	case "forms/import":
		return map[string]any{"form": map[string]any{"id": id, "nodes": []any{}}, "flows": []any{}}
	default:
		created["id"] = id
	}

	return created
}

// redactSecrets copies the value and replaces every client secret and credential, so a secret read from
// the tenant for a reused application can never reach the plan.
func redactSecrets(value any) any {
	switch typed := value.(type) {
	case []any:
		redacted := make([]any, 0, len(typed))
		for _, item := range typed {
			redacted = append(redacted, redactSecrets(item))
		}

		return redacted
	case map[string]any:
		redacted := make(map[string]any, len(typed))

		for key, item := range typed {
			if key == "client_secret" {
				redacted[key] = dryRunClientSecret
				continue
			}

			// The refresh_token of an application is a settings object, which holds no secret.
			if _, isText := item.(string); isText && slices.Contains(credentialKeys, key) {
				redacted[key] = dryRunRedacted
				continue
			}

			redacted[key] = redactSecrets(item)
		}

		return redacted
	default:
		return value
	}
}

func jsonResponse(request *http.Request, body any) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode the answer to %s: %w", request.URL.Path, err)
	}

	return &http.Response{
		Status:        "200 OK",
		StatusCode:    http.StatusOK,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        http.Header{"Content-Type": []string{"application/json"}},
		Body:          io.NopCloser(bytes.NewReader(data)),
		ContentLength: int64(len(data)),
		Request:       request,
	}, nil
}

// dryRunTenant builds the tenant of a dry run: its clients read from the tenant and keep the writes.
// It is built from what the clients of the CLI were built with.
func dryRunTenant(access apiAccess) (*portalsTenant, *portalsDryRun, error) {
	return dryRunTenantOver(customClientWithRetries(access.invokerMetadata).Transport, access)
}

// dryRunTenantOver is dryRunTenant with the transport that sends the reads.
func dryRunTenantOver(base http.RoundTripper, access apiAccess) (*portalsTenant, *portalsDryRun, error) {
	plan := newPortalsDryRun(base)
	httpClient := &http.Client{Transport: plan}

	v1, err := initializeManagementClientWith(access.domain, access.accessToken, httpClient)
	if err != nil {
		return nil, nil, fmt.Errorf("could not prepare the dry run: %w", err)
	}

	v3, err := initializeManagementClientV3With(access.domain, access.accessToken, httpClient)
	if err != nil {
		return nil, nil, fmt.Errorf("could not prepare the dry run: %w", err)
	}

	return newPortalsTenant(auth0.NewAPI(v1), auth0.NewAPIV3(v3)), plan, nil
}

// dryRunResult is the result of setup for a dry run: the result of a real run, and the requests it
// would send in the order it would send them.
type dryRunResult struct {
	DryRun bool `json:"dryRun"`
	portals.SetupResult
	Requests []plannedRequest `json:"requests"`
}
