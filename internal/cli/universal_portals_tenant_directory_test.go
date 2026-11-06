package cli

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/portals"
)

func TestPortalsTenant_CustomDomain(t *testing.T) {
	t.Parallel()

	t.Run("it reads the default domain", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodGet, "custom-domains/default", 200, `{"domain":"login.acme.com","status":"ready"}`)

		domain, err := tenant.ReadDefaultCustomDomain(context.Background())

		require.NoError(t, err)
		assert.Equal(t, portals.CustomDomain{Domain: "login.acme.com", Status: "ready"}, domain)
	})

	t.Run("a tenant without one is a 404", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodGet, "custom-domains/default", 404, `{"statusCode":404,"message":"Not Found"}`)

		_, err := tenant.ReadDefaultCustomDomain(context.Background())

		assert.True(t, portals.IsStatus(err, http.StatusNotFound), "got %v", err)
	})
}

func TestPortalsTenant_Users(t *testing.T) {
	t.Parallel()

	t.Run("the email is searched as an exact phrase, with its quotes escaped", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodGet, "users", 200, `{"start":0,"limit":100,"total":1,"users":[
			{"user_id":"auth0|abc","email":"jane@acme.com","name":"Jane"}]}`)

		users, err := tenant.SearchUsersByEmail(context.Background(), `ja"ne@acme.com`)

		require.NoError(t, err)
		assert.Equal(t, []portals.User{{ID: "auth0|abc", Email: "jane@acme.com", Name: "Jane"}}, users)

		query := server.received(http.MethodGet, "users")[0].Query
		assert.Equal(t, []string{`email:"ja\"ne@acme.com"`}, query["q"])
		assert.Equal(t, []string{"v3"}, query["search_engine"])
	})

	t.Run("an ID with a pipe is escaped once", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodGet, "users/auth0%7Cabc", 200, `{"user_id":"auth0|abc","email":"jane@acme.com"}`)

		user, err := tenant.ReadUser(context.Background(), "auth0|abc")

		require.NoError(t, err)
		assert.Equal(t, "auth0|abc", user.ID)
	})

	t.Run("a user that does not exist is a 404", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodGet, "users/auth0%7Cnope", 404, `{"statusCode":404,"message":"The user does not exist."}`)

		_, err := tenant.ReadUser(context.Background(), "auth0|nope")

		assert.True(t, portals.IsStatus(err, http.StatusNotFound), "got %v", err)
	})

	t.Run("the organizations of a user", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodGet, "users/auth0%7Cabc/organizations", 200, `{"start":0,"limit":100,"total":1,"organizations":[
			{"id":"org_1","name":"acme","display_name":"Acme Corp"}]}`)

		organizations, err := tenant.ListUserOrganizations(context.Background(), "auth0|abc")

		require.NoError(t, err)
		assert.Equal(t, []portals.Organization{{ID: "org_1", Name: "acme", DisplayName: "Acme Corp"}}, organizations)
	})
}

