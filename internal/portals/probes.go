package portals

import "net/http"

// Probe is one request that checks a permission setup needs, without changing anything. A read
// that works, a write the API rejects with a 400, and, for a probe on a resource that does not
// exist, a 404, all mean the scope is present. A 403 names the scope that is missing.
type Probe struct {
	Name   string
	Method string
	Path   string
	Body   any
	// BusinessOnly marks a probe that only the business template needs.
	BusinessOnly bool
	// FakeTarget marks a probe on a resource that does not exist, so a 404 also means the scope is present.
	FakeTarget bool
}

// The IDs of the probes on a single resource are shaped like real ones, and nothing has them, so a
// probe can never change anything.
const (
	fakeClientID       = "cli_preflight0000000"
	fakeGrantID        = "cgr_preflight0000000"
	fakeVaultID        = "ac_preflight0000000000000"
	fakeOrganizationID = "org_preflight0000000"
	fakeRoleID         = "rol_preflight0000000"
	fakeUserID         = "auth0|preflight0000000000"
)

// SetupProbes returns the checks setup runs before it creates anything, for the template. The
// create probes send an empty body, which the API rejects with a 400 for the missing properties.
func SetupProbes(template Template) []Probe {
	probes := []Probe{
		{Name: "list resource servers", Method: http.MethodGet, Path: "resource-servers?per_page=1"},
		{Name: "create resource server", Method: http.MethodPost, Path: "resource-servers", Body: map[string]any{}},
		{Name: "list clients", Method: http.MethodGet, Path: "clients?per_page=1"},
		{Name: "create client", Method: http.MethodPost, Path: "clients", Body: map[string]any{}},
		{Name: "list client grants", Method: http.MethodGet, Path: "client-grants?per_page=1"},
		{Name: "create client grant", Method: http.MethodPost, Path: "client-grants", Body: map[string]any{}},
		// A rerun updates the application, the grants and the vault connection it finds.
		{Name: "update client", Method: http.MethodPatch, Path: "clients/" + fakeClientID, Body: map[string]any{}, FakeTarget: true},
		{Name: "update client grant", Method: http.MethodPatch, Path: "client-grants/" + fakeGrantID, Body: map[string]any{}, FakeTarget: true},
		{Name: "list vault connections", Method: http.MethodGet, Path: "flows/vault/connections"},
		{Name: "create vault connection", Method: http.MethodPost, Path: "flows/vault/connections", Body: map[string]any{}},
		{Name: "update vault connection", Method: http.MethodPatch, Path: "flows/vault/connections/" + fakeVaultID, Body: map[string]any{}, FakeTarget: true},
		{Name: "list forms", Method: http.MethodGet, Path: "forms?per_page=1"},
		{Name: "import forms", Method: http.MethodPost, Path: "forms/import", Body: map[string]any{}},
		{Name: "list portals", Method: http.MethodGet, Path: "portals"},
		{Name: "create portal", Method: http.MethodPost, Path: "portals", Body: map[string]any{}},
		{
			Name:         "look up users by email",
			Method:       http.MethodGet,
			Path:         "users?q=email%3A%22preflight%40example.invalid%22&search_engine=v3",
			BusinessOnly: true,
		},
		{Name: "list organizations", Method: http.MethodGet, Path: "organizations?per_page=1", BusinessOnly: true},
		// Business setup can create an organization and a role, then link the user and the connection.
		{Name: "create organization", Method: http.MethodPost, Path: "organizations", Body: map[string]any{}, BusinessOnly: true},
		{
			Name:         "enable organization connection",
			Method:       http.MethodPost,
			Path:         "organizations/" + fakeOrganizationID + "/enabled_connections",
			Body:         map[string]any{},
			BusinessOnly: true,
			FakeTarget:   true,
		},
		{
			Name:         "add organization member",
			Method:       http.MethodPost,
			Path:         "organizations/" + fakeOrganizationID + "/members",
			Body:         map[string]any{},
			BusinessOnly: true,
			FakeTarget:   true,
		},
		{
			Name:         "assign a role to a member",
			Method:       http.MethodPost,
			Path:         "organizations/" + fakeOrganizationID + "/members/" + fakeUserID + "/roles",
			Body:         map[string]any{},
			BusinessOnly: true,
			FakeTarget:   true,
		},
		{Name: "create role", Method: http.MethodPost, Path: "roles", Body: map[string]any{}, BusinessOnly: true},
		{
			Name:         "add role permissions",
			Method:       http.MethodPost,
			Path:         "roles/" + fakeRoleID + "/permissions",
			Body:         map[string]any{},
			BusinessOnly: true,
			FakeTarget:   true,
		},
	}

	selected := probes[:0:0]
	for _, probe := range probes {
		if !probe.BusinessOnly || template == TemplateBusiness {
			selected = append(selected, probe)
		}
	}

	return selected
}
