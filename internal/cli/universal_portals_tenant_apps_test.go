package cli

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/portals"
)

func intPointer(value int) *int { return &value }

func boolPointer(value bool) *bool { return &value }

func TestPortalsTenant_Clients(t *testing.T) {
	t.Parallel()

	t.Run("a list asks only for the fields it reads", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodGet, "clients", 200, `{"start":0,"limit":100,"total":1,"clients":[
			{"client_id":"cli_1","name":"Acme (Universal Portals)","client_metadata":{"portals_setup_slug":"acme","count":3},
			 "session_transfer":{"allowed_authentication_methods":["query"]}}]}`)

		clients, err := tenant.ListClients(context.Background())

		require.NoError(t, err)
		require.Len(t, clients, 1)
		assert.Equal(t, "cli_1", clients[0].ID)
		assert.Equal(t, "Acme (Universal Portals)", clients[0].Name)
		assert.Equal(t, map[string]string{"portals_setup_slug": "acme"}, clients[0].Metadata, "metadata that is not text is left out")
		assert.Equal(t, []string{"query"}, clients[0].SessionTransfer.AllowedAuthenticationMethods)
		assert.Equal(t, []string{"client_id,name,client_metadata,session_transfer"}, server.received(http.MethodGet, "clients")[0].Query["fields"])
	})

	t.Run("a read returns the whole configuration and never the secret", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodGet, "clients/cli_1", 200, `{
			"client_id":"cli_1","client_secret":"must-not-be-returned","name":"Acme","app_type":"regular_web",
			"is_first_party":true,"oidc_conformant":true,"token_endpoint_auth_method":"client_secret_post",
			"is_token_endpoint_ip_header_trusted":true,"require_proof_of_possession":false,
			"callbacks":["https://acme/cb"],"allowed_logout_urls":["https://acme"],
			"grant_types":["authorization_code","refresh_token"],
			"organization_usage":"allow","organization_require_behavior":"no_prompt",
			"refresh_token":{"rotation_type":"non-rotating","expiration_type":"expiring","token_lifetime":86400,
				"idle_token_lifetime":3600,"infinite_idle_token_lifetime":false,
				"policies":[{"audience":"https://acme/me/","scope":["read:me:factors"]}]},
			"session_transfer":{"allowed_authentication_methods":["query"],"enforce_device_binding":"ip"},
			"oidc_logout":{"backchannel_logout_urls":["https://acme/logout"],
				"backchannel_logout_initiators":{"mode":"custom","selected_initiators":["rp-logout"]}}}`)

		client, err := tenant.ReadClient(context.Background(), "cli_1")

		require.NoError(t, err)
		assert.Empty(t, client.Secret)
		assert.Equal(t, portals.Client{
			ID: "cli_1",
			ClientSettings: portals.ClientSettings{
				Name:                        "Acme",
				AppType:                     "regular_web",
				IsFirstParty:                boolPointer(true),
				OIDCConformant:              boolPointer(true),
				TokenEndpointAuthMethod:     "client_secret_post",
				IPHeaderTrusted:             boolPointer(true),
				RequireProofOfPossession:    boolPointer(false),
				Callbacks:                   []string{"https://acme/cb"},
				AllowedLogoutURLs:           []string{"https://acme"},
				GrantTypes:                  []string{"authorization_code", "refresh_token"},
				OrganizationUsage:           "allow",
				OrganizationRequireBehavior: "no_prompt",
				RefreshToken: &portals.RefreshToken{
					RotationType:              "non-rotating",
					ExpirationType:            "expiring",
					TokenLifetime:             intPointer(86400),
					IdleTokenLifetime:         intPointer(3600),
					InfiniteIdleTokenLifetime: boolPointer(false),
					Policies: []portals.RefreshTokenPolicy{
						{Audience: "https://acme/me/", Scope: []string{"read:me:factors"}},
					},
				},
				SessionTransfer: &portals.SessionTransfer{
					AllowedAuthenticationMethods: []string{"query"},
					EnforceDeviceBinding:         "ip",
				},
				OIDCLogout: &portals.OIDCLogout{
					BackchannelLogoutURLs: []string{"https://acme/logout"},
					Mode:                  "custom",
					SelectedInitiators:    []string{"rp-logout"},
				},
			},
		}, client)
	})

	t.Run("a secret is read on its own, and is empty when the login cannot read it", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodGet, "clients/cli_1", 200, `{"client_id":"cli_1","client_secret":"s3cr3t"}`)
		server.reply(http.MethodGet, "clients/cli_2", 200, `{"client_id":"cli_2"}`)

		secret, err := tenant.ReadClientSecret(context.Background(), "cli_1")
		require.NoError(t, err)
		assert.Equal(t, "s3cr3t", secret)

		secret, err = tenant.ReadClientSecret(context.Background(), "cli_2")
		require.NoError(t, err)
		assert.Empty(t, secret)
		assert.Equal(t, []string{"client_id,client_secret"}, server.received(http.MethodGet, "clients/cli_1")[0].Query["fields"])
	})

	t.Run("a create sends every setting and returns the ID and the secret", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodPost, "clients", 201, `{"client_id":"cli_1","client_secret":"s3cr3t","name":"Acme"}`)

		created, err := tenant.CreateClient(context.Background(), portals.ClientSettings{
			Name:                        "Acme",
			AppType:                     "regular_web",
			IsFirstParty:                boolPointer(true),
			OIDCConformant:              boolPointer(true),
			TokenEndpointAuthMethod:     "client_secret_post",
			IPHeaderTrusted:             boolPointer(true),
			Callbacks:                   []string{"https://acme/cb"},
			AllowedLogoutURLs:           []string{"https://acme"},
			GrantTypes:                  []string{"authorization_code"},
			OrganizationUsage:           "allow",
			OrganizationRequireBehavior: "no_prompt",
			Metadata:                    map[string]string{"portals_setup_slug": "acme"},
			RefreshToken: &portals.RefreshToken{
				RotationType:      "non-rotating",
				ExpirationType:    "expiring",
				TokenLifetime:     intPointer(86400),
				IdleTokenLifetime: intPointer(3600),
				Policies:          []portals.RefreshTokenPolicy{{Audience: "https://acme/me/", Scope: []string{"read:me:factors"}}},
			},
			SessionTransfer: &portals.SessionTransfer{AllowedAuthenticationMethods: []string{"query"}, EnforceDeviceBinding: "ip"},
			OIDCLogout: &portals.OIDCLogout{
				BackchannelLogoutURLs: []string{"https://acme/logout"},
				Mode:                  "custom",
				SelectedInitiators:    []string{"rp-logout", "session-expired"},
			},
		})

		require.NoError(t, err)
		assert.Equal(t, "cli_1", created.ID)
		assert.Equal(t, "s3cr3t", created.Secret)
		assert.JSONEq(t, `{
			"name":"Acme","app_type":"regular_web","is_first_party":true,"oidc_conformant":true,
			"token_endpoint_auth_method":"client_secret_post","is_token_endpoint_ip_header_trusted":true,
			"callbacks":["https://acme/cb"],"allowed_logout_urls":["https://acme"],"grant_types":["authorization_code"],
			"organization_usage":"allow","organization_require_behavior":"no_prompt",
			"client_metadata":{"portals_setup_slug":"acme"},
			"refresh_token":{"rotation_type":"non-rotating","expiration_type":"expiring","token_lifetime":86400,
				"idle_token_lifetime":3600,"policies":[{"audience":"https://acme/me/","scope":["read:me:factors"]}]},
			"session_transfer":{"allowed_authentication_methods":["query"],"enforce_device_binding":"ip"},
			"oidc_logout":{"backchannel_logout_urls":["https://acme/logout"],
				"backchannel_logout_initiators":{"mode":"custom","selected_initiators":["rp-logout","session-expired"]}}}`,
			server.received(http.MethodPost, "clients")[0].Body)
	})

	t.Run("an update sends only the settings that are set", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodPatch, "clients/cli_1", 200, `{"client_id":"cli_1"}`)

		err := tenant.UpdateClient(context.Background(), "cli_1", portals.ClientSettings{
			Callbacks:       []string{"https://acme/cb"},
			IPHeaderTrusted: boolPointer(false),
		})

		require.NoError(t, err)
		assert.JSONEq(t,
			`{"callbacks":["https://acme/cb"],"is_token_endpoint_ip_header_trusted":false}`,
			server.received(http.MethodPatch, "clients/cli_1")[0].Body,
		)
	})
}

