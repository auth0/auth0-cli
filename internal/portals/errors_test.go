package portals_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/portals"
)

func TestAPIError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  *portals.APIError
		want string
	}{
		{
			name: "with a code and a message",
			err:  &portals.APIError{Status: 409, Code: "portal_conflict", Message: "Slug already in use"},
			want: "409 portal_conflict: Slug already in use",
		},
		{
			name: "with a message only",
			err:  &portals.APIError{Status: 404, Message: "The client does not exist"},
			want: "404: The client does not exist",
		},
		{
			name: "with a status only",
			err:  &portals.APIError{Status: 500},
			want: "500: Internal Server Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.err.Error())
		})
	}
}

func TestStatusCodeAndIsStatus(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("create client %q: %w", "Acme", &portals.APIError{Status: http.StatusConflict, Code: "conflict"})

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		isConflict bool
	}{
		{name: "an API error", err: &portals.APIError{Status: 409, Code: "conflict"}, wantStatus: 409, wantCode: "conflict", isConflict: true},
		{name: "an API error with context added", err: wrapped, wantStatus: 409, wantCode: "conflict", isConflict: true},
		{name: "another error", err: errors.New("network down")},
		{name: "no error"},
		{name: "another status", err: &portals.APIError{Status: 404}, wantStatus: 404},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.wantStatus, portals.StatusOf(tt.err))
			assert.Equal(t, tt.wantCode, portals.CodeOf(tt.err))
			assert.Equal(t, tt.isConflict, portals.IsStatus(tt.err, http.StatusConflict, http.StatusBadRequest))
		})
	}
}

func TestFixesOf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want []string
	}{
		{
			name: "an error that names its fixes",
			err:  &portals.MissingScopesError{Scopes: []string{"read:clients"}},
			want: portals.ScopeFixes([]string{"read:clients"}),
		},
		{
			name: "an error that names its fixes, with context added",
			err:  fmt.Errorf("set up: %w", &portals.MissingScopesError{Scopes: []string{"read:clients"}}),
			want: portals.ScopeFixes([]string{"read:clients"}),
		},
		{
			name: "a missing scope with nothing to ask for has no fixes",
			err:  &portals.MissingScopesError{},
		},
		{
			name: "a request refused for one of several scopes: the first is enough",
			err:  &portals.APIError{Status: 403, Code: "insufficient_scope", Message: "Insufficient scope, expected any of: read:portals,update:portals"},
			want: portals.ScopeFixes([]string{"read:portals"}),
		},
		{
			name: "a request refused for all of several scopes",
			err:  fmt.Errorf("read portal: %w", &portals.APIError{Status: 403, Message: "Insufficient scope, expected all of: read:portals,update:portals"}),
			want: portals.ScopeFixes([]string{"read:portals", "update:portals"}),
		},
		{name: "a 403 that is not about a scope", err: &portals.APIError{Status: 403, Code: "forbidden", Message: "Forbidden"}},
		{name: "another error", err: errors.New("network down")},
		{name: "no error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, portals.FixesOf(tt.err))
		})
	}
}

func TestPermissionCheckError_Fixes(t *testing.T) {
	t.Parallel()

	fixes := (&portals.PermissionCheckError{Probes: []string{"create portal"}, Tenant: "acme.us.auth0.com"}).Fixes()

	assert.Equal(t, "Check that Universal Portals is enabled for this tenant.", fixes[0])
	assert.Len(t, fixes, 3)
}
