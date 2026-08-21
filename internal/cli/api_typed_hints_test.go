package cli

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSuggestTypedCommand(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		rawURI   string
		expected string
	}{
		{
			name:     "collection create maps to typed create",
			method:   http.MethodPost,
			rawURI:   "clients",
			expected: "auth0 apps create",
		},
		{
			name:     "collection list maps to typed list",
			method:   http.MethodGet,
			rawURI:   "clients",
			expected: "auth0 apps list",
		},
		{
			name:     "id path maps to typed show",
			method:   http.MethodGet,
			rawURI:   "clients/abc123",
			expected: "auth0 apps show",
		},
		{
			name:     "id path maps to typed delete",
			method:   http.MethodDelete,
			rawURI:   "clients/abc123",
			expected: "auth0 apps delete",
		},
		{
			name:     "nested actions path",
			method:   http.MethodPost,
			rawURI:   "actions/actions",
			expected: "auth0 actions create",
		},
		{
			name:     "nested actions id path",
			method:   http.MethodPatch,
			rawURI:   "actions/actions/act_1",
			expected: "auth0 actions update",
		},
		{
			name:     "sub-resource with placeholder before literal",
			method:   http.MethodGet,
			rawURI:   "roles/role_1/permissions",
			expected: "auth0 roles permissions list",
		},
		{
			name:     "leading and trailing slashes are ignored",
			method:   http.MethodGet,
			rawURI:   "/resource-servers/",
			expected: "auth0 apis list",
		},
		{
			name:     "query string is ignored",
			method:   http.MethodGet,
			rawURI:   "users?fields=email",
			expected: "auth0 users search",
		},
		{
			name:     "method is matched case-insensitively",
			method:   "get",
			rawURI:   "roles",
			expected: "auth0 roles list",
		},
		{
			name:     "no hint for endpoint without a typed command",
			method:   http.MethodGet,
			rawURI:   "connections",
			expected: "",
		},
		{
			name:     "no hint when method has no typed equivalent",
			method:   http.MethodPut,
			rawURI:   "clients/abc123",
			expected: "",
		},
		{
			name:     "no hint for unknown deeper path",
			method:   http.MethodGet,
			rawURI:   "clients/abc123/credentials",
			expected: "",
		},
		{
			name:     "empty path returns no hint",
			method:   http.MethodGet,
			rawURI:   "",
			expected: "",
		},
		{
			name:     "tenant settings singleton path",
			method:   http.MethodPatch,
			rawURI:   "tenants/settings",
			expected: "auth0 tenant-settings update",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := suggestTypedCommand(test.method, test.rawURI)
			assert.Equal(t, test.expected, actual)
		})
	}
}
