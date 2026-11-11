package cli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/portals"
	"github.com/auth0/auth0-cli/internal/portals/portalstest"
)

// stubTransport answers every request with a fixed body and remembers what reached it.
type stubTransport struct {
	mu       sync.Mutex
	received []string
}

func (s *stubTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	s.mu.Lock()
	s.received = append(s.received, request.Method+" "+request.URL.Path)
	s.mu.Unlock()

	return jsonResponse(request, map[string]any{"from": "the tenant"})
}

func (s *stubTransport) reached() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]string(nil), s.received...)
}

func send(t *testing.T, transport http.RoundTripper, method, path, body string) map[string]any {
	t.Helper()

	request, err := http.NewRequestWithContext(context.Background(), method, "https://acme.us.auth0.com/api/v2/"+path, strings.NewReader(body))
	require.NoError(t, err)

	response, err := transport.RoundTrip(request)
	require.NoError(t, err)

	defer func() { _ = response.Body.Close() }()

	data, err := io.ReadAll(response.Body)
	require.NoError(t, err)

	var answer map[string]any
	require.NoError(t, json.Unmarshal(data, &answer), string(data))

	return answer
}

func TestPortalsDryRun(t *testing.T) {
	t.Parallel()

	t.Run("a read goes to the tenant", func(t *testing.T) {
		t.Parallel()

		tenant := &stubTransport{}
		plan := newPortalsDryRun(tenant)

		answer := send(t, plan, http.MethodGet, "clients?per_page=1", "")

		assert.Equal(t, map[string]any{"from": "the tenant"}, answer)
		assert.Equal(t, []string{"GET /api/v2/clients"}, tenant.reached())
		assert.Empty(t, plan.planned())
	})

	t.Run("a write is kept and never sent", func(t *testing.T) {
		t.Parallel()

		tenant := &stubTransport{}
		plan := newPortalsDryRun(tenant)

		send(t, plan, http.MethodPost, "resource-servers", `{"name":"API","identifier":"https://acme/me/"}`)
		send(t, plan, http.MethodPatch, "client-grants/cgr_1", `{"scope":["a"]}`)
		send(t, plan, http.MethodDelete, "client-grants/cgr_2", "")

		assert.Empty(t, tenant.reached())
		assert.Equal(t, []plannedRequest{
			{Method: "POST", Path: "resource-servers", Body: map[string]any{"name": "API", "identifier": "https://acme/me/"}},
			{Method: "PATCH", Path: "client-grants/cgr_1", Body: map[string]any{"scope": []any{"a"}}},
			{Method: "DELETE", Path: "client-grants/cgr_2"},
		}, plan.planned())
	})

	t.Run("a create is answered with a placeholder ID, numbered in the order of the creates", func(t *testing.T) {
		t.Parallel()

		plan := newPortalsDryRun(&stubTransport{})

		first := send(t, plan, http.MethodPost, "resource-servers", `{"name":"API"}`)
		second := send(t, plan, http.MethodPost, "client-grants", `{"audience":"https://acme/me/"}`)
		update := send(t, plan, http.MethodPatch, "client-grants/x", `{"scope":["a"]}`)

		assert.Equal(t, map[string]any{"name": "API", "id": "<new:resource-servers:1>"}, first)
		assert.Equal(t, map[string]any{"audience": "https://acme/me/", "id": "<new:client-grants:2>"}, second)
		assert.Equal(t, map[string]any{}, update)
	})

	t.Run("a new application comes with a placeholder secret", func(t *testing.T) {
		t.Parallel()

		plan := newPortalsDryRun(&stubTransport{})

		answer := send(t, plan, http.MethodPost, "clients", `{"name":"Acme"}`)

		assert.Equal(t, map[string]any{"name": "Acme", "client_id": "<new:clients:1>", "client_secret": "<client secret>"}, answer)
	})

	t.Run("an imported form comes with a placeholder ID and no flow", func(t *testing.T) {
		t.Parallel()

		plan := newPortalsDryRun(&stubTransport{})

		answer := send(t, plan, http.MethodPost, "forms/import", `{"form":{"name":"Profile"}}`)

		assert.Equal(t, map[string]any{
			"form":  map[string]any{"id": "<new:forms:1>", "nodes": []any{}},
			"flows": []any{},
		}, answer)
	})

	t.Run("a read about a resource that does not exist yet finds nothing, and does not reach the tenant", func(t *testing.T) {
		t.Parallel()

		tenant := &stubTransport{}
		plan := newPortalsDryRun(tenant)
		created := send(t, plan, http.MethodPost, "clients", `{"name":"Acme"}`)

		path := "client-grants?client_id=" + url.QueryEscape(created["client_id"].(string))
		answer := send(t, plan, http.MethodGet, path, "")

		assert.Equal(t, map[string]any{}, answer)
		assert.Empty(t, tenant.reached())
	})

	t.Run("a secret never reaches the plan, wherever it is", func(t *testing.T) {
		t.Parallel()

		plan := newPortalsDryRun(&stubTransport{})

		send(t, plan, http.MethodPost, "portals", `{"slug":"acme","client":{"client_id":"cli_1","client_secret":"real-secret"},"list":[{"client_secret":"another"}]}`)

		encoded, err := json.Marshal(plan.planned())
		require.NoError(t, err)
		assert.NotContains(t, string(encoded), "real-secret")
		assert.NotContains(t, string(encoded), "another")

		body := plan.planned()[0].Body.(map[string]any)
		assert.Equal(t, "<client secret>", body["client"].(map[string]any)["client_secret"])
		assert.Equal(t, "<client secret>", body["list"].([]any)[0].(map[string]any)["client_secret"])
		assert.Equal(t, "cli_1", body["client"].(map[string]any)["client_id"], "the rest is kept")
	})

	t.Run("the answer to a create never repeats a secret that was sent", func(t *testing.T) {
		t.Parallel()

		plan := newPortalsDryRun(&stubTransport{})

		answer := send(t, plan, http.MethodPost, "portals", `{"slug":"acme","client":{"client_id":"cli_1","client_secret":"real-secret"}}`)

		encoded, err := json.Marshal(answer)
		require.NoError(t, err)
		assert.NotContains(t, string(encoded), "real-secret")
	})

	t.Run("the other credentials a request can carry are left out of the plan too", func(t *testing.T) {
		t.Parallel()

		plan := newPortalsDryRun(&stubTransport{})

		send(t, plan, http.MethodPost, "flows/vault/connections", `{"name":"v","setup":{"client_id":"cli_2","secret":"s-1","private_key":"k-1","password":"p-1","token":"t-1","access_token":"a-1","refresh_token":"r-1"}}`)

		encoded, err := json.Marshal(plan.planned())
		require.NoError(t, err)

		for _, secret := range []string{"s-1", "k-1", "p-1", "t-1", "a-1", "r-1"} {
			assert.NotContains(t, string(encoded), secret)
		}

		assert.Contains(t, string(encoded), "cli_2", "the rest is kept")
	})

	t.Run("the refresh token settings of an application are not a credential", func(t *testing.T) {
		t.Parallel()

		plan := newPortalsDryRun(&stubTransport{})

		answer := send(t, plan, http.MethodPost, "clients", `{"name":"Acme","refresh_token":{"rotation_type":"non-rotating"}}`)

		assert.Equal(t, map[string]any{"rotation_type": "non-rotating"}, answer["refresh_token"])
	})

	t.Run("the connections enabled in the plan are the ones the organization has", func(t *testing.T) {
		t.Parallel()

		plan := newPortalsDryRun(&stubTransport{})
		organization := send(t, plan, http.MethodPost, "organizations", `{"name":"acme"}`)["id"].(string)

		send(t, plan, http.MethodPost, "organizations/"+organization+"/enabled_connections", `{"connection_id":"con_db"}`)
		answer := send(t, plan, http.MethodGet, "organizations/"+organization+"/enabled_connections", "")

		assert.Equal(t, []any{map[string]any{"connection_id": "con_db"}}, answer["enabled_connections"])
	})
}

