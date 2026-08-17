package cli

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/auth0/go-auth0/v3/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveInvokerMetadata(t *testing.T) {
	testCases := []struct {
		name         string
		agentClient  string
		isCI         bool
		expectedKind string
		expectedName string
	}{
		{
			name:         "named agent",
			agentClient:  "claude-code",
			expectedKind: "agent",
			expectedName: "claude-code",
		},
		{
			// The CI field is what keeps the environment from being lost here.
			name:         "named agent inside CI still reports as agent",
			agentClient:  "cursor",
			isCI:         true,
			expectedKind: "agent",
			expectedName: "cursor",
		},
		{
			name:         "unidentified agent",
			agentClient:  "unknown-agent",
			expectedKind: "agent",
			expectedName: "unknown",
		},
		{
			name:         "unidentified agent inside CI",
			agentClient:  "unknown-agent",
			isCI:         true,
			expectedKind: "agent",
			expectedName: "unknown",
		},
		{
			name:         "interactive human",
			agentClient:  "human",
			expectedKind: "human",
			expectedName: "unknown",
		},
		{
			// No TTY does not mean no person: piping output or running from a script
			// still reports as human.
			name:         "non-interactive human outside CI",
			agentClient:  "unknown",
			expectedKind: "human",
			expectedName: "unknown",
		},
		{
			name:         "human on a TTY inside CI",
			agentClient:  "human",
			isCI:         true,
			expectedKind: "ci",
			expectedName: "unknown",
		},
		{
			name:         "no signal inside CI",
			agentClient:  "unknown",
			isCI:         true,
			expectedKind: "ci",
			expectedName: "unknown",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			metadata := resolveInvokerMetadata(testCase.agentClient, testCase.isCI)

			assert.Equal(t, testCase.expectedKind, metadata.InvokerKind)
			assert.Equal(t, testCase.expectedName, metadata.InvokerAgent)
			assert.Equal(t, testCase.isCI, metadata.CI, "the CI signal must be reported independently of the kind")
		})
	}
}

// TestInvokerKindIsAlwaysKnown pins invoker_kind to a closed set, so consumers can
// treat it as an enum. No detectAgent output resolves to "unknown": a missing agent
// signal means a person, whether or not a terminal is attached.
func TestInvokerKindIsAlwaysKnown(t *testing.T) {
	// Every shape detectAgent can return: named agents, an unnamed agent, a sanitized
	// AUTH0_CLI_CLIENT value, and both Tier 4 fallbacks.
	agentClients := append(
		[]string{agentClientUnknownAgent, "client-something", invokerKindHuman, invokerUnknown},
		knownAgentClients...,
	)

	for _, agentClient := range agentClients {
		for _, isCI := range []bool{false, true} {
			metadata := resolveInvokerMetadata(agentClient, isCI)

			assert.Contains(
				t,
				[]string{invokerKindAgent, invokerKindCI, invokerKindHuman},
				metadata.InvokerKind,
				"agentClient %q with CI %v produced an out-of-set kind", agentClient, isCI,
			)
		}
	}
}

// TestInvokerMetadataAlwaysReportsCI pins the absence of omitempty on the CI field:
// "not CI" must be an explicit false, not a missing key.
func TestInvokerMetadataAlwaysReportsCI(t *testing.T) {
	value := resolveInvokerMetadata("claude-code", false).headerValue()

	decoded, err := base64.StdEncoding.DecodeString(value)
	require.NoError(t, err)
	assert.JSONEq(
		t,
		`{"invoker_kind":"agent","invoker_agent":"claude-code","ci":false}`,
		string(decoded),
	)
}

func TestInvokerMetadataHeaderValue(t *testing.T) {
	value := invokerMetadata{InvokerKind: "agent", InvokerAgent: "claude-code"}.headerValue()

	// The value must be base64-encoded JSON, matching the sibling Auth0-Client header.
	decoded, err := base64.StdEncoding.DecodeString(value)
	require.NoError(t, err)
	assert.JSONEq(t, `{"invoker_kind":"agent","invoker_agent":"claude-code","ci":false}`, string(decoded))
}

// TestInvokerMetadataHeaderIsCanonical guards the constant against Go's header
// canonicalization, so the name in code always matches the name on the wire.
func TestInvokerMetadataHeaderIsCanonical(t *testing.T) {
	assert.Equal(t, invokerMetadataHeader, http.CanonicalHeaderKey(invokerMetadataHeader))
}

func TestInvokerMetadataTransportSetsHeader(t *testing.T) {
	var receivedHeader string

	testServer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		receivedHeader = request.Header.Get(invokerMetadataHeader)
	}))
	t.Cleanup(testServer.Close)

	metadata := invokerMetadata{InvokerKind: "agent", InvokerAgent: "claude-code"}.headerValue()
	client := customClientWithRetries(metadata)

	request, err := http.NewRequest(http.MethodGet, testServer.URL, nil)
	require.NoError(t, err)

	response, err := client.Do(request)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, response.Body.Close())
	})

	assert.Equal(t, metadata, receivedHeader)
	assert.Empty(t, request.Header.Get(invokerMetadataHeader), "the original request should not be mutated")
}

func TestInvokerMetadataTransportOmitsEmptyHeader(t *testing.T) {
	headerPresent := true

	testServer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		_, headerPresent = request.Header[invokerMetadataHeader]
	}))
	t.Cleanup(testServer.Close)

	request, err := http.NewRequest(http.MethodGet, testServer.URL, nil)
	require.NoError(t, err)

	response, err := customClientWithRetries("").Do(request)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, response.Body.Close())
	})

	assert.False(t, headerPresent, "an empty payload should omit the header entirely")
}

