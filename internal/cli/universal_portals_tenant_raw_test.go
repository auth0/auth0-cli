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

func TestPortalsTenant_Forms(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("a list follows the pages", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.handle(http.MethodGet, "forms", func(r receivedRequest) (int, string) {
			page := "0"
			if r.Query["page"] != nil {
				page = r.Query["page"][0]
			}

			switch page {
			case "0":
				return 200, `{"start":0,"limit":1,"total":2,"forms":[{"id":"ap_1","name":"Profile","created_at":"2026-01-01T00:00:00.000Z","updated_at":"2026-01-01T00:00:00.000Z"}]}`
			case "1":
				return 200, `{"start":1,"limit":1,"total":2,"forms":[{"id":"ap_2","name":"Privacy","created_at":"2026-01-01T00:00:00.000Z","updated_at":"2026-01-01T00:00:00.000Z"}]}`
			default:
				return 200, `{"start":2,"limit":1,"total":2,"forms":[]}`
			}
		})

		forms, err := tenant.ListForms(ctx)

		require.NoError(t, err)
		assert.Equal(t, []portals.Form{{ID: "ap_1", Name: "Profile"}, {ID: "ap_2", Name: "Privacy"}}, forms)
	})

	t.Run("a read takes the flow from the flow node, and ignores a node it does not know", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodGet, "forms/ap_1", 200, `{"id":"ap_1","nodes":[
			{"type":"BRAND_NEW_NODE","config":{"anything":true}},
			{"type":"STEP","config":{}},
			{"type":"FLOW","config":{"flow_id":"af_1"}}]}`)

		form, err := tenant.ReadForm(ctx, "ap_1")

		require.NoError(t, err)
		assert.Equal(t, portals.FormDetail{ID: "ap_1", FlowID: "af_1"}, form)
	})

	t.Run("a form that does not exist is a 404", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodGet, "forms/ap_1", 404, `{"statusCode":404,"message":"Form not found"}`)

		_, err := tenant.ReadForm(ctx, "ap_1")

		assert.True(t, portals.IsStatus(err, http.StatusNotFound), "got %v", err)
	})

	imports := []struct {
		name       string
		response   string
		wantFlowID string
		wantErr    string
	}{
		{
			name:       "an import whose flows are an object keyed by placeholder",
			response:   `{"form":{"id":"ap_1"},"flows":{"#FLOW-1#":{"id":"af_1","name":"Flow"}}}`,
			wantFlowID: "af_1",
		},
		{
			name:       "an import whose flows are an array",
			response:   `{"form":{"id":"ap_1"},"flows":[{"id":"af_2"}]}`,
			wantFlowID: "af_2",
		},
		{
			name:       "an import without flows takes the flow from the form",
			response:   `{"form":{"id":"ap_1","nodes":[{"type":"FLOW","config":{"flow_id":"af_3"}}]}}`,
			wantFlowID: "af_3",
		},
		{
			name:     "an import that returns no form",
			response: `{"flows":[]}`,
			wantErr:  "the import did not return the ID of the form",
		},
	}

	for _, tt := range imports {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, tenant := newPortalsServer(t)
			server.reply(http.MethodPost, "forms/import", 200, tt.response)

			imported, err := tenant.ImportForm(ctx, map[string]any{"form": map[string]any{"name": "Profile"}})

			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, portals.ImportedForm{ID: "ap_1", FlowID: tt.wantFlowID}, imported)
			assert.JSONEq(t, `{"form":{"name":"Profile"}}`, server.received(http.MethodPost, "forms/import")[0].Body)
		})
	}
}

func TestPortalsTenant_Portals(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("a list reads an object and follows the pages", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.handle(http.MethodGet, "portals", func(r receivedRequest) (int, string) {
			if r.Query["page"][0] == "0" {
				return 200, `{"portals":` + portalsJSON(0, portalsPageSize) + `}`
			}

			return 200, `{"portals":` + portalsJSON(portalsPageSize, portalsPageSize+1) + `}`
		})

		listed, err := tenant.ListPortals(ctx)

		require.NoError(t, err)
		assert.Len(t, listed, portalsPageSize+1)
		assert.Equal(t, portals.PortalSummary{ID: "up_0", Slug: "portal-0", Name: "Portal 0"}, listed[0])
		assert.Equal(t, []string{"100"}, server.received(http.MethodGet, "portals")[0].Query["per_page"])
	})

	t.Run("a list reads an array", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodGet, "portals", 200, portalsJSON(0, 2))

		listed, err := tenant.ListPortals(ctx)

		require.NoError(t, err)
		assert.Len(t, listed, 2)
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

// An id comes from the command line, so a character that means something in a URL must stay inside
// its path segment and cannot change the route or add a query.
func TestPortalsTenant_AnIdStaysOneSegment(t *testing.T) {
	t.Parallel()

	const (
		id      = "up_x?y/../z"
		escaped = "up_x%3Fy%2F..%2Fz"
	)

	tests := []struct {
		name string
		path string
		call func(tenant *portalsTenant) error
	}{
		{
			name: "reading a portal",
			path: "portals/" + escaped,
			call: func(tenant *portalsTenant) error {
				_, err := tenant.ReadPortal(context.Background(), id)

				return err
			},
		},
		{
			name: "updating a portal",
			path: "portals/" + escaped,
			call: func(tenant *portalsTenant) error {
				return tenant.UpdatePortal(context.Background(), id, portals.PortalPatch{})
			},
		},
		{
			name: "reading a form",
			path: "forms/" + escaped,
			call: func(tenant *portalsTenant) error {
				_, err := tenant.ReadForm(context.Background(), id)

				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, tenant := newPortalsServer(t)
			server.handle(http.MethodGet, tt.path, func(receivedRequest) (int, string) { return 404, `{"statusCode":404}` })
			server.handle(http.MethodPatch, tt.path, func(receivedRequest) (int, string) { return 404, `{"statusCode":404}` })

			_ = tt.call(tenant)

			require.Len(t, server.requests, 1)
			assert.Equal(t, tt.path, server.requests[0].Path)
			assert.Empty(t, server.requests[0].Query, "the id adds no query")
		})
	}
}

func TestPortalsTenant_Probe(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("a read that works returns no error, and keeps the query", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodGet, "clients", 200, `{"clients":[]}`)

		require.NoError(t, tenant.Probe(ctx, http.MethodGet, "clients?per_page=1", nil))

		assert.Equal(t, []string{"1"}, server.received(http.MethodGet, "clients")[0].Query["per_page"])
	})

	t.Run("a write the API rejects is an error with its status", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodPost, "clients", 400, `{"statusCode":400,"message":"Payload validation error: 'Missing required property'","errorCode":"invalid_body"}`)

		err := tenant.Probe(ctx, http.MethodPost, "clients", map[string]any{})

		assert.True(t, portals.IsStatus(err, http.StatusBadRequest), "got %v", err)
		assert.JSONEq(t, `{}`, server.received(http.MethodPost, "clients")[0].Body)
	})

	t.Run("a scope the login lacks is a 403 that names it", func(t *testing.T) {
		t.Parallel()

		server, tenant := newPortalsServer(t)
		server.reply(http.MethodPost, "forms/import", 403, `{"statusCode":403,"message":"Insufficient scope, expected any of: create:forms","errorCode":"insufficient_scope"}`)

		err := tenant.Probe(ctx, http.MethodPost, "forms/import", map[string]any{})

		require.True(t, portals.IsStatus(err, http.StatusForbidden), "got %v", err)
		assert.Contains(t, err.Error(), "create:forms")
	})
}
