package auth0

import (
	"net/http"

	"github.com/auth0/go-auth0/management"
)

type HTTPClientAPI interface {
	// NewRequest returns a new HTTP request.
	// If the payload is not nil it will be encoded as JSON.
	NewRequest(method, uri string, payload interface{}, options ...management.RequestOption) (*http.Request, error)

	// Do triggers an HTTP request and returns an HTTP response,
	// handling any context cancellations or timeouts.
	Do(req *http.Request) (*http.Response, error)
}
