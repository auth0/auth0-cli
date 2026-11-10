package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRejection_write(t *testing.T) {
	rej := reject(http.StatusForbidden, codeInsufficientScope, `Plugin a/b did not declare scope "read:users".`)
	rej.header = http.Header{}
	rej.header.Set("Allow", "GET")

	recorder := httptest.NewRecorder()
	rej.write(recorder)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	assert.Equal(t, "GET", recorder.Header().Get("Allow"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, map[string]any{
		"statusCode": float64(403),
		"error":      "Forbidden",
		"message":    `Plugin a/b did not declare scope "read:users".`,
		"errorCode":  "insufficient_scope",
	}, body)
}

func TestSafeText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain text", in: "GET /api/v2/users", want: "GET /api/v2/users"},
		{name: "a control character", in: "a\x1b[31mb", want: "a?[31mb"},
		{name: "a newline", in: "a\nb", want: "a?b"},
		{name: "a bidi override", in: "a\u202eb", want: "a?b"},
		{name: "invalid UTF-8", in: "a\xffb", want: "a?b"},
		{name: "long text", in: strings.Repeat("a", 100), want: strings.Repeat("a", 80) + "..."},
		{name: "text of exactly the limit", in: strings.Repeat("a", 80), want: strings.Repeat("a", 80)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, safeText(test.in))
		})
	}
}

func TestQuoted(t *testing.T) {
	assert.Equal(t, `"a\x1b[31m\u00e9"`, quoted("a\x1b[31mé"))
	assert.Equal(t, `"`+strings.Repeat("a", 80)+`..."`, quoted(strings.Repeat("a", 100)))
	assert.NotContains(t, quoted("\xff\xfe"), "\xff")
}