// newDryRunServer starts a local TLS server for a tenant with nothing in it, and returns the access to
// it. A dry run and a run of the checks of the permissions both go to this server.
func newDryRunServer(t *testing.T) (*portalsServer, *httptest.Server, apiAccess) {
	t.Helper()

	server := &portalsServer{t: t, routes: map[string]func(receivedRequest) (int, string){}}
	tls := httptest.NewTLSServer(http.HandlerFunc(server.serve))
	t.Cleanup(tls.Close)

	host := strings.TrimPrefix(tls.URL, "https://")

	empty := map[string]string{
		"resource-servers":        `{"resource_servers":[],"total":0}`,
		"clients":                 `{"clients":[],"total":0}`,
		"client-grants":           `{"client_grants":[]}`,
		"flows/vault/connections": `{"connections":[]}`,
		"forms":                   `{"forms":[],"total":0}`,
		"portals":                 `{"portals":[]}`,
	}
	for path, body := range empty {
		server.reply(http.MethodGet, path, 200, body)
	}

	server.reply(http.MethodGet, "custom-domains/default", 404, `{"statusCode":404,"message":"Not Found"}`)
	server.reply(http.MethodGet, "custom-domains", 200, `[]`)

	// The checks of the permissions send writes with an empty body, which the API rejects.
	for _, path := range []string{"resource-servers", "clients", "client-grants", "flows/vault/connections", "forms/import", "portals"} {
		server.reply(http.MethodPost, path, 400, `{"statusCode":400,"message":"Payload validation error"}`)
	}

	server.reply(http.MethodPatch, "clients/cli_preflight0000000", 404, `{"statusCode":404,"message":"The client does not exist"}`)
	server.reply(http.MethodPatch, "client-grants/cgr_preflight0000000", 404, `{"statusCode":404,"message":"The grant does not exist"}`)
	server.reply(http.MethodPatch, "flows/vault/connections/ac_preflight0000000000000", 404, `{"statusCode":404,"message":"Connection not found"}`)

	return server, tls, apiAccess{domain: host, accessToken: "token"}
}

