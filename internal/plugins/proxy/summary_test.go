package proxy

import (
	"fmt"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
)

func TestDenialLog_summary(t *testing.T) {
	tests := []struct {
		name string
		do   func(l *denialLog)
		want string
	}{
		{name: "nothing was denied", do: func(*denialLog) {}},
		{
			name: "one call",
			do:   func(l *denialLog) { l.deny("read:users", "GET", "/api/v2/users") },
			want: "Plugin checkmate: 1 API call was denied (read:users for GET /api/v2/users). Review permissions.scopes in plugin.json.",
		},
		{
			name: "the same call twice",
			do: func(l *denialLog) {
				l.deny("read:users", "GET", "/api/v2/users")
				l.deny("read:users", "GET", "/api/v2/users")
			},
			want: "Plugin checkmate: 2 API calls were denied (read:users for GET /api/v2/users). Review permissions.scopes in plugin.json.",
		},
		{
			name: "a call that is denied for another reason than a scope",
			do:   func(l *denialLog) { l.deny("", "GET", "/oauth/token") },
			want: "Plugin checkmate: 1 API call was denied (GET /oauth/token). Review permissions.scopes in plugin.json.",
		},
		{
			name: "some different calls",
			do: func(l *denialLog) {
				l.deny("read:users", "GET", "/api/v2/users")
				l.deny("", "DELETE", "/api/v2/clients/a")
				l.deny("read:users", "GET", "/api/v2/users")
			},
			want: "Plugin checkmate: 3 API calls were denied (read:users for GET /api/v2/users, DELETE /api/v2/clients/a). Review permissions.scopes in plugin.json.",
		},
		{
			name: "only Auth0 refused",
			do:   func(l *denialLog) { l.refuse() },
			want: "Plugin checkmate: Auth0 refused 1 call because your session lacks a scope. Review permissions.scopes in plugin.json.",
		},
		{
			name: "both",
			do: func(l *denialLog) {
				l.deny("read:users", "GET", "/api/v2/users")
				l.refuse()
				l.refuse()
			},
			want: "Plugin checkmate: 1 API call was denied (read:users for GET /api/v2/users) and Auth0 refused 2 calls because your session lacks a scope. Review permissions.scopes in plugin.json.",
		},
		{
			name: "calls without a scope first do not push the missing scope out",
			do: func(l *denialLog) {
				l.deny("", "DELETE", "/api/v2/clients/a")
				l.deny("", "GET", "/oauth/token")
				l.deny("", "GET", "/userinfo")

				for range 5 {
					l.deny("read:users", "GET", "/api/v2/users")
				}
			},
			want: "Plugin checkmate: 8 API calls were denied (read:users for GET /api/v2/users, DELETE /api/v2/clients/a, GET /oauth/token, 1 other). Review permissions.scopes in plugin.json.",
		},
		{
			name: "every missing scope is named before the calls without one",
			do: func(l *denialLog) {
				l.deny("", "GET", "/oauth/token")
				l.deny("read:users", "GET", "/api/v2/users")
				l.deny("read:users", "GET", "/api/v2/users/a")
				l.deny("read:clients", "GET", "/api/v2/clients")
				l.deny("read:roles", "GET", "/api/v2/roles")
				l.deny("read:logs", "GET", "/api/v2/logs")
			},
			want: "Plugin checkmate: 6 API calls were denied (read:users for GET /api/v2/users, read:clients for GET /api/v2/clients, read:roles for GET /api/v2/roles, 3 other). Review permissions.scopes in plugin.json.",
		},
		{
			name: "many different calls",
			do: func(l *denialLog) {
				for n := range 30 {
					l.deny("read:users", "GET", fmt.Sprintf("/api/v2/users/%d", n))
				}
			},
			want: "Plugin checkmate: 30 API calls were denied (read:users for GET /api/v2/users/0, 29 other). Review permissions.scopes in plugin.json.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var log denialLog
			test.do(&log)

			assert.Equal(t, test.want, log.summary("checkmate"))
		})
	}
}

func TestDenialLog_keepsAtMostTwentyDifferentCalls(t *testing.T) {
	var log denialLog

	for n := range 100 {
		log.deny("", "GET", fmt.Sprintf("/api/v2/%d", n))
	}

	assert.Len(t, log.kept, 20)
	assert.Equal(t, 100, log.total)
	assert.Contains(t, log.summary("checkmate"), "100 API calls were denied")
}

func TestDenialLog_keepsAMissingScopeWhenMemoryIsFull(t *testing.T) {
	var log denialLog

	for n := range 50 {
		log.deny("", "GET", fmt.Sprintf("/api/v2/%d", n))
	}

	log.deny("read:users", "GET", "/api/v2/users")
	log.deny("read:users", "GET", "/api/v2/users/a")

	assert.Len(t, log.kept, 20)
	assert.Contains(t, log.summary("checkmate"), "(read:users for GET /api/v2/users, ")

	// A second scope is kept too, and more calls for a scope that is kept are
	// only counted.
	for n := range 20 {
		log.deny("read:logs", "GET", fmt.Sprintf("/api/v2/logs/%d", n))
	}

	assert.Len(t, log.kept, 20)
	assert.Contains(t, log.summary("checkmate"), "read:users for GET /api/v2/users")
	assert.Contains(t, log.summary("checkmate"), "read:logs for GET /api/v2/logs/0")
}

func TestDenialLog_summaryHasNoControlCharacters(t *testing.T) {
	var log denialLog

	log.deny("read:users", "GET\x1b[31m", "/api/v2/\x1b]0;title\x07\u202e\u0085\n"+strings.Repeat("a", 500))
	log.refuse()

	summary := log.summary("check\x1bmate\n")

	assert.NotEmpty(t, summary)
	assert.Equal(t, -1, strings.IndexFunc(summary, func(r rune) bool { return unicode.IsControl(r) || r > unicode.MaxASCII }))
	assert.Less(t, len(summary), 600)
}
