package portals_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/portals"
)

func TestDashboardURLs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		tenant string
		got    func(tenant string) string
		want   string
	}{
		{
			name:   "an application opens its settings, on the host of the region",
			tenant: "acme.eu.auth0.com",
			got:    func(tenant string) string { return portals.ApplicationURL(tenant, "abc123") },
			want:   "https://manage.auth0.com/dashboard/eu/acme/applications/abc123/settings",
		},
		{
			name:   "a tenant without a region code is treated as us",
			tenant: "dev-example.auth0.com",
			got:    func(tenant string) string { return portals.ApplicationURL(tenant, "abc123") },
			want:   "https://manage.auth0.com/dashboard/us/dev-example/applications/abc123/settings",
		},
		{
			name:   "a tenant in another cloud keeps its service domain",
			tenant: "acme.us.example-cloud.com",
			got:    func(tenant string) string { return portals.ApplicationURL(tenant, "abc123") },
			want:   "https://manage.example-cloud.com/dashboard/us/acme/applications/abc123/settings",
		},
		{
			name:   "a portal is edited in the dashboard",
			tenant: "acme.us.auth0.com",
			got:    func(tenant string) string { return portals.PortalEditURL(tenant, "up_1") },
			want:   "https://manage.auth0.com/dashboard/us/acme/universal-portals/up_1/edit",
		},
		{
			name:   "a form is edited in the Forms app",
			tenant: "acme.us.auth0.com",
			got:    func(tenant string) string { return portals.FormEditURL(tenant, "ap_1") },
			want:   "https://forms.auth0.com/tenants/us/acme/forms/ap_1/edit",
		},
		{
			name:   "a flow is edited in the Forms app",
			tenant: "acme.us.auth0.com",
			got:    func(tenant string) string { return portals.FlowEditURL(tenant, "af_1") },
			want:   "https://forms.auth0.com/tenants/us/acme/flows/af_1/edit",
		},
		{
			name:   "the vault opens the list of the AUTH0 app type",
			tenant: "acme.us.auth0.com",
			got:    portals.VaultAppsURL,
			want:   "https://forms.auth0.com/tenants/us/acme/vault/apps/AUTH0/edit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.got(tt.tenant))
		})
	}
}