// newTenantsOver returns the tenant of a dry run, the plan, and the tenant that sends the checks.
func newTenantsOver(t *testing.T, tls *httptest.Server, access apiAccess) (*portalsTenant, *portalsDryRun, *portalsTenant) {
	t.Helper()

	dry, plan, err := dryRunTenantOver(tls.Client().Transport, access)
	require.NoError(t, err)

	v1, err := initializeManagementClientWith(access.domain, access.accessToken, tls.Client())
	require.NoError(t, err)
	v3, err := initializeManagementClientV3With(access.domain, access.accessToken, tls.Client())
	require.NoError(t, err)

	return dry, plan, newPortalsTenant(auth0.NewAPI(v1), auth0.NewAPIV3(v3))
}

func dryRunDeps(dry portals.API, prober portals.Prober) (portals.SetupDeps, *portalstest.Reporter) {
	reporter := &portalstest.Reporter{}

	return portals.SetupDeps{
		API:          dry,
		Prober:       prober,
		Reporter:     reporter,
		Prompter:     &portalstest.Prompter{},
		TenantDomain: "acme.us.auth0.com",
	}, reporter
}

// The tests below build the clients the way the CLI does, and the SDK changes a map that all its
// clients share when one is built, so they do not run in parallel with each other.
func TestPortalsDryRun_SetupOfAConsumerPortal(t *testing.T) {
	server, tls, access := newDryRunServer(t)
	dry, plan, checker := newTenantsOver(t, tls, access)
	deps, _ := dryRunDeps(dry, checker)

	result, err := portals.Setup(context.Background(), consumerOptionsForDryRun(), deps)

	require.NoError(t, err)

	t.Run("it reports what a real run would create", func(t *testing.T) {
		assert.Equal(t, portals.StatusCreated, result.Portal.Status)
		assert.Equal(t, portals.StatusCreated, result.Application.Status)

		for _, resource := range result.Resources {
			assert.Equal(t, portals.StatusCreated, resource.Status, resource.Type)
		}
	})

	t.Run("it plans every write, in the order of a real run", func(t *testing.T) {
		var steps []string
		for _, request := range plan.planned() {
			steps = append(steps, request.Method+" "+request.Path)
		}

		assert.Equal(t, []string{
			"POST resource-servers",
			"POST clients",
			"POST client-grants",
			"POST clients",
			"POST client-grants",
			"POST flows/vault/connections",
			"POST forms/import",
			"POST forms/import",
			"POST forms/import",
			"POST portals",
		}, steps)
	})

	t.Run("the portal is planned with the placeholder of the application and its placeholder secret", func(t *testing.T) {
		portal := plan.planned()[len(plan.planned())-1]
		client := portal.Body.(map[string]any)["client"].(map[string]any)

		assert.Equal(t, "<new:clients:2>", client["client_id"])
		assert.Equal(t, "<client secret>", client["client_secret"])
		assert.Equal(t, "<new:clients:2>", result.Application.ClientID)
	})

	t.Run("nothing reached the tenant but the reads and the checks", func(t *testing.T) {
		// A write that was sent would have been a second request to the route, and the only one
		// the tenant knows is the check, which carries an empty body.
		for _, path := range []string{"resource-servers", "clients", "client-grants", "flows/vault/connections", "forms/import", "portals"} {
			posts := server.received(http.MethodPost, path)
			require.Len(t, posts, 1, "POST "+path)
			assert.JSONEq(t, `{}`, posts[0].Body, "POST "+path)
		}

		assert.Len(t, server.received(http.MethodGet, "client-grants"), 1, "only the check, because the application does not exist yet")
	})
}

