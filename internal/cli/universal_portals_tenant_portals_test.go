package cli

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/portals"
)

func TestPortalsTenant_Portals(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("a list is one request without paging, because the endpoint has none", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodGet, "portals", 200, `{"portals":`+portalsJSON(0, portalsPageSize+1)+`}`)

		listed, err := tenant.ListPortals(ctx)

		require.NoError(t, err)
		assert.Len(t, listed, portalsPageSize+1, "more than a page of portals in one answer is all of them")
		assert.Equal(t, portals.PortalSummary{ID: "up_0", Slug: "portal-0", Name: "Portal 0"}, listed[0])

		requests := server.received(http.MethodGet, "portals")
		require.Len(t, requests, 1)
		assert.Empty(t, requests[0].Query)
	})

	t.Run("a read returns the pages, the sidebar and the application", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodGet, "portals/up_1", 200, `{
			"id":"up_1","slug":"acme","name":"Acme",
			"client":{"client_id":"cli_1"},
			"navigation":{"sidebar":{"components":[{"type":"sidebar:component:auth0:internal_link:v2","config":{"label":"Profile","page_slug":"profile"}}]}},
			"pages":{"default":"profile","standard":[{"title":"Profile","slug":"profile","components":[
				{"type":"page:component:auth0:structure:section","config":{"variant":"card","children":[
					{"type":"page:component:auth0:form","config":{"form_id":"ap_1"}}]}}]}]}}`)

		portal, err := tenant.ReadPortal(ctx, "up_1")

		require.NoError(t, err)
		assert.Equal(t, "acme", portal.Slug)
		assert.Equal(t, "cli_1", portal.Client.ClientID)
		assert.Equal(t, "profile", portal.Pages.Default)
		assert.Equal(t, "page:component:auth0:structure:section", portal.Pages.Standard[0].Components[0].Type)
		assert.Equal(t, "page_slug", firstKey(portal.Navigation.Sidebar.Components[0].Config, "page_slug"))
	})

	t.Run("a read keeps a component type the SDK does not know", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodGet, "portals/up_1", 200, `{"id":"up_1","slug":"acme","pages":{"default":"p","standard":[{"title":"P","slug":"p","components":[
			{"type":"page:component:auth0:brand_new","config":{"anything":1}}]}]}}`)

		portal, err := tenant.ReadPortal(ctx, "up_1")

		require.NoError(t, err)
		assert.Equal(t, "page:component:auth0:brand_new", portal.Pages.Standard[0].Components[0].Type)
		assert.EqualValues(t, 1, portal.Pages.Standard[0].Components[0].Config["anything"])
	})

	t.Run("a create sends the application with its secret", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodPost, "portals", 201, `{"id":"up_1","slug":"acme","client":{"client_id":"cli_1"}}`)

		created, err := tenant.CreatePortal(ctx, portals.CreatePortalRequest{
			Slug: "acme",
			Name: "Acme",
			Client: &portals.PortalClient{
				ClientID:                "cli_1",
				ClientSecret:            "s3cr3t",
				TokenEndpointAuthMethod: "client_secret_post",
			},
			Pages: &portals.Pages{Default: "profile", Standard: []portals.Page{{Title: "Profile", Slug: "profile"}}},
		})

		require.NoError(t, err)
		assert.Equal(t, "up_1", created.ID)
		assert.JSONEq(t, `{
			"slug":"acme","name":"Acme",
			"client":{"client_id":"cli_1","client_secret":"s3cr3t","token_endpoint_auth_method":"client_secret_post"},
			"pages":{"default":"profile","standard":[{"title":"Profile","slug":"profile"}]}}`,
			server.received(http.MethodPost, "portals")[0].Body)
	})

	t.Run("a slug that is taken is a 409 with its code", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodPost, "portals", 409, `{"statusCode":409,"message":"Slug already in use","errorCode":"portal_conflict"}`)

		_, err := tenant.CreatePortal(ctx, portals.CreatePortalRequest{Slug: "acme", Client: &portals.PortalClient{}})

		assert.True(t, portals.IsStatus(err, http.StatusConflict), "got %v", err)
		assert.Equal(t, "portal_conflict", portals.CodeOf(err))
	})

	t.Run("an update sends only the parts that are set", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodPatch, "portals/up_1", 200, `{"id":"up_1"}`)

		err := tenant.UpdatePortal(ctx, "up_1", portals.PortalPatch{Pages: &portals.Pages{Default: "profile"}})

		require.NoError(t, err)
		assert.JSONEq(t, `{"pages":{"default":"profile"}}`, server.received(http.MethodPatch, "portals/up_1")[0].Body)
	})
}

func portalsJSON(first, last int) string {
	items := make([]string, 0, last-first)
	for number := first; number < last; number++ {
		id := itoa(number)
		items = append(items, `{"id":"up_`+id+`","slug":"portal-`+id+`","name":"Portal `+id+`"}`)
	}

	return "[" + strings.Join(items, ",") + "]"
}

// firstKey returns key when the config has it, so a test can tell a missing config from a wrong one.
func firstKey(config map[string]any, key string) string {
	if _, found := config[key]; found {
		return key
	}

	return ""
}
