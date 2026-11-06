package portals_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/portals"
	"github.com/auth0/auth0-cli/internal/portals/portalstest"
)

// auditedPortal is a consumer portal made by setup, with a connection enabled for its application, so
// each test breaks exactly one thing.
type auditedPortal struct {
	tenant   *portalstest.Tenant
	portalID string
	clientID string
}

func newAuditedPortal(t *testing.T) auditedPortal {
	t.Helper()

	tenant := portalstest.New()
	seedDatabase(tenant)

	result := runSetup(t, tenant, consumerOptions())
	tenant.EnableClientConnection(result.Application.ClientID, "con_db")

	return auditedPortal{tenant: tenant, portalID: result.Portal.ID, clientID: result.Application.ClientID}
}

func (p auditedPortal) audit(t *testing.T, options portals.AuditOptions) (portals.AuditOutcome, *portalstest.Reporter) {
	t.Helper()

	options.PortalID = p.portalID

	reporter := &portalstest.Reporter{}
	outcome, err := portals.Audit(context.Background(), options, portals.AuditDeps{
		API:          p.tenant,
		Reporter:     reporter,
		Prompter:     &portalstest.Prompter{},
		TenantDomain: tenantDomain,
	})
	require.NoError(t, err)

	return outcome, reporter
}

func (p auditedPortal) findings(t *testing.T) []portals.Finding {
	t.Helper()

	findings, err := portals.FindingsOf(context.Background(), p.tenant, tenantDomain, p.portalID)
	require.NoError(t, err)

	return findings
}

// legacyLink is a sidebar link in the shape the portals used before the :v2 types.
func legacyLink(to string) portals.SidebarComponent {
	return portals.SidebarComponent{
		Type:   "sidebar:component:auth0:internal_link",
		Config: map[string]any{"label": "Profile", "to": to, "icon": "user"},
	}
}

func (p auditedPortal) setSidebar(components ...portals.SidebarComponent) {
	p.tenant.ModifyPortal(p.portalID, func(portal *portals.Portal) { portal.Navigation.Sidebar.Components = components })
}

// withLegacyProfileLink gives the sidebar of the portal a link in the legacy shape for the first page,
// one that still names the page, so it is the only thing wrong with the sidebar.
func (p auditedPortal) withLegacyProfileLink() {
	legacy := legacyLink("profile")
	legacy.Config["page_slug"] = "profile"

	p.setSidebar(
		legacy,
		sidebarLink("Security", "security", "shield-check"),
		sidebarLink("Legal & privacy", "legal-privacy", "file-text"),
	)
}

func TestAudit_APortalMadeBySetup(t *testing.T) {
	t.Parallel()

	t.Run("a consumer portal passes", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)

		outcome, _ := portal.audit(t, portals.AuditOptions{})

		assert.Empty(t, outcome.Result.Findings)
		assert.False(t, outcome.Failed())
	})

	t.Run("a business portal passes", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		seedDatabase(tenant)
		seedUser(tenant, "auth0|jane", "jane@acme.com")
		seedOrganization(tenant, "org_1", "acme", "auth0|jane")
		seedAdminRole(tenant)
		tenant.AssignRole("org_1", "auth0|jane", "rol_admin")

		result := runSetup(t, tenant, businessOptions("auth0|jane"))
		tenant.EnableClientConnection(result.Application.ClientID, "con_db")
		portal := auditedPortal{tenant: tenant, portalID: result.Portal.ID, clientID: result.Application.ClientID}

		outcome, _ := portal.audit(t, portals.AuditOptions{Organization: "org_1"})

		assert.Empty(t, outcome.Result.Findings)
	})
}

