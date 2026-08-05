package display

import (
	"testing"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
)

func TestMakeClientGrantView(t *testing.T) {
	t.Run("maps the client-grant fields", func(t *testing.T) {
		grant := &managementv3.ClientGrantResponseContent{
			ID:                   auth0.String("cgr_1"),
			ClientID:             auth0.String("client-id-1"),
			Audience:             auth0.String("https://travel0.com/api"),
			Scope:                []string{"read:users", "update:users"},
			OrganizationUsage:    managementv3.ClientGrantOrganizationUsageEnumRequire.Ptr(),
			AllowAnyOrganization: auth0.Bool(true),
		}

		view, scopesTruncated := makeClientGrantView(grant)

		assert.Equal(t, "client-id-1", view.ClientID)
		assert.Equal(t, "https://travel0.com/api", view.Audience)
		assert.Equal(t, "read:users, update:users", view.Scopes)
		assert.Equal(t, "require", view.OrganizationUsage)
		assert.Equal(t, "✓", view.AllowAnyOrganization)
		assert.False(t, scopesTruncated)
	})

	t.Run("falls back to default_for when client id is empty", func(t *testing.T) {
		grant := &managementv3.ClientGrantResponseContent{
			ID:         auth0.String("cgr_2"),
			DefaultFor: managementv3.ClientGrantDefaultForEnumThirdPartyClients.Ptr(),
			Audience:   auth0.String("https://travel0.com/api"),
		}

		view, _ := makeClientGrantView(grant)

		assert.Equal(t, "third_party_clients", view.ClientID)
	})

	t.Run("shows (all scopes) when the grant allows all scopes", func(t *testing.T) {
		grant := &managementv3.ClientGrantResponseContent{
			ID:             auth0.String("cgr_3"),
			ClientID:       auth0.String("client-id-3"),
			Audience:       auth0.String("https://travel0.com/api"),
			AllowAllScopes: auth0.Bool(true),
		}

		view, scopesTruncated := makeClientGrantView(grant)

		assert.Equal(t, "(all scopes)", view.Scopes)
		assert.False(t, scopesTruncated)
	})
}

func TestClientGrantView_KeyValues(t *testing.T) {
	keys := func(keyValues [][]string) []string {
		out := make([]string, 0, len(keyValues))
		for _, kv := range keyValues {
			out = append(out, kv[0])
		}
		return out
	}

	t.Run("includes the organization rows when the grant uses organizations", func(t *testing.T) {
		grant := &managementv3.ClientGrantResponseContent{
			ID:                   auth0.String("cgr_1"),
			ClientID:             auth0.String("client-id-1"),
			Audience:             auth0.String("https://travel0.com/api"),
			Scope:                []string{"read:users"},
			OrganizationUsage:    managementv3.ClientGrantOrganizationUsageEnumRequire.Ptr(),
			AllowAnyOrganization: auth0.Bool(true),
		}

		view, _ := makeClientGrantView(grant)

		assert.Equal(t,
			[]string{"ID", "CLIENT ID", "AUDIENCE", "SCOPES", "SUBJECT TYPE", "ORGANIZATION USAGE", "ALLOW ANY ORGANIZATION"},
			keys(view.KeyValues()),
		)
	})

	t.Run("omits the organization rows when the grant has no organization usage", func(t *testing.T) {
		grant := &managementv3.ClientGrantResponseContent{
			ID:       auth0.String("cgr_2"),
			ClientID: auth0.String("client-id-2"),
			Audience: auth0.String("https://travel0.com/api"),
			Scope:    []string{"read:users"},
		}

		view, _ := makeClientGrantView(grant)

		assert.Equal(t,
			[]string{"ID", "CLIENT ID", "AUDIENCE", "SCOPES", "SUBJECT TYPE"},
			keys(view.KeyValues()),
		)
	})

	t.Run("shows the subject type for a non-client subject type", func(t *testing.T) {
		grant := &managementv3.ClientGrantResponseContent{
			ID:          auth0.String("cgr_3"),
			ClientID:    auth0.String("client-id-3"),
			Audience:    auth0.String("https://travel0.com/api"),
			Scope:       []string{},
			SubjectType: managementv3.ClientGrantSubjectTypeEnumUser.Ptr(),
		}

		view, _ := makeClientGrantView(grant)

		assert.Equal(t, "user", view.SubjectType)
	})

	t.Run("defaults the subject type to client when unset", func(t *testing.T) {
		grant := &managementv3.ClientGrantResponseContent{
			ID:       auth0.String("cgr_4"),
			ClientID: auth0.String("client-id-4"),
			Audience: auth0.String("https://travel0.com/api"),
			Scope:    []string{"read:users"},
		}

		view, _ := makeClientGrantView(grant)

		assert.Equal(t, "client", view.SubjectType)
	})
}

func TestMakeClientGrantTableView(t *testing.T) {
	t.Run("shows the scope count, not the scope values", func(t *testing.T) {
		grant := &managementv3.ClientGrantResponseContent{
			ID:       auth0.String("cgr_1"),
			ClientID: auth0.String("client-id-1"),
			Audience: auth0.String("https://travel0.com/api"),
			Scope:    []string{"read:users", "update:users", "delete:users"},
		}

		view := makeClientGrantTableView(grant)

		assert.Equal(t, "cgr_1", view.ID)
		assert.Equal(t, "client-id-1", view.ClientID)
		assert.Equal(t, "https://travel0.com/api", view.Audience)
		assert.Equal(t, "3", view.Scopes)
		assert.Equal(t, []string{"cgr_1", "client-id-1", "https://travel0.com/api", "3"}, view.AsTableRow())
	})

	t.Run("shows all when the grant allows all scopes", func(t *testing.T) {
		grant := &managementv3.ClientGrantResponseContent{
			ID:             auth0.String("cgr_3"),
			ClientID:       auth0.String("client-id-3"),
			Audience:       auth0.String("https://travel0.com/api"),
			AllowAllScopes: auth0.Bool(true),
		}

		view := makeClientGrantTableView(grant)

		assert.Equal(t, "all", view.Scopes)
		assert.Equal(t, []string{"cgr_3", "client-id-3", "https://travel0.com/api", "all"}, view.AsTableRow())
	})

	t.Run("falls back to default_for when client id is empty", func(t *testing.T) {
		grant := &managementv3.ClientGrantResponseContent{
			ID:         auth0.String("cgr_2"),
			DefaultFor: managementv3.ClientGrantDefaultForEnumThirdPartyClients.Ptr(),
			Audience:   auth0.String("https://travel0.com/api"),
		}

		view := makeClientGrantTableView(grant)

		assert.Equal(t, "third_party_clients", view.ClientID)
	})
}

func TestClientGrantTableView_AsTableHeader(t *testing.T) {
	view := clientGrantTableView{}
	assert.Equal(t, []string{"ID", "Client ID", "Audience", "Scopes"}, view.AsTableHeader())
}
