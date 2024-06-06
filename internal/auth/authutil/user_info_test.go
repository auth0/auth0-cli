package authutil

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserInfo(t *testing.T) {
	t.Run("Successfully call user info endpoint", func(t *testing.T) {
		ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer token", r.Header.Get("authorization"))

			w.Header().Set("Content-Type", "application/json")
			_, err := io.WriteString(w, `{"name": "Joe Bloggs","email_verified":true}`)
			require.NoError(t, err)
		}))

		defer ts.Close()
		parsedURL, err := url.Parse(ts.URL)
		assert.NoError(t, err)

		user, err := FetchUserInfo(ts.Client(), parsedURL.Host, "token")

		assert.NoError(t, err)
		assert.Equal(t, "Joe Bloggs", *user.Name)
		assert.Equal(t, true, *user.EmailVerified)
	})

	t.Run("Successfully call user info endpoint with string encoded email verified field", func(t *testing.T) {
		ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer token", r.Header.Get("authorization"))

			w.Header().Set("Content-Type", "application/json")
			_, err := io.WriteString(w, `{"email_verified":"true"}`)
			require.NoError(t, err)
		}))

		defer ts.Close()
		parsedURL, err := url.Parse(ts.URL)
		assert.NoError(t, err)

		user, err := FetchUserInfo(ts.Client(), parsedURL.Host, "token")

		assert.NoError(t, err)
		assert.Equal(t, true, *user.EmailVerified)
	})

	testCases := []struct {
		name       string
		expect     string
		httpStatus int
		response   string
	}{
		{
			name:       "Bad status code",
			expect:     "unable to fetch user info: 404 Not Found",
			httpStatus: http.StatusNotFound,
		},
		{
			name:       "Malformed JSON",
			expect:     "cannot decode response: unexpected EOF",
			httpStatus: http.StatusOK,
			response:   `{ "foo": "bar" `,
		},
		{
			name:       "Email verified field not string or bool",
			expect:     "cannot decode response: email_verified field expected to be bool or string, got: float64",
			httpStatus: http.StatusOK,
			response:   `{ "email_verified": 0 }`,
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

			_, err = FetchUserInfo(ts.Client(), parsedURL.Host, "token")

			assert.EqualError(t, err, testCase.expect)
		})
	}
}