func TestAudit_WhatItReports(t *testing.T) {
	t.Parallel()

	t.Run("a deleted form is an error that fails the audit", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.DeleteForm(portal.tenant.Forms()[0].ID)

		outcome, _ := portal.audit(t, portals.AuditOptions{})

		assert.Contains(t, idsOf(outcome.Result.Findings), "P007")
		assert.True(t, outcome.Failed())
	})

	t.Run("a form ID the API cannot match is a missing form, not a crash", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.ModifyPortal(portal.portalID, func(p *portals.Portal) {
			p.Pages.Standard[0].Components[0].Config["children"].([]any)[0].(map[string]any)["config"].(map[string]any)["form_id"] = "ap_not-a-real-form-id"
		})

		outcome, _ := portal.audit(t, portals.AuditOptions{})

		assert.Contains(t, idsOf(outcome.Result.Findings), "P007")
	})

	t.Run("a deleted application is P009, not a crash", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.DeleteClient(portal.clientID)

		outcome, _ := portal.audit(t, portals.AuditOptions{})

		assert.Equal(t, []string{"P009"}, idsOf(outcome.Result.Findings))
		assert.True(t, outcome.Failed())
	})

	t.Run("a warning fails the audit only in a strict audit", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.ModifyPortal(portal.portalID, func(p *portals.Portal) { p.CustomCode = &portals.CustomCode{Head: "<script></script>"} })

		lenient, _ := portal.audit(t, portals.AuditOptions{})
		strict, _ := portal.audit(t, portals.AuditOptions{Strict: true})

		assert.Equal(t, 1, lenient.Result.Warnings)
		assert.False(t, lenient.Failed())
		assert.True(t, strict.Failed())
	})

	t.Run("an organization with no connection in common with the application is P022", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		seedDatabase(tenant)
		seedUser(tenant, "auth0|jane", "jane@acme.com")
		seedOrganization(tenant, "org_1", "acme", "auth0|jane")
		seedAdminRole(tenant)
		tenant.AssignRole("org_1", "auth0|jane", "rol_admin")
		result := runSetup(t, tenant, businessOptions("auth0|jane"))
		tenant.EnableClientConnection(result.Application.ClientID, "con_other")
		tenant.AddConnection(portals.Connection{ID: "con_other", Name: "Other"}, "google-oauth2")
		portal := auditedPortal{tenant: tenant, portalID: result.Portal.ID, clientID: result.Application.ClientID}

		outcome, _ := portal.audit(t, portals.AuditOptions{Organization: "org_1"})

		assert.Equal(t, []string{"P022"}, idsOf(outcome.Result.Findings))
	})

	t.Run("a portal that does not exist is an error", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.portalID = "up_missing"

		_, err := portals.Audit(context.Background(), portals.AuditOptions{PortalID: "up_missing"}, portals.AuditDeps{
			API: portal.tenant, Reporter: &portalstest.Reporter{}, Prompter: &portalstest.Prompter{}, TenantDomain: tenantDomain,
		})

		assert.True(t, portals.IsStatus(err, http.StatusNotFound), "got %v", err)
	})

	t.Run("a refused read says which scope it needs", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.Deny("GET portals/"+portal.portalID, "Insufficient scope, expected any of: read:portals")

		_, err := portals.Audit(context.Background(), portals.AuditOptions{PortalID: portal.portalID}, portals.AuditDeps{
			API: portal.tenant, Reporter: &portalstest.Reporter{}, Prompter: &portalstest.Prompter{}, TenantDomain: tenantDomain,
		})

		require.Error(t, err)
		assert.Equal(t, portals.ScopeFixes([]string{"read:portals"}), portals.FixesOf(err))
	})
}

func TestAudit_WhatItShows(t *testing.T) {
	t.Parallel()

	t.Run("a healthy portal", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)

		_, reporter := portal.audit(t, portals.AuditOptions{})

		assert.Equal(t, []string{
			"banner: Universal Portals audit | acme.us.auth0.com",
			"task: Reading the portal",
			`section: Portal "acme-account"`,
			"item: No problems found | ",
			"newline: ",
			"item: 0 errors, 0 warnings | ",
		}, reporter.Lines())
	})

	t.Run("a table of what it found, the commands for what it cannot fix, and a summary", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.withLegacyProfileLink()
		portal.tenant.ModifyClient(portal.clientID, func(client *portals.Client) { client.IPHeaderTrusted = boolPointer(false) })
		portal.tenant.ModifyClient(portal.clientID, func(client *portals.Client) {
			client.SessionTransfer = &portals.SessionTransfer{}
		})
		portal.tenant.DeleteGrant(portal.tenant.Grants()[0].ID)

		_, reporter := portal.audit(t, portals.AuditOptions{})

		lines := reporter.Lines()
		assert.Contains(t, lines, "table: ,ID,Check,Fix")
		assert.Contains(t, lines, "row: ✖,P003,navigation shape,Convert the sidebar links to the :v2 link types")
		assert.Contains(t, lines, "row: ▲,P017,token endpoint IP header,Trust the token endpoint IP header")
		assert.Contains(t, lines, "row: ▲,P024,session transfer,Allow session transfer on the portal application")
		assert.True(t, reporter.Contains("detail", "P024 manually: auth0 api patch"))
		assert.True(t, reporter.Contains("error", "2 errors, 2 warnings"), "%v", lines)
	})

	t.Run("one error and no warning", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.withLegacyProfileLink()

		_, reporter := portal.audit(t, portals.AuditOptions{})

		assert.True(t, reporter.Contains("error", "1 error, 0 warnings"))
	})

	t.Run("only warnings", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.ModifyClient(portal.clientID, func(client *portals.Client) { client.IPHeaderTrusted = boolPointer(false) })

		_, reporter := portal.audit(t, portals.AuditOptions{})

		assert.True(t, reporter.Contains("warn", "0 errors, 1 warning"))
	})
}