func consumerOptionsForDryRun() portals.SetupOptions {
	return portals.SetupOptions{Template: portals.TemplateConsumer, Name: "Acme Account", Slug: "acme-account"}
}

func TestPortalsDryRun_ASecretReadFromTheTenantNeverReachesThePlan(t *testing.T) {
	server, tls, access := newDryRunServer(t)

	// The application of an earlier run exists, and its secret is read to plan the portal.
	server.reply(http.MethodGet, "clients", 200, `{"clients":[
		{"client_id":"cli_existing","name":"Acme Account (Universal Portals)","client_metadata":{"portals_setup_slug":"acme-account"},
		 "session_transfer":{"allowed_authentication_methods":["query"]}}],"start":0,"limit":100,"total":1}`)
	server.reply(http.MethodGet, "clients/cli_existing", 200, `{"client_id":"cli_existing","client_secret":"real-secret-xyz"}`)

	dry, plan, checker := newTenantsOver(t, tls, access)
	deps, reporter := dryRunDeps(dry, checker)

	result, err := portals.Setup(context.Background(), consumerOptionsForDryRun(), deps)

	require.NoError(t, err)
	assert.Equal(t, portals.StatusReused, result.Application.Status)

	encoded, err := json.Marshal(plan.planned())
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "real-secret-xyz", "the plan never carries the real secret")

	for _, line := range reporter.Lines() {
		assert.NotContains(t, line, "real-secret-xyz")
	}

	var steps []string
	for _, request := range plan.planned() {
		steps = append(steps, request.Method+" "+request.Path)
	}

	assert.Contains(t, steps, "PATCH clients/cli_existing", "the settings of the application are brought back")

	portal := plan.planned()[len(plan.planned())-1]
	assert.Equal(t, "<client secret>", portal.Body.(map[string]any)["client"].(map[string]any)["client_secret"])
}

func TestReportPortalsPlan(t *testing.T) {
	t.Parallel()

	result := portals.SetupResult{Template: portals.TemplateConsumer, Portal: portals.PortalResult{ID: "<new:portals:9>", Status: portals.StatusCreated}}
	planned := []plannedRequest{{Method: "POST", Path: "portals", Body: map[string]any{"slug": "acme"}}}

	t.Run("a person gets a summary and no JSON", func(t *testing.T) {
		t.Parallel()

		run := newSetupRun(false)
		terminal := newPortalsTerminal(&run.messages, plainPortalsStyle(), false, false)

		reportPortalsPlan(run.cli, terminal, result, planned)

		assert.Contains(t, run.messages.String(), "Dry run complete. 1 request(s) would change the tenant.")
		assert.Contains(t, run.messages.String(), "Run the same command without --dry-run to set up the portal.")
		assert.Empty(t, run.results.String())
	})

	t.Run("JSON is the result and the plan", func(t *testing.T) {
		t.Parallel()

		run := newSetupRun(true)
		terminal := newPortalsTerminal(&run.messages, plainPortalsStyle(), true, false)

		reportPortalsPlan(run.cli, terminal, result, planned)

		var printed map[string]any
		require.NoError(t, json.Unmarshal(run.results.Bytes(), &printed))
		assert.Equal(t, true, printed["dryRun"])
		assert.Equal(t, "consumer", printed["template"])
		assert.Equal(t, []any{map[string]any{"method": "POST", "path": "portals", "body": map[string]any{"slug": "acme"}}}, printed["requests"])
		assert.Empty(t, run.messages.String())
	})

	t.Run("a plan with no request is an empty list, not null", func(t *testing.T) {
		t.Parallel()

		run := newSetupRun(true)
		terminal := newPortalsTerminal(&run.messages, plainPortalsStyle(), true, false)

		reportPortalsPlan(run.cli, terminal, result, nil)

		assert.Contains(t, run.results.String(), `"requests": []`)
	})
}

