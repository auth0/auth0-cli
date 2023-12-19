package auth

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWaitUntilUserLogsIn(t *testing.T) {
	state := State{
		"1234",
		"12345",
		"https://example.com/12345",
		1000,
		1,
	}

	t.Run("successfully waits and handles response", func(t *testing.T) {
		counter := 0
		tokenResponse := `{
			"access_token": "Zm9v.eyJhdWQiOiBbImh0dHBzOi8vYXV0aDAtY2xpLXRlc3QudXMuYXV0aDAuY29tL2FwaS92Mi8iXX0",
			"id_token": "id-token-here",
			"refresh_token": "refresh-token-here",
			"scope": "scope-here",
			"token_type": "token-type-here",
			"expires_in": 1000
		}`
		ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			if counter < 1 {
				_, err := io.WriteString(w, `{
					"error": "authorization_pending",
					"error_description": "still pending auth"
				}`)
				require.NoError(t, err)
			} else {
				_, err := io.WriteString(w, tokenResponse)
				require.NoError(t, err)
			}
			counter++
		}))

		defer ts.Close()

		parsedURL, err := url.Parse(ts.URL)
		assert.NoError(t, err)
		u := url.URL{Scheme: "https", Host: parsedURL.Host, Path: "/oauth/token"}
		credentials.OauthTokenEndpoint = u.String()

		result, err := WaitUntilUserLogsIn(context.Background(), ts.Client(), state)

		assert.NoError(t, err)
		assert.Equal(t, "auth0-cli-test", result.Tenant)
		assert.Equal(t, "auth0-cli-test.us.auth0.com", result.Domain)
	})

	testCases := []struct {
		name       string
		httpStatus int
		response   string
		expect     string
	}{
		{
			name:       "handle malformed JSON",
			httpStatus: http.StatusOK,
			response:   "foo",
			expect:     "cannot decode response: invalid character 'o' in literal false (expecting 'a')",
		},
		{
			name:       "should pass through authorization server errors",
			httpStatus: http.StatusOK,
			response:   "{\"error\": \"slow_down\", \"error_description\": \"slow down!\"}",
			expect:     "slow down!",
		},
		{
			name:       "should error if can't parse tenant info",
			httpStatus: http.StatusOK,
			response:   "{\"access_token\": \"bad.token\"}",
			expect:     "cannot parse tenant from the given access token: illegal base64 data at input byte 4",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(testCase.httpStatus)
				if testCase.response != "" {
					_, err := io.WriteString(w, testCase.response)
					require.NoError(t, err)
				}
			}))

			defer ts.Close()

			parsedURL, err := url.Parse(ts.URL)
			assert.NoError(t, err)
			u := url.URL{Scheme: "https", Host: parsedURL.Host, Path: "/oauth/token"}
			credentials.OauthTokenEndpoint = u.String()

			_, err = WaitUntilUserLogsIn(context.Background(), ts.Client(), state)

			assert.EqualError(t, err, testCase.expect)
		})
	}
}

func TestGetDeviceCode(t *testing.T) {
	t.Run("successfully retrieve state from response", func(t *testing.T) {
		ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, err := io.WriteString(w, `{
				"device_code": "device-code-here",
				"user_code": "user-code-here",
				"verification_uri_complete": "verification-uri-here",
				"expires_in": 1000,
				"interval": 1
			}`)
			require.NoError(t, err)
		}))

		defer ts.Close()

		parsedURL, err := url.Parse(ts.URL)
		assert.NoError(t, err)
		u := url.URL{Scheme: "https", Host: parsedURL.Host, Path: "/oauth/device/code"}
		credentials.DeviceCodeEndpoint = u.String()

		state, err := GetDeviceCode(context.Background(), ts.Client(), []string{})

		assert.NoError(t, err)
		assert.Equal(t, "device-code-here", state.DeviceCode)
		assert.Equal(t, "user-code-here", state.UserCode)
		assert.Equal(t, "verification-uri-here", state.VerificationURI)
		assert.Equal(t, 1000, state.ExpiresIn)
		assert.Equal(t, 1, state.Interval)
		assert.Equal(t, time.Duration(4000000000), state.IntervalDuration())
	})

	testCases := []struct {
		name       string
		httpStatus int
		response   string
		expect     string
	}{
		{
			name:       "handle HTTP status errors",
			httpStatus: http.StatusNotFound,
			response:   "Test response return",
			expect:     "received a 404 response: Test response return",
		},
		{
			name:       "handle bad JSON response",
			httpStatus: http.StatusOK,
			response:   "foo",
			expect:     "failed to decode the response: invalid character 'o' in literal false (expecting 'a')",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(testCase.httpStatus)
				if testCase.response != "" {
					_, err := io.WriteString(w, testCase.response)
					require.NoError(t, err)
				}
			}))

			defer ts.Close()

			parsedURL, err := url.Parse(ts.URL)
			assert.NoError(t, err)
			u := url.URL{Scheme: "https", Host: parsedURL.Host, Path: "/oauth/device/code"}
			credentials.DeviceCodeEndpoint = u.String()

			_, err = GetDeviceCode(context.Background(), ts.Client(), []string{})

			assert.EqualError(t, err, testCase.expect)
		})
	}
}

func TestParseTenant(t *testing.T) {
	t.Run("Successfully parse tenant and domain", func(t *testing.T) {
		tenant, domain, err := parseTenant("Zm9v.eyJhdWQiOiBbImh0dHBzOi8vYXV0aDAtY2xpLXRlc3QudXMuYXV0aDAuY29tL2FwaS92Mi8iXX0")
		assert.NoError(t, err)
		assert.Equal(t, "auth0-cli-test", tenant)
		assert.Equal(t, "auth0-cli-test.us.auth0.com", domain)
	})

	testCases := []struct {
		name        string
		accessToken string
		err         string
	}{
		{
			name:        "bad base64 encoding",
			accessToken: "bad.token.foo",
			err:         "illegal base64 data at input byte 4",
		},
		{
			name:        "bad json encoding",
			accessToken: "Zm9v.Zm9v", // Foo encoded in base64.
			err:         "invalid character 'o' in literal false (expecting 'a')",
		},
		{
			name:        "invalid URL in aud array",
			accessToken: "Zm9v.eyJhdWQiOiBbIjpleGFtcGxlLmNvbSJdfQ",
			err:         "parse \":example.com\": missing protocol scheme",
		},
		{
			name:        "no matching URL aud array",
			accessToken: "Zm9v.eyJhdWQiOiBbImh0dHBzOi8vZXhhbXBsZXMuY29tIl19",
			err:         "audience not found for /api/v2/",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tenant, domain, err := parseTenant(testCase.accessToken)
			assert.EqualError(t, err, testCase.err)
			assert.Equal(t, "", tenant)
			assert.Equal(t, "", domain)
		})
	}
}