func TestPortalsTenant_ClientGrants(t *testing.T) {
	t.Parallel()

	t.Run("a list follows the checkpoint and filters by application", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.handle(http.MethodGet, "client-grants", func(r receivedRequest) (int, string) {
			if r.Query["from"] == nil {
				return 200, `{"client_grants":[{"id":"cgr_1","client_id":"cli_1","audience":"https://acme/me/","scope":["a"]}],"next":"cursor"}`
			}

			return 200, `{"client_grants":[{"id":"cgr_2","client_id":"cli_1","audience":"https://acme/my-org/","scope":["b","c"]}]}`
		})

		grants, err := tenant.ListClientGrants(context.Background(), "cli_1")

		require.NoError(t, err)
		assert.Equal(t, []portals.ClientGrant{
			{ID: "cgr_1", ClientID: "cli_1", Audience: "https://acme/me/", Scope: []string{"a"}},
			{ID: "cgr_2", ClientID: "cli_1", Audience: "https://acme/my-org/", Scope: []string{"b", "c"}},
		}, grants)

		requests := server.received(http.MethodGet, "client-grants")
		require.Len(t, requests, 2)
		assert.Equal(t, []string{"cli_1"}, requests[0].Query["client_id"])
		assert.Equal(t, []string{"cursor"}, requests[1].Query["from"])
	})

	tests := []struct {
		name        string
		subjectType string
		wantSubject any
		wantErr     string
	}{
		{name: "a grant for users", subjectType: "user", wantSubject: "user"},
		{name: "a grant for the application itself", subjectType: "client", wantSubject: "client"},
		{name: "a grant that leaves the subject to the API", subjectType: ""},
		{name: "a subject type nobody knows", subjectType: "robot", wantErr: `unknown client grant subject type "robot"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, tenant := newPortalsServer(t)
			server.reply(http.MethodPost, "client-grants", 201, `{"id":"cgr_1","client_id":"cli_1","audience":"https://acme/me/","scope":["a"]}`)

			grant, err := tenant.CreateClientGrant(context.Background(), portals.ClientGrantSpec{
				ClientID:    "cli_1",
				Audience:    "https://acme/me/",
				Scope:       []string{"a"},
				SubjectType: tt.subjectType,
			})

			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				assert.Empty(t, server.received(http.MethodPost, "client-grants"))
				return
			}

			require.NoError(t, err)
			assert.Equal(t, "cgr_1", grant.ID)

			body := server.received(http.MethodPost, "client-grants")[0].json(t)
			assert.Equal(t, "cli_1", body["client_id"])
			assert.Equal(t, tt.wantSubject, body["subject_type"])
		})
	}

	t.Run("an update sends the scopes", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodPatch, "client-grants/cgr_1", 200, `{"id":"cgr_1"}`)

		require.NoError(t, tenant.UpdateClientGrantScope(context.Background(), "cgr_1", []string{"a", "b"}))

		assert.JSONEq(t, `{"scope":["a","b"]}`, server.received(http.MethodPatch, "client-grants/cgr_1")[0].Body)
	})
}

func TestPortalsTenant_Vault(t *testing.T) {
	t.Parallel()

	setup := portals.VaultSetup{Domain: "acme.us.auth0.com", ClientID: "cli_2", ClientSecret: "s3cr3t"}

	t.Run("a create sends the setup of an Auth0 application", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodPost, "flows/vault/connections", 201, `{"id":"ac_1","name":"Universal Portals forms vault"}`)

		created, err := tenant.CreateVaultConnection(context.Background(), "Universal Portals forms vault", setup)

		require.NoError(t, err)
		assert.Equal(t, portals.VaultConnection{ID: "ac_1", Name: "Universal Portals forms vault"}, created)
		assert.JSONEq(t, `{
			"app_id":"AUTH0","name":"Universal Portals forms vault",
			"setup":{"domain":"acme.us.auth0.com","client_id":"cli_2","client_secret":"s3cr3t","type":"OAUTH_APP"}}`,
			server.received(http.MethodPost, "flows/vault/connections")[0].Body)
	})

	t.Run("an update sends only the setup", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodPatch, "flows/vault/connections/ac_1", 200, `{"id":"ac_1"}`)

		require.NoError(t, tenant.UpdateVaultConnection(context.Background(), "ac_1", setup))

		assert.JSONEq(t,
			`{"setup":{"domain":"acme.us.auth0.com","client_id":"cli_2","client_secret":"s3cr3t","type":"OAUTH_APP"}}`,
			server.received(http.MethodPatch, "flows/vault/connections/ac_1")[0].Body,
		)
	})

	t.Run("a list returns the IDs and the names", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodGet, "flows/vault/connections", 200, `{"connections":[{"id":"ac_1","name":"Vault"}]}`)

		listed, err := tenant.ListVaultConnections(context.Background())

		require.NoError(t, err)
		assert.Equal(t, []portals.VaultConnection{{ID: "ac_1", Name: "Vault"}}, listed)
	})
}

func TestPortalsTenant_Connections(t *testing.T) {
	t.Parallel()

	t.Run("the connections of an application page by checkpoint and never by page", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.handle(http.MethodGet, "clients/cli_1/connections", func(r receivedRequest) (int, string) {
			if r.Query["from"] == nil {
				return 200, `{"connections":[{"id":"con_1","name":"db"}],"next":"cursor"}`
			}

			return 200, `{"connections":[{"id":"con_2","name":"google"}]}`
		})

		connections, err := tenant.ListClientConnections(context.Background(), "cli_1")

		require.NoError(t, err)
		assert.Equal(t, []portals.Connection{{ID: "con_1", Name: "db"}, {ID: "con_2", Name: "google"}}, connections)

		for _, request := range server.received(http.MethodGet, "clients/cli_1/connections") {
			assert.Equal(t, []string{"100"}, request.Query["take"])
			assert.NotContains(t, request.Query, "page")
			assert.NotContains(t, request.Query, "per_page")
		}
	})

	t.Run("the database connections are asked for by strategy", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodGet, "connections", 200, `{"connections":[{"id":"con_1","name":"db","strategy":"auth0"}]}`)

		connections, err := tenant.ListDatabaseConnections(context.Background())

		require.NoError(t, err)
		assert.Equal(t, []portals.Connection{{ID: "con_1", Name: "db"}}, connections)
		assert.Equal(t, []string{"auth0"}, server.received(http.MethodGet, "connections")[0].Query["strategy"])
	})
}

func TestPortalsTenant_TracksWhetherItChangedTheTenant(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("reads and checks do not change it", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodGet, "resource-servers", 200, `{"resource_servers":[],"total":0}`)
		server.reply(http.MethodPost, "clients", 400, `{"statusCode":400,"message":"Payload validation error"}`)

		_, err := tenant.ListResourceServers(ctx)
		require.NoError(t, err)
		assert.Error(t, tenant.Probe(ctx, http.MethodPost, "clients", map[string]any{}))

		assert.False(t, tenant.hasWritten())
	})

	t.Run("a request that failed did not change it", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodPost, "resource-servers", 409, `{"statusCode":409,"message":"already exists"}`)

		_, err := tenant.CreateResourceServer(ctx, portals.ResourceServerSpec{Identifier: "https://acme/me/"})
		require.Error(t, err)

		assert.False(t, tenant.hasWritten())
	})

	writes := map[string]func(tenant *portalsTenant) error{
		"a created resource server": func(tenant *portalsTenant) error {
			_, err := tenant.CreateResourceServer(ctx, portals.ResourceServerSpec{Identifier: "https://acme/me/"})
			return err
		},
		"a created application": func(tenant *portalsTenant) error {
			_, err := tenant.CreateClient(ctx, portals.ClientSettings{Name: "Acme"})
			return err
		},
		"an updated application": func(tenant *portalsTenant) error {
			return tenant.UpdateClient(ctx, "cli_1", portals.ClientSettings{Name: "Acme"})
		},
		"a created grant": func(tenant *portalsTenant) error {
			_, err := tenant.CreateClientGrant(ctx, portals.ClientGrantSpec{ClientID: "cli_1", Audience: "https://acme/me/"})
			return err
		},
		"an updated grant": func(tenant *portalsTenant) error {
			return tenant.UpdateClientGrantScope(ctx, "cgr_1", []string{"a"})
		},
		"a created vault connection": func(tenant *portalsTenant) error {
			_, err := tenant.CreateVaultConnection(ctx, "Vault", portals.VaultSetup{})
			return err
		},
		"an imported form": func(tenant *portalsTenant) error {
			_, err := tenant.ImportForm(ctx, map[string]any{"form": map[string]any{"name": "Profile"}})
			return err
		},
		"a created portal": func(tenant *portalsTenant) error {
			_, err := tenant.CreatePortal(ctx, portals.CreatePortalRequest{Slug: "acme", Client: &portals.PortalClient{ClientID: "cli_1"}})
			return err
		},
		"a created organization": func(tenant *portalsTenant) error {
			_, err := tenant.CreateOrganization(ctx, portals.OrganizationSpec{Name: "acme"})
			return err
		},
		"a member added to an organization": func(tenant *portalsTenant) error {
			return tenant.AddOrganizationMember(ctx, "org_1", "auth0|a")
		},
		"a role assigned to a member": func(tenant *portalsTenant) error {
			return tenant.AssignMemberRole(ctx, "org_1", "auth0|a", "rol_1")
		},
		"a created role": func(tenant *portalsTenant) error {
			_, err := tenant.CreateRole(ctx, portals.RoleSpec{Name: "Admin"})
			return err
		},
	}

	for name, write := range writes {
		t.Run(name+" changes it", func(t *testing.T) {
			t.Parallel()

			server, tenant := newPortalsServer(t)
			for _, route := range []struct{ method, path string }{
				{http.MethodPost, "resource-servers"}, {http.MethodPost, "clients"}, {http.MethodPatch, "clients/cli_1"},
				{http.MethodPost, "client-grants"}, {http.MethodPatch, "client-grants/cgr_1"},
				{http.MethodPost, "flows/vault/connections"}, {http.MethodPost, "forms/import"}, {http.MethodPost, "portals"},
				{http.MethodPost, "organizations"}, {http.MethodPost, "organizations/org_1/members"},
				{http.MethodPost, "organizations/org_1/members/auth0%7Ca/roles"}, {http.MethodPost, "roles"},
			} {
				server.reply(route.method, route.path, 200, `{"id":"x","client_id":"x","form":{"id":"ap_1"}}`)
			}

			require.NoError(t, write(tenant))

			assert.True(t, tenant.hasWritten())
		})
	}
}
