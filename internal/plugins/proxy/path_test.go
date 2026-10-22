package proxy

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTarget(t *testing.T) {
	longSegments := "/api/v2" + strings.Repeat("/a", 16)

	tests := []struct {
		name         string
		target       string
		wantPath     string
		wantSegments []string
		wantQuery    string
		wantStatus   int
		wantCode     errorCode

		// The reason is part of the message, for the rejections that other checks
		// would also catch, so that a test fails when one of them is removed.
		wantReason string
	}{
		{name: "a plain call", target: "/api/v2/clients", wantPath: "/api/v2/clients", wantSegments: []string{"clients"}},
		{name: "an id with a pipe, escaped the way clients do", target: "/api/v2/users/auth0%7C123", wantPath: "/api/v2/users/auth0|123", wantSegments: []string{"users", "auth0|123"}},
		{name: "an id with a colon escaped", target: "/api/v2/users/a%3Ab", wantPath: "/api/v2/users/a:b", wantSegments: []string{"users", "a:b"}},
		{name: "escapes in lowercase", target: "/api/v2/users/a%7cb", wantPath: "/api/v2/users/a|b", wantSegments: []string{"users", "a|b"}},
		{name: "the query as it came", target: "/api/v2/clients?per_page=1&fields=a%2Cb", wantPath: "/api/v2/clients", wantSegments: []string{"clients"}, wantQuery: "per_page=1&fields=a%2Cb"},
		{name: "an empty query", target: "/api/v2/clients?", wantPath: "/api/v2/clients", wantSegments: []string{"clients"}},
		{name: "16 segments after the prefix", target: longSegments, wantPath: longSegments, wantSegments: strings.Split(longSegments[8:], "/")},

		{name: "dot dot", target: "/api/v2/../users", wantStatus: 400},
		{name: "dot dot after a resource", target: "/api/v2/clients/../users", wantStatus: 400},
		{name: "a dot", target: "/api/v2/./clients", wantStatus: 400},
		{name: "escaped dot dot", target: "/api/v2/%2e%2e/users", wantStatus: 400, wantReason: "escaped slash, dot, backslash, null byte or percent sign"},
		{name: "escaped dot dot in capitals", target: "/api/v2/%2E%2E/users", wantStatus: 400, wantReason: "escaped slash, dot, backslash, null byte or percent sign"},
		{name: "one escaped dot", target: "/api/v2/.%2e/users", wantStatus: 400, wantReason: "escaped slash, dot, backslash, null byte or percent sign"},
		{name: "an escaped slash", target: "/api/v2/clients%2fusers", wantStatus: 400, wantReason: "escaped slash, dot, backslash, null byte or percent sign"},
		{name: "an escaped slash in capitals", target: "/api/v2/clients%2Fusers", wantStatus: 400, wantReason: "escaped slash, dot, backslash, null byte or percent sign"},
		{name: "an escaped backslash", target: "/api/v2/clients%5cusers", wantStatus: 400, wantReason: "escaped slash, dot, backslash, null byte or percent sign"},
		{name: "an escaped backslash in capitals", target: "/api/v2/clients%5Cusers", wantStatus: 400, wantReason: "escaped slash, dot, backslash, null byte or percent sign"},
		{name: "a backslash", target: `/api/v2/clients\users`, wantStatus: 400},
		{name: "an escaped null byte", target: "/api/v2/clients%00", wantStatus: 400, wantReason: "escaped slash, dot, backslash, null byte or percent sign"},
		{name: "an escaped percent sign", target: "/api/v2/%252e%252e/users", wantStatus: 400, wantReason: "escaped slash, dot, backslash, null byte or percent sign"},
		{name: "an escaped newline", target: "/api/v2/clients%0a", wantStatus: 400},
		{name: "an escaped C1 control", target: "/api/v2/clients%c2%85", wantStatus: 400},
		{name: "an escaped semicolon", target: "/api/v2/clients%3bx", wantStatus: 400},
		{name: "a semicolon", target: "/api/v2/clients;x=1", wantStatus: 400},
		{name: "an escape of a letter", target: "/api/v2/%63lients", wantStatus: 400, wantReason: "needs no escape"},
		{name: "an escape of a digit", target: "/api/v2/users/%31", wantStatus: 400, wantReason: "needs no escape"},
		{name: "an escape cut short", target: "/api/v2/clients%2", wantStatus: 400},
		{name: "an escape that is not hexadecimal", target: "/api/v2/clients%zz", wantStatus: 400},
		{name: "an escape of invalid UTF-8", target: "/api/v2/clients%ff", wantStatus: 400},
		{name: "invalid UTF-8", target: "/api/v2/clients\xff", wantStatus: 400},
		{name: "a fragment", target: "/api/v2/clients#x", wantStatus: 400},
		{name: "a double slash", target: "/api/v2//clients", wantStatus: 400},
		{name: "a double slash at the start", target: "//api/v2/clients", wantStatus: 400},
		{name: "a trailing slash", target: "/api/v2/clients/", wantStatus: 400},
		{name: "the prefix with a trailing slash", target: "/api/v2/", wantStatus: 400},
		{name: "the root", target: "/", wantStatus: 400},
		{name: "17 segments after the prefix", target: longSegments + "/a", wantStatus: 400},
		{name: "an absolute URL", target: "http://example.com/api/v2/clients", wantStatus: 400},
		{name: "an authority", target: "example.com:443", wantStatus: 400},
		{name: "the asterisk", target: "*", wantStatus: 400},
		{name: "a query with a space", target: "/api/v2/clients?a=b c", wantStatus: 400},
		{name: "a query with a control character", target: "/api/v2/clients?a=\x01", wantStatus: 400},
		{name: "a query with a fragment", target: "/api/v2/clients?a=b#c", wantStatus: 400},
		{name: "a query with a byte above ASCII", target: "/api/v2/clients?a=\xc3\xa9", wantStatus: 400},

		{name: "a path of 2049 bytes", target: "/api/v2/" + strings.Repeat("a", 2041), wantStatus: 414},
		{name: "a query of 8193 bytes", target: "/api/v2/clients?" + strings.Repeat("a", 8193), wantStatus: 414},

		{name: "the token endpoint", target: "/oauth/token", wantStatus: 403, wantCode: codeForbiddenPath},
		{name: "the userinfo endpoint", target: "/userinfo", wantStatus: 403, wantCode: codeForbiddenPath},
		{name: "the prefix in capitals", target: "/API/V2/clients", wantStatus: 403, wantCode: codeForbiddenPath},
		{name: "the prefix with a capital", target: "/api/V2/clients", wantStatus: 403, wantCode: codeForbiddenPath},
		{name: "a look-alike prefix", target: "/api/v2x/clients", wantStatus: 403, wantCode: codeForbiddenPath},
		{name: "the prefix alone", target: "/api/v2", wantStatus: 403, wantCode: codeForbiddenPath},
		{name: "the prefix of another version", target: "/api/v3/clients", wantStatus: 403, wantCode: codeForbiddenPath},
		{name: "the prefix under another path", target: "/x/api/v2/clients", wantStatus: 403, wantCode: codeForbiddenPath},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, rej := parseTarget(test.target)

			if test.wantStatus != 0 {
				require.NotNil(t, rej)
				assert.Equal(t, test.wantStatus, rej.status)
				assert.Empty(t, got)

				if test.wantCode != "" {
					assert.Equal(t, test.wantCode, rej.code)
				}

				assert.Contains(t, rej.message, test.wantReason)

				return
			}

			require.Nil(t, rej)
			assert.Equal(t, test.wantPath, got.path)
			assert.Equal(t, test.wantSegments, got.segments)
			assert.Equal(t, test.wantQuery, got.query)
		})
	}
}

func TestParseTarget_ErrorsEchoOnlyASafeForm(t *testing.T) {
	_, rej := parseTarget("/api/v2/clients\x1b[31m%00" + strings.Repeat("a", 500))

	require.NotNil(t, rej)
	assert.Equal(t, http.StatusBadRequest, rej.status)
	assert.NotContains(t, rej.message, "\x1b")
	assert.Less(t, len(rej.message), 300)
}
