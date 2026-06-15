package auth0

import (
	"context"
	"net/http"

	"github.com/auth0/go-auth0/management"
)

type HTTPClientAPI interface {
	// NewRequest returns a new HTTP request.
	// If the payload is not nil it will be encoded as JSON.
	NewRequest(ctx context.Context, method, uri string, payload interface{}, options ...management.RequestOption) (*http.Request, error)

	// Do triggers an HTTP request and returns an HTTP response,
	// handling any context cancellations or timeouts.
	Do(req *http.Request) (*http.Response, error)

	// Request combines NewRequest and Do, encoding the payload as JSON and
	// returning an error for any non-2xx response.
	Request(ctx context.Context, method, uri string, payload interface{}, options ...management.RequestOption) error

	// URI builds a fully-qualified Management API URL from the given path segments.
	URI(path ...string) string
}