// trustTestServer points the shared http.DefaultTransport, which
// customClientWithRetries builds on, at the test server's certificate so a real TLS
// handshake succeeds without weakening verification. Restored on cleanup.
func trustTestServer(t *testing.T, server *httptest.Server) {
	t.Helper()

	transport, ok := http.DefaultTransport.(*http.Transport)
	require.True(t, ok, "http.DefaultTransport is expected to be *http.Transport")

	original := transport.TLSClientConfig
	t.Cleanup(func() { transport.TLSClientConfig = original })

	pool := x509.NewCertPool()
	pool.AddCert(server.Certificate())
	transport.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
}

// capturedRequest records what a request actually looked like on the server side.
type capturedRequest struct {
	headerKeys []string
	metadata   string
	auth0Cli   string
	userAgent  string
	proto      string
}

// newMetadataEchoServer serves an empty JSON list over TLS and captures the headers
// of the first request it receives.
func newMetadataEchoServer(t *testing.T, captured *capturedRequest) *httptest.Server {
	t.Helper()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if captured.proto == "" {
			for key := range r.Header {
				captured.headerKeys = append(captured.headerKeys, key)
			}
			captured.proto = r.Proto
			captured.metadata = r.Header.Get(invokerMetadataHeader)
			captured.auth0Cli = r.Header.Get("Auth0-Client")
			captured.userAgent = r.Header.Get("User-Agent")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)

	return server
}

// domainOf strips the scheme so the value matches what the CLI stores as a tenant domain.
func domainOf(server *httptest.Server) string {
	return server.URL[len("https://"):]
}

// assertMetadataHeader decodes the captured header and checks the payload.
func assertMetadataHeader(t *testing.T, captured capturedRequest, expected invokerMetadata) {
	t.Helper()

	t.Logf("server received %s: %s", invokerMetadataHeader, captured.metadata)

	require.NotEmpty(t, captured.metadata, "the %s header never reached the server", invokerMetadataHeader)

	raw, err := base64.StdEncoding.DecodeString(captured.metadata)
	require.NoError(t, err, "header value must be valid base64")
	t.Logf("decoded payload: %s", raw)

	var decoded invokerMetadata
	require.NoError(t, json.Unmarshal(raw, &decoded), "decoded header must be valid JSON")
	assert.Equal(t, expected, decoded)

	// The header name must survive Go's canonicalization exactly as spelled.
	assert.Contains(t, captured.headerKeys, invokerMetadataHeader)

	// Sanity check that this is a genuine SDK request and that our header rides
	// alongside the existing telemetry rather than displacing it.
	assert.Contains(t, captured.userAgent, userAgent)
}

// TestManagementClientV1SendsInvokerMetadata drives the real v1 client constructor
// through a real TLS request and asserts the header arrives intact.
func TestManagementClientV1SendsInvokerMetadata(t *testing.T) {
	var captured capturedRequest
	server := newMetadataEchoServer(t, &captured)
	trustTestServer(t, server)

	metadata := invokerMetadata{InvokerKind: "agent", InvokerAgent: "claude-code", CI: true}

	api, err := initializeManagementClient(domainOf(server), "test-token", metadata.headerValue())
	require.NoError(t, err)

	_, err = api.ResourceServer.List(t.Context())
	require.NoError(t, err)

	assertMetadataHeader(t, captured, metadata)
	assert.NotEmpty(t, captured.auth0Cli, "Auth0-Client should still be sent alongside it")
}

// TestManagementClientV3SendsInvokerMetadata does the same for the v3 client, since
// both share customClientWithRetries and both must carry the header.
func TestManagementClientV3SendsInvokerMetadata(t *testing.T) {
	var captured capturedRequest
	server := newMetadataEchoServer(t, &captured)
	trustTestServer(t, server)

	metadata := invokerMetadata{InvokerKind: "human", InvokerAgent: "unknown", CI: false}

	api, err := initializeManagementClientV3(domainOf(server), "test-token", metadata.headerValue())
	require.NoError(t, err)

	_, err = api.ClientGrants.List(t.Context(), &management.ListClientGrantsRequestParameters{})
	require.NoError(t, err)

	assertMetadataHeader(t, captured, metadata)

	// Note: unlike v1, the v3 client does not send Auth0-Client here despite
	// option.WithAuth0ClientEnvEntry being configured. Passing option.WithHTTPClient
	// replaces the client the SDK built, discarding its Auth0-Client transport. That is
	// pre-existing SDK behaviour, unrelated to this header, and recorded for visibility.
	t.Logf("v3 Auth0-Client: %q", captured.auth0Cli)
}

// TestManagementClientSendsMetadataOnRetries proves the header is present on retried
// attempts too, not just the first, since it is stamped outside the retry transports.
func TestManagementClientSendsMetadataOnRetries(t *testing.T) {
	var attempts int
	var metadataPerAttempt []string

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		metadataPerAttempt = append(metadataPerAttempt, r.Header.Get(invokerMetadataHeader))

		// Fail the first attempt with a retryable status.
		if attempts == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)
	trustTestServer(t, server)

	metadata := invokerMetadata{InvokerKind: "agent", InvokerAgent: "cursor", CI: false}

	api, err := initializeManagementClient(domainOf(server), "test-token", metadata.headerValue())
	require.NoError(t, err)

	_, err = api.ResourceServer.List(t.Context())
	require.NoError(t, err)

	require.Equal(t, 2, attempts, "expected one retry after the 503")
	for attempt, value := range metadataPerAttempt {
		assert.Equal(t, metadata.headerValue(), value, "attempt %d lost the header", attempt+1)
	}
}
