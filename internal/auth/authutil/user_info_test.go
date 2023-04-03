package authutil

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserInfo(t *testing.T) {
	t.Run("Test success call", func(t *testing.T) {
		ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer token", r.Header.Get("authorization"))

			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"name": "Joe Bloggs"}`)
		}))

		defer ts.Close()
		parsedURL, err := url.Parse(ts.URL)
		assert.NoError(t, err)

		user, err := FetchUserInfo(ts.Client(), parsedURL.Host, "token")

		assert.NoError(t, err)
		assert.Equal(t, "Joe Bloggs", *user.Name)
	})
}
