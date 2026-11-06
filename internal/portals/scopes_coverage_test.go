package portals_test

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/portals"
	"github.com/auth0/auth0-cli/internal/portals/portalstest"
)

// scopeOf is the scope the Management API asks for a request: the verb comes from the HTTP method and
// the resource is the first segment of the path.
func scopeOf(call string) string {
	method, path, _ := strings.Cut(call, " ")

	verbs := map[string]string{
		http.MethodGet:    "read",
		http.MethodPost:   "create",
		http.MethodPatch:  "update",
		http.MethodPut:    "update",
		http.MethodDelete: "delete",
	}

	resource, _, _ := strings.Cut(path, "/")

	return verbs[method] + ":" + strings.ReplaceAll(resource, "-", "_")
}

// Every request that setup, its checks, the audit and its fixes make needs a scope. If the list the
// advice asks for misses one, a person who followed the advice would still be refused.
func TestRequiredScopes_CoverEveryRequest(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	required := portals.RequiredScopes()

	t.Run("a consumer portal, its checks, and an audit that fixes it", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		seedDatabase(tenant)
		result := runSetup(t, tenant, consumerOptions())
		tenant.EnableClientConnection(result.Application.ClientID, "con_db")

		for _, template := range []portals.Template{portals.TemplateConsumer, portals.TemplateBusiness} {
			_, err := portals.RunPreflight(ctx, tenant, portals.SetupProbes(template), tenantDomain)
			require.NoError(t, err)
		}

		// Break the portal in the ways the fixes repair.
		tenant.ModifyPortal(result.Portal.ID, func(p *portals.Portal) {
			p.Pages.Default = ""
			p.Navigation.Sidebar.Components = []portals.SidebarComponent{legacyLink("profile")}
		})
		tenant.ModifyClient(result.Application.ClientID, func(client *portals.Client) {
			client.RefreshToken.RotationType = "rotating"
			client.RefreshToken.Policies = nil
			client.GrantTypes = append(client.GrantTypes, "client_credentials")
			client.Callbacks = nil
		})
		tenant.AddGrant(portals.ClientGrant{ID: "cgr_mgmt", ClientID: result.Application.ClientID, Audience: "https://acme.us.auth0.com/api/v2/"})

		_, err := portals.Audit(ctx, portals.AuditOptions{PortalID: result.Portal.ID, Fix: true}, portals.AuditDeps{
			API: tenant, Reporter: &portalstest.Reporter{}, Prompter: &portalstest.Prompter{}, TenantDomain: tenantDomain,
		})
		require.NoError(t, err)

		assertScopesCover(t, required, tenant.Calls())
	})

	t.Run("a business portal for a user who gets an organization and a role", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		seedDatabase(tenant)
		seedUser(tenant, "auth0|a", "jane@acme.com")
		deps, _, _ := interactiveDeps(tenant, answers{
			asks:     []string{"Acme Admins", "acme-admins"},
			chooses:  []string{"None of these"},
			confirms: []bool{true, true},
		})

		_, err := portals.Setup(ctx, businessOptions("jane@acme.com"), deps)
		require.NoError(t, err)

		assertScopesCover(t, required, tenant.Calls())
		assert.Contains(t, tenant.Calls(), "POST roles", "the role of the portal was created")
	})
}

func assertScopesCover(t *testing.T, required, calls []string) {
	t.Helper()

	var needed []string

	for _, call := range calls {
		if scope := scopeOf(call); !slices.Contains(needed, scope) {
			needed = append(needed, scope)
		}
	}

	slices.Sort(needed)

	var missing []string

	for _, scope := range needed {
		if !slices.Contains(required, scope) {
			missing = append(missing, scope)
		}
	}

	assert.Empty(t, missing, "add these scopes to the scopes the advice asks for")
}
