package cli

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/rehttp"
	"github.com/auth0/go-auth0/management"
	managementv3 "github.com/auth0/go-auth0/v3/management/client"
	"github.com/auth0/go-auth0/v3/management/option"

	"github.com/auth0/auth0-cli/internal/buildinfo"
)

func initializeManagementClient(tenantDomain string, accessToken string, httpClient *http.Client) (*management.Management, error) {
	client, err := management.New(
		tenantDomain,
		management.WithStaticToken(accessToken),
		management.WithUserAgent(fmt.Sprintf("%v/%v", userAgent, strings.TrimPrefix(buildinfo.Version, "v"))),
		management.WithAuth0ClientEnvEntry("Auth0-CLI", strings.TrimPrefix(buildinfo.Version, "v")),
		management.WithNoRetries(),
		management.WithClient(httpClient),
	)

	return client, err
}

func initializeManagementClientV3(tenantDomain string, accessToken string, httpClient *http.Client) (*managementv3.Management, error) {
	client, err := managementv3.New(
		tenantDomain,
		option.WithToken(accessToken),
		option.WithUserAgent(fmt.Sprintf("%v/%v", userAgent, strings.TrimPrefix(buildinfo.Version, "v"))),
		option.WithAuth0ClientEnvEntry("Auth0-CLI", strings.TrimPrefix(buildinfo.Version, "v")),
		// If not set, it defaults to 2 as per v2@v2.5.0/management/internal/retrier.go:43.
		// Setting it to 1 to avoid retries from `go-auth0` since we have our own retry logic in the custom HTTP client.
		// TODO: confirm this assumption, or check if this needs to be excluded like terraform provider.
		option.WithMaxAttempts(1),
		option.WithHTTPClient(httpClient),
	)
	return client, err
}

// switchableTransport sends the requests of the Management API clients. A command can put a transport
// in front of it for as long as it runs, for example to keep the writes of a dry run.
type switchableTransport struct {
	base http.RoundTripper

	mu    sync.RWMutex
	front http.RoundTripper
}

// RoundTrip implements http.RoundTripper.
func (t *switchableTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	t.mu.RLock()
	next := t.front
	t.mu.RUnlock()

	if next == nil {
		next = t.base
	}

	return next.RoundTrip(request)
}

// use sends the requests through front until restore is called.
func (t *switchableTransport) use(front http.RoundTripper) (restore func()) {
	t.mu.Lock()
	t.front = front
	t.mu.Unlock()

	return func() {
		t.mu.Lock()
		t.front = nil
		t.mu.Unlock()
	}
}

func customClientWithRetries(invokerMetadata string) *http.Client {
	client := &http.Client{
		// The metadata transport wraps the retry transports so the header is stamped
		// once, before any retry decision, and is preserved across retries.
		Transport: invokerMetadataTransport{
			metadata: invokerMetadata,
			base: rateLimitTransport(
				retryableErrorTransport(
					http.DefaultTransport,
				),
			),
		},
	}

	return client
}

func rateLimitTransport(tripper http.RoundTripper) http.RoundTripper {
	return rehttp.NewTransport(tripper, rateLimitRetry, rateLimitDelay)
}

func rateLimitRetry(attempt rehttp.Attempt) bool {
	if attempt.Response == nil {
		return false
	}

	return attempt.Response.StatusCode == http.StatusTooManyRequests
}

func rateLimitDelay(attempt rehttp.Attempt) time.Duration {
	resetAt := attempt.Response.Header.Get("X-RateLimit-Reset")

	resetAtUnix, err := strconv.ParseInt(resetAt, 10, 64)
	if err != nil {
		resetAtUnix = time.Now().Add(5 * time.Second).Unix()
	}

	return time.Duration(resetAtUnix-time.Now().Unix()) * time.Second
}

func retryableErrorTransport(tripper http.RoundTripper) http.RoundTripper {
	retryableCodes := []int{
		http.StatusServiceUnavailable,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusGatewayTimeout,
		// Cloudflare-specific server error that is generated
		// because Cloudflare did not receive an HTTP response
		// from the origin server after an HTTP Connection was made.
		524,
	}

	return rehttp.NewTransport(
		tripper,
		rehttp.RetryAll(
			rehttp.RetryMaxRetries(3),
			rehttp.RetryAny(
				rehttp.RetryStatuses(retryableCodes...),
				rehttp.RetryIsErr(retryableErrorRetryFunc),
			),
		),
		rehttp.ExpJitterDelay(500*time.Millisecond, 10*time.Second),
	)
}

func retryableErrorRetryFunc(err error) bool {
	if err == nil {
		return false
	}

	if v, ok := err.(*url.Error); ok {
		// Don't retry if the error was due to too many redirects.
		if regexp.MustCompile(`stopped after \d+ redirects\z`).MatchString(v.Error()) {
			return false
		}

		// Don't retry if the error was due to an invalid protocol scheme.
		if regexp.MustCompile(`unsupported protocol scheme`).MatchString(v.Error()) {
			return false
		}

		// Don't retry if the certificate issuer is unknown.
		if _, ok := v.Err.(*tls.CertificateVerificationError); ok {
			return false
		}

		// Don't retry if the certificate issuer is unknown.
		if _, ok := v.Err.(x509.UnknownAuthorityError); ok {
			return false
		}
	}

	// The error is likely recoverable so retry.
	return true
}
