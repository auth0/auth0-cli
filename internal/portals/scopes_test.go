package portals_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/portals"
)

func TestScopeFixes(t *testing.T) {
	t.Parallel()

	const loginPrefix = "Signed in as a user: auth0 login --scopes "

	t.Run("a user login asks for every scope the tool uses, so it does not drop the ones granted before", func(t *testing.T) {
		t.Parallel()

		login := portals.ScopeFixes([]string{"read:portals"})[0]

		assert.Equal(t, portals.RequiredScopes(), strings.Split(strings.TrimPrefix(login, loginPrefix), ","))
	})

	t.Run("a scope outside the list of the tool is still asked for, once", func(t *testing.T) {
		t.Parallel()

		login := portals.ScopeFixes([]string{"read:something_new", "read:something_new", "read:portals"})[0]

		assert.True(t, strings.HasSuffix(login, ",read:something_new"), login)
		assert.Equal(t, 1, strings.Count(login, "read:something_new"))
		assert.Equal(t, 1, strings.Count(login, "read:portals"))
	})

	t.Run("a client-credentials login only needs the missing scopes added to its grant", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t,
			"Signed in with client credentials: add read:portals, create:portals to the application's Management API grant",
			portals.ScopeFixes([]string{"read:portals", "create:portals"})[1],
		)
	})

	t.Run("without a named scope the grant is checked against all of them", func(t *testing.T) {
		t.Parallel()

		grant := portals.ScopeFixes(nil)[1]

		assert.True(t, strings.HasPrefix(grant, "Signed in with client credentials: check that the application's Management API grant has read:clients, "), grant)
		assert.NotContains(t, grant, "add  to")
	})
}

func TestRequiredScopes_ReturnsACopy(t *testing.T) {
	t.Parallel()

	scopes := portals.RequiredScopes()
	scopes[0] = "changed"

	assert.NotEqual(t, "changed", portals.RequiredScopes()[0])
}

func TestParseScopeGap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		err     error
		want    portals.ScopeGap
		wantGap bool
	}{
		{
			name:    "any of one scope",
			err:     &portals.APIError{Status: 403, Message: "Insufficient scope, expected any of: create:clients"},
			want:    portals.ScopeGap{Probe: "create client", Requirement: portals.RequirementAny, Scopes: []string{"create:clients"}},
			wantGap: true,
		},
		{
			name:    "all of several scopes, with spaces",
			err:     &portals.APIError{Status: 403, Message: "Insufficient scope, expected all of: create:forms, create:flows"},
			want:    portals.ScopeGap{Probe: "create client", Requirement: portals.RequirementAll, Scopes: []string{"create:forms", "create:flows"}},
			wantGap: true,
		},
		{
			name:    "the message with context added around the error",
			err:     wrapped(&portals.APIError{Status: 403, Message: "Insufficient scope, expected any of: read:portals"}),
			want:    portals.ScopeGap{Probe: "create client", Requirement: portals.RequirementAny, Scopes: []string{"read:portals"}},
			wantGap: true,
		},
		{name: "another status", err: &portals.APIError{Status: 500, Message: "expected any of: create:clients"}},
		{name: "a 403 without a scope message", err: &portals.APIError{Status: 403, Message: "Forbidden"}},
		{name: "an error that is not from the API", err: assert.AnError},
		{name: "no error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gap, isGap := portals.ParseScopeGap("create client", tt.err)

			assert.Equal(t, tt.wantGap, isGap)
			assert.Equal(t, tt.want, gap)
		})
	}
}

func TestScopeGap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		gap          portals.ScopeGap
		wantChoice   bool
		wantRequired []string
	}{
		{
			name:         "one scope is required",
			gap:          portals.ScopeGap{Requirement: portals.RequirementAny, Scopes: []string{"a"}},
			wantRequired: []string{"a"},
		},
		{
			name:         "any of several is a choice, and the first is the one to ask for",
			gap:          portals.ScopeGap{Requirement: portals.RequirementAny, Scopes: []string{"a", "b"}},
			wantChoice:   true,
			wantRequired: []string{"a"},
		},
		{
			name:         "all of several are required",
			gap:          portals.ScopeGap{Requirement: portals.RequirementAll, Scopes: []string{"a", "b"}},
			wantRequired: []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.wantChoice, tt.gap.IsChoice())
			assert.Equal(t, tt.wantRequired, tt.gap.Required())
		})
	}
}
