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

func probeNames(probes []portals.Probe) []string {
	names := make([]string, 0, len(probes))
	for _, probe := range probes {
		names = append(names, probe.Name)
	}

	return names
}

func TestSetupProbes(t *testing.T) {
	t.Parallel()

	consumer := probeNames(portals.SetupProbes(portals.TemplateConsumer))
	business := probeNames(portals.SetupProbes(portals.TemplateBusiness))

	t.Run("the consumer template checks what a rerun updates too", func(t *testing.T) {
		t.Parallel()

		assert.Contains(t, consumer, "update client")
		assert.Contains(t, consumer, "update client grant")
		assert.Contains(t, consumer, "update vault connection")
		assert.NotContains(t, consumer, "create organization")
	})

	t.Run("the business template also checks what the test user needs", func(t *testing.T) {
		t.Parallel()

		assert.Subset(t, business, consumer)
		assert.Subset(t, business, []string{
			"look up users by email", "list organizations", "create organization", "enable organization connection",
			"add organization member", "assign a role to a member", "create role", "add role permissions",
		})
	})

	t.Run("a probe never changes anything in the tenant", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()

		result, err := portals.RunPreflight(context.Background(), tenant, portals.SetupProbes(portals.TemplateBusiness), "acme.us.auth0.com")

		require.NoError(t, err)
		assert.Empty(t, result.Gaps)
		assert.Empty(t, result.Unexpected, "every probe is answered the way the API answers it")
		assert.Empty(t, tenant.Clients())
		assert.Empty(t, tenant.Portals())
		assert.Empty(t, tenant.Organizations())
		assert.Empty(t, tenant.Roles())
	})

	t.Run("a scope that is denied is found by the probe that needs it", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		tenant.Deny("PATCH client-grants/cgr_preflight0000000", "Insufficient scope, expected any of: update:client_grants")

		result, err := portals.RunPreflight(context.Background(), tenant, portals.SetupProbes(portals.TemplateConsumer), "acme.us.auth0.com")

		require.NoError(t, err)
		require.Len(t, result.Gaps, 1)
		assert.Equal(t, "update client grant", result.Gaps[0].Probe)
		assert.Equal(t, []string{"update:client_grants"}, result.RequiredScopes())
	})
}

// shapeOf is a request without its IDs, so a probe on a made-up resource and the real request on a
// resource that exists have the same shape.
func shapeOf(call string) string {
	static := []string{
		"resource-servers", "clients", "client-grants", "flows", "vault", "connections", "forms", "import",
		"portals", "custom-domains", "users", "organizations", "roles", "permissions", "members", "enabled_connections",
	}

	method, path, _ := strings.Cut(call, " ")
	segments := strings.Split(strings.SplitN(path, "?", 2)[0], "/")

	for index, segment := range segments {
		if !slices.Contains(static, segment) {
			segments[index] = "{id}"
		}
	}

	return method + " " + strings.Join(segments, "/")
}

// The check of the permissions exists so that setup does not stop half way for a permission it could
// have found at the start, so each write setup makes needs a probe of its own shape.
func TestSetupProbes_CoverEveryWriteOfSetup(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	tenant := portalstest.New()
	seedDatabase(tenant)
	seedUser(tenant, "auth0|a", "jane@acme.com")

	// A first run, and a rerun that updates the application it made.
	runSetup(t, tenant, consumerOptions())
	runSetup(t, tenant, consumerOptions())

	// A business run that creates an organization and a role, and gives the user the role.
	deps, _, _ := interactiveDeps(tenant, answers{
		asks:     []string{"Acme Admins", "acme-admins"},
		chooses:  []string{"None of these"},
		confirms: []bool{true, true},
	})
	_, err := portals.Setup(ctx, businessOptions("jane@acme.com"), deps)
	require.NoError(t, err)

	var probed []string
	for _, probe := range portals.SetupProbes(portals.TemplateBusiness) {
		probed = append(probed, shapeOf(probe.Method+" "+probe.Path))
	}

	var unprobed []string

	for _, call := range tenant.Calls() {
		if strings.HasPrefix(call, http.MethodGet+" ") || strings.Contains(call, "preflight") {
			continue
		}

		if shape := shapeOf(call); !slices.Contains(probed, shape) && !slices.Contains(unprobed, shape) {
			unprobed = append(unprobed, shape)
		}
	}

	assert.Empty(t, unprobed, "add a probe for each of these writes")
}

func TestTemplate_IsValid(t *testing.T) {
	t.Parallel()

	assert.True(t, portals.TemplateConsumer.IsValid())
	assert.True(t, portals.TemplateBusiness.IsValid())
	assert.False(t, portals.Template("").IsValid())
	assert.False(t, portals.Template("enterprise").IsValid())
}