// The business template is the one that creates an organization, enables a connection for it and adds
// a member. The organization the plan makes does not exist, so the reads that follow its creation are
// answered from the plan.
func TestPortalsDryRun_SetupOfABusinessPortalThatCreatesAnOrganization(t *testing.T) {
	server, tls, access := newDryRunServer(t)

	server.reply(http.MethodGet, "users", 200, `{"start":0,"limit":50,"length":1,"total":1,"users":[{"user_id":"auth0|a","email":"jane@acme.com","name":"jane"}]}`)
	server.reply(http.MethodGet, "users/auth0%7Ca/organizations", 200, `{"organizations":[],"total":0}`)
	server.reply(http.MethodGet, "organizations", 200, `{"organizations":[],"total":0}`)
	server.reply(http.MethodGet, "roles", 200, `{"roles":[],"total":0}`)
	server.reply(http.MethodGet, "connections", 200, `{"connections":[{"id":"con_db","name":"Username-Password-Authentication","strategy":"auth0"}]}`)

	// The checks of the permissions that only a business portal needs.
	server.reply(http.MethodPost, "organizations", 400, `{"statusCode":400,"message":"Payload validation error"}`)
	server.reply(http.MethodPost, "roles", 400, `{"statusCode":400,"message":"Payload validation error"}`)

	for _, path := range []string{
		"organizations/org_preflight0000000/enabled_connections",
		"organizations/org_preflight0000000/members",
		"organizations/org_preflight0000000/members/auth0%7Cpreflight0000000000/roles",
		"roles/rol_preflight0000000/permissions",
	} {
		server.reply(http.MethodPost, path, 404, `{"statusCode":404,"message":"Not found"}`)
	}

	// The reads of the organization of a member and of its connections, which the checks send too.
	for _, path := range []string{
		"organizations/org_preflight0000000/enabled_connections",
		"organizations/org_preflight0000000/members/auth0%7Cpreflight0000000000/roles",
	} {
		server.reply(http.MethodGet, path, 404, `{"statusCode":404,"message":"Not found"}`)
	}

	dry, plan, checker := newTenantsOver(t, tls, access)
	deps, _ := dryRunDeps(dry, checker)
	deps.Interactive = true

	// The user belongs to no organization, so setup offers to create one, and then a role.
	asks := []string{"Acme Admins", "acme-admins"}
	confirms := []bool{true, false}

	deps.Prompter = &portalstest.Prompter{
		AskFunc: func(question, _, _ string) (string, error) {
			if len(asks) == 0 {
				return "", errors.New("unexpected question: " + question)
			}

			answer := asks[0]
			asks = asks[1:]

			return answer, nil
		},
		ChooseFunc: func(question string, options []portals.Choice) (int, error) {
			for index, option := range options {
				if strings.Contains(option.Label, "None of these") {
					return index, nil
				}
			}

			return 0, errors.New("unexpected choice: " + question)
		},
		ConfirmFunc: func(question string, _ bool) (bool, error) {
			if len(confirms) == 0 {
				return false, errors.New("unexpected confirmation: " + question)
			}

			answer := confirms[0]
			confirms = confirms[1:]

			return answer, nil
		},
	}

	result, err := portals.Setup(context.Background(), portals.SetupOptions{
		Template: portals.TemplateBusiness, Name: "Acme Admin", Slug: "acme-admin", UserQuery: "jane@acme.com",
	}, deps)

	require.NoError(t, err)
	assert.Equal(t, "<new:organizations:1>", result.Organization.ID)

	var steps []string
	for _, request := range plan.planned() {
		steps = append(steps, request.Method+" "+request.Path)
	}

	assert.Equal(t, []string{
		"POST organizations",
		"POST organizations/<new:organizations:1>/enabled_connections",
		"POST organizations/<new:organizations:1>/members",
		"POST resource-servers",
		"POST resource-servers",
		"POST clients",
		"POST client-grants",
		"POST client-grants",
		"POST clients",
		"POST client-grants",
		"POST flows/vault/connections",
		"POST forms/import",
		"POST forms/import",
		"POST forms/import",
		"POST portals",
	}, steps)

	// A write that was sent would be a second request to the route. The one the tenant got is the check
	// of the permissions, which carries an empty body.
	for _, path := range []string{"organizations", "roles"} {
		posts := server.received(http.MethodPost, path)
		require.Len(t, posts, 1, "POST "+path)
		assert.JSONEq(t, `{}`, posts[0].Body, "POST "+path)
	}
}
