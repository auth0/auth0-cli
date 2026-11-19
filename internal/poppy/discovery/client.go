// Package discovery fetches the public documents a Poppy site publishes, with
// an HTTP client that never sends credentials.
package discovery

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	requestTimeout = 10 * time.Second
	maxRedirects   = 5
	maxBodyBytes   = 1 << 20
	userAgent      = "auth0-cli-poppy-doctor"
)

var (
	// ErrTooManyRedirects is returned when a request is redirected more than five times.
	ErrTooManyRedirects = errors.New("stopped after 5 redirects")

	// ErrBodyTooLarge is returned when a response body is larger than 1 MiB.
	ErrBodyTooLarge = errors.New("response body is larger than 1 MiB")
)

// InsecureURLError reports a request URL or a redirect target that is not HTTPS.
type InsecureURLError struct {
	URL string
}

func (e *InsecureURLError) Error() string {
	return fmt.Sprintf("%s is not an HTTPS URL", e.URL)
}

// Response is a fully read HTTP response.
type Response struct {
	// URL is the URL the response came from, after any redirects.
	URL        string
	StatusCode int
	Header     http.Header
	Body       []byte
}

// Client sends credential-free GET requests.
type Client struct {
	http *http.Client
}

// New returns a client that sends requests through hc. A nil hc is a plain
// client. The client always drops the cookie jar, caps the time of a request
// and allows HTTPS redirects only, whatever hc says.
func New(hc *http.Client) *Client {
	var c http.Client
	if hc != nil {
		c = *hc
	}
	c.Jar = nil
	c.Timeout = requestTimeout
	c.CheckRedirect = checkRedirect

	return &Client{http: &c}
}

// Get requests an HTTPS URL. Any HTTP status is returned as a response; only
// transport failures, insecure or excessive redirects and oversized bodies are
// errors.
func (c *Client) Get(ctx context.Context, rawURL string) (*Response, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	if u.Scheme != "https" || u.Host == "" {
		return nil, &InsecureURLError{URL: rawURL}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build the request for %s: %w", rawURL, err)
	}
	req.Header.Set("User-Agent", userAgent)

	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, maxBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read the response from %s: %w", rawURL, err)
	}
	if len(body) > maxBodyBytes {
		return nil, ErrBodyTooLarge
	}

	return &Response{URL: res.Request.URL.String(), StatusCode: res.StatusCode, Header: res.Header, Body: body}, nil
}

func checkRedirect(req *http.Request, via []*http.Request) error {
	if req.URL.Scheme != "https" {
		return &InsecureURLError{URL: req.URL.String()}
	}
	if len(via) > maxRedirects {
		return ErrTooManyRedirects
	}

	return nil
}