func TestPortalsTenant_Organizations(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("it lists, reads and creates", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodGet, "organizations", 200, `{"start":0,"limit":100,"total":1,"organizations":[{"id":"org_1","name":"acme","display_name":"Acme"}]}`)
		server.reply(http.MethodGet, "organizations/org_1", 200, `{"id":"org_1","name":"acme","display_name":"Acme"}`)
		server.reply(http.MethodPost, "organizations", 201, `{"id":"org_2","name":"globex","display_name":"Globex"}`)

		listed, err := tenant.ListOrganizations(ctx)
		require.NoError(t, err)
		assert.Equal(t, []portals.Organization{{ID: "org_1", Name: "acme", DisplayName: "Acme"}}, listed)

		read, err := tenant.ReadOrganization(ctx, "org_1")
		require.NoError(t, err)
		assert.Equal(t, "acme", read.Name)

		created, err := tenant.CreateOrganization(ctx, portals.OrganizationSpec{Name: "globex", DisplayName: "Globex"})
		require.NoError(t, err)
		assert.Equal(t, portals.Organization{ID: "org_2", Name: "globex", DisplayName: "Globex"}, created)
		assert.JSONEq(t, `{"name":"globex","display_name":"Globex"}`, server.received(http.MethodPost, "organizations")[0].Body)
	})

	t.Run("a name that is taken is a 409", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodPost, "organizations", 409, `{"statusCode":409,"message":"The organization name is already in use."}`)

		_, err := tenant.CreateOrganization(ctx, portals.OrganizationSpec{Name: "acme"})

		assert.True(t, portals.IsStatus(err, http.StatusConflict), "got %v", err)
	})

	t.Run("a connection is enabled without assigning membership on login", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodPost, "organizations/org_1/enabled_connections", 201, `{"connection_id":"con_1"}`)
		server.reply(http.MethodGet, "organizations/org_1/enabled_connections", 200, `{"start":0,"limit":100,"total":1,"enabled_connections":[{"connection_id":"con_1"}]}`)

		require.NoError(t, tenant.EnableOrganizationConnection(ctx, "org_1", "con_1"))
		ids, err := tenant.ListOrganizationConnections(ctx, "org_1")

		require.NoError(t, err)
		assert.Equal(t, []string{"con_1"}, ids)
		assert.JSONEq(t,
			`{"connection_id":"con_1","assign_membership_on_login":false}`,
			server.received(http.MethodPost, "organizations/org_1/enabled_connections")[0].Body,
		)
	})

	t.Run("members and their roles", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodPost, "organizations/org_1/members", 204, ``)
		server.reply(http.MethodPost, "organizations/org_1/members/auth0%7Cabc/roles", 204, ``)
		server.reply(http.MethodGet, "organizations/org_1/members/auth0%7Cabc/roles", 200, `{"start":0,"limit":100,"total":1,"roles":[{"id":"rol_1","name":"Admin"}]}`)

		require.NoError(t, tenant.AddOrganizationMember(ctx, "org_1", "auth0|abc"))
		require.NoError(t, tenant.AssignMemberRole(ctx, "org_1", "auth0|abc", "rol_1"))
		roles, err := tenant.ListMemberRoles(ctx, "org_1", "auth0|abc")

		require.NoError(t, err)
		assert.Equal(t, []portals.Role{{ID: "rol_1", Name: "Admin"}}, roles)
		assert.JSONEq(t, `{"members":["auth0|abc"]}`, server.received(http.MethodPost, "organizations/org_1/members")[0].Body)
		assert.JSONEq(t, `{"roles":["rol_1"]}`, server.received(http.MethodPost, "organizations/org_1/members/auth0%7Cabc/roles")[0].Body)
	})
}

func TestPortalsTenant_Roles(t *testing.T) {
	t.Parallel()

	server, tenant := newPortalsServer(t)
	ctx := context.Background()
	server.reply(http.MethodGet, "roles", 200, `{"start":0,"limit":100,"total":1,"roles":[{"id":"rol_1","name":"Admin"}]}`)
	server.reply(http.MethodPost, "roles", 200, `{"id":"rol_2","name":"Universal Portals - Admin"}`)
	server.reply(http.MethodGet, "roles/rol_1/permissions", 200, `{"start":0,"limit":100,"total":1,"permissions":[
		{"permission_name":"read:my_org:details","resource_server_identifier":"https://acme/my-org/"}]}`)
	server.reply(http.MethodPost, "roles/rol_1/permissions", 201, ``)

	roles, err := tenant.ListRoles(ctx)
	require.NoError(t, err)
	assert.Equal(t, []portals.Role{{ID: "rol_1", Name: "Admin"}}, roles)

	created, err := tenant.CreateRole(ctx, portals.RoleSpec{Name: "Universal Portals - Admin", Description: "Organization permissions."})
	require.NoError(t, err)
	assert.Equal(t, portals.Role{ID: "rol_2", Name: "Universal Portals - Admin"}, created)
	assert.JSONEq(t,
		`{"name":"Universal Portals - Admin","description":"Organization permissions."}`,
		server.received(http.MethodPost, "roles")[0].Body,
	)

	permissions, err := tenant.ListRolePermissions(ctx, "rol_1")
	require.NoError(t, err)
	assert.Equal(t, []portals.Permission{{Name: "read:my_org:details", ResourceServerIdentifier: "https://acme/my-org/"}}, permissions)

	require.NoError(t, tenant.AddRolePermissions(ctx, "rol_1", []portals.Permission{
		{Name: "update:my_org:details", ResourceServerIdentifier: "https://acme/my-org/"},
	}))
	assert.JSONEq(t,
		`{"permissions":[{"permission_name":"update:my_org:details","resource_server_identifier":"https://acme/my-org/"}]}`,
		server.received(http.MethodPost, "roles/rol_1/permissions")[0].Body,
	)
}
