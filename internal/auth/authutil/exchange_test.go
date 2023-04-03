package authutil

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExchangeCodeForToken(t *testing.T) {
	t.Run("Test success call", func(t *testing.T) {
		ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{
				"access_token": "access-token-here",
				"id_token": "id-token-here",
				"token_type": "token-type-here",
				"expires_in": 1000
			}`)
		}))

		defer ts.Close()
		parsedURL, err := url.Parse(ts.URL)
		assert.NoError(t, err)

		token, err := ExchangeCodeForToken(ts.Client(), parsedURL.Host, "some-client-id", "some-client-secret", "some-code", "http://localhost:8484")

		assert.NoError(t, err)
		assert.Equal(t, "access-token-here", token.AccessToken)
		assert.Equal(t, "id-token-here", token.IDToken)
		assert.Equal(t, "token-type-here", token.TokenType)
		assert.Equal(t, int64(1000), token.ExpiresIn)
	})

}
