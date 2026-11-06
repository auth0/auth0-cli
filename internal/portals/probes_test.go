package portals_test

import (
	"context"
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

		assert.Len(t, consumer, 14)
		assert.Contains(t, consumer, "update client grant")
		assert.Contains(t, consumer, "update vault connection")
		assert.NotContains(t, consumer, "create organization")
	})

	t.Run("the business template also checks what the test user needs", func(t *testing.T) {
		t.Parallel()

		assert.Len(t, business, 21)
		assert.Subset(t, business, consumer)
		assert.Subset(t, business, []string{
			"look up users by email", "list organizations", "create organization", "enable organization connection",
			"add organization member", "create role", "add role permissions",
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

func TestTemplate_IsValid(t *testing.T) {
	t.Parallel()

	assert.True(t, portals.TemplateConsumer.IsValid())
	assert.True(t, portals.TemplateBusiness.IsValid())
	assert.False(t, portals.Template("").IsValid())
	assert.False(t, portals.Template("enterprise").IsValid())
}
