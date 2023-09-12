package cli

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCustomClientWithRetries(t *testing.T) {
	t.Run("it retries on rate limit error", func(t *testing.T) {
		apiCalls := 0
		fail := true
		testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			apiCalls++

			if fail {
				fail = false
				writer.WriteHeader(429)
				resetAt := time.Now().Add(time.Second).Unix()
				writer.Header().Set("X-RateLimit-Reset", strconv.Itoa(int(resetAt)))
				return
			}

			writer.WriteHeader(200)
		}))

		client := customClientWithRetries()

		request, err := http.NewRequest(http.MethodGet, testServer.URL, nil)
		require.NoError(t, err)

		response, err := client.Do(request)
		require.NoError(t, err)

		assert.Equal(t, 200, response.StatusCode)
		assert.False(t, fail)
		assert.Equal(t, 2, apiCalls)

		t.Cleanup(func() {
			testServer.Close()
			err := response.Body.Close()
			require.NoError(t, err)
		})
	})

	t.Run("it retries on server error", func(t *testing.T) {
		apiCalls := 0
		fail := true
		testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			apiCalls++

			if fail {
				fail = false
				writer.WriteHeader(500)
				return
			}

			writer.WriteHeader(200)
		}))

		client := customClientWithRetries()

		request, err := http.NewRequest(http.MethodGet, testServer.URL, nil)
		require.NoError(t, err)

		response, err := client.Do(request)
		require.NoError(t, err)

		assert.Equal(t, 200, response.StatusCode)
		assert.False(t, fail)
		assert.Equal(t, 2, apiCalls)

		t.Cleanup(func() {
			testServer.Close()
			err := response.Body.Close()
			require.NoError(t, err)
		})
	})

	t.Run("it does not retry more than 3 times on server error", func(t *testing.T) {
		apiCalls := 0
		testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			apiCalls++
			writer.WriteHeader(500)
		}))

		client := customClientWithRetries()

		request, err := http.NewRequest(http.MethodGet, testServer.URL, nil)
		require.NoError(t, err)

		response, err := client.Do(request)
		require.NoError(t, err)

		assert.Equal(t, 500, response.StatusCode)
		assert.Equal(t, 3+1, apiCalls) // 3 retries + 1 first call.

		t.Cleanup(func() {
			testServer.Close()
			err := response.Body.Close()
			require.NoError(t, err)
		})
	})
}

func TestRetryableErrorRetryFunc(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "NilError",
			err:      nil,
			expected: false,
		},
		{
			name: "TooManyRedirectsError",
			err: &url.Error{
				Op:  "Get",
				URL: "http://example.com",
				Err: errors.New("stopped after 5 redirects"),
			},
			expected: false,
		},
		{
			name: "UnsupportedProtocolSchemeError",
			err: &url.Error{
				Op:  "Get",
				URL: "ftp://example.com",
				Err: errors.New("unsupported protocol scheme"),
			},
			expected: false,
		},
		{
			name: "CertificateVerificationError",
			err: &url.Error{
				Op:  "Get",
				URL: "https://example.com",
				Err: &tls.CertificateVerificationError{},
			},
			expected: false,
		},
		{
			name: "UnknownAuthorityError",
			err: &url.Error{
				Op:  "Get",
				URL: "https://example.com",
				Err: x509.UnknownAuthorityError{},
			},
			expected: false,
		},
		{
			name:     "OtherError",
			err:      errors.New("some other error"),
			expected: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actual := retryableErrorRetryFunc(testCase.err)
			assert.Equal(t, testCase.expected, actual)
		})
	}
}
