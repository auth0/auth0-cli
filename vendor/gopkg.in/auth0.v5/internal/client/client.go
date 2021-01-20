package client

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"time"

	"github.com/PuerkitoBio/rehttp"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
	"gopkg.in/auth0.v5"
)

// UserAgent is the default user agent string
var UserAgent = fmt.Sprintf("Go-Auth0-SDK/%s", auth0.Version)

// RoundTripFunc is an adapter to allow the use of ordinary functions as HTTP
// round trips.
type RoundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip executes a single HTTP transaction, returning
// a Response for the provided Request.
func (rf RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return rf(req)
}

// RateLimitTransport wraps base transport with rate limiting functionality.
//
// When a 429 status code is returned by the remote server, the
// "X-RateLimit-Reset" header is used to determine how long the transport will
// wait until re-issuing the failed request.
func RateLimitTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return rehttp.NewTransport(base, retry, delay)
}

func retry(attempt rehttp.Attempt) bool {
	if attempt.Response == nil {
		return false
	}
	return attempt.Response.StatusCode == http.StatusTooManyRequests
}

func delay(attempt rehttp.Attempt) time.Duration {
	resetAt := attempt.Response.Header.Get("X-RateLimit-Reset")
	resetAtUnix, err := strconv.ParseInt(resetAt, 10, 64)
	if err != nil {
		resetAtUnix = time.Now().Add(5 * time.Second).Unix()
	}
	return time.Duration(resetAtUnix-time.Now().Unix()) * time.Second
}

// RateLimitTransport wraps base transport with a customized "User-Agent" header
func UserAgentTransport(base http.RoundTripper, userAgent string) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		req.Header.Set("User-Agent", userAgent)
		return base.RoundTrip(req)
	})
}

func dumpRequest(r *http.Request) {
	b, _ := httputil.DumpRequestOut(r, true)
	log.Printf("\n%s\n", b)
}

func dumpResponse(r *http.Response) {
	b, _ := httputil.DumpResponse(r, true)
	log.Printf("\n%s\n\n", b)
}

// RateLimitTransport wraps base transport with the ability to log the contents
// of requests and responses.
func DebugTransport(base http.RoundTripper, debug bool) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	if !debug {
		return base
	}
	return RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		dumpRequest(req)
		res, err := base.RoundTrip(req)
		if err != nil {
			return res, err
		}
		dumpResponse(res)
		return res, nil
	})
}

// Option is the type used to configure a client.
type Option func(*http.Client)

// WithDebug configures the client to enable debug.
func WithDebug(debug bool) Option {
	return func(c *http.Client) {
		c.Transport = DebugTransport(c.Transport, debug)
	}
}

// WithRateLimit configures the client to enable rate limiting.
func WithRateLimit() Option {
	return func(c *http.Client) {
		c.Transport = RateLimitTransport(c.Transport)
	}
}

// WithUserAgent configures the client to overwrite the user agent header.
func WithUserAgent(userAgent string) Option {
	return func(c *http.Client) {
		c.Transport = UserAgentTransport(c.Transport, userAgent)
	}
}

// Wrap the base client with transports that enable OAuth2 authentication.
func Wrap(base *http.Client, tokenSource oauth2.TokenSource, options ...Option) *http.Client {
	if base == nil {
		base = http.DefaultClient
	}
	client := &http.Client{
		Timeout: base.Timeout,
		Transport: &oauth2.Transport{
			Base:   base.Transport,
			Source: tokenSource,
		},
	}
	for _, option := range options {
		option(client)
	}
	return client
}

func ClientCredentials(ctx context.Context, uri, clientID, clientSecret string) oauth2.TokenSource {
	return (&clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     uri + "/oauth/token",
		EndpointParams: url.Values{
			"audience": {uri + "/api/v2/"},
		},
	}).TokenSource(ctx)
}

func StaticToken(token string) oauth2.TokenSource {
	return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
}
