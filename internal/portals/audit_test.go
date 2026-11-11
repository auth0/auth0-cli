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

	t.Run("reads that fail together are all reported", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.FailNext("GET resource-servers", http.StatusInternalServerError)
		portal.tenant.FailNext("GET custom-domains/default", http.StatusInternalServerError)

		_, err := portals.Audit(context.Background(), portals.AuditOptions{PortalID: portal.portalID}, portals.AuditDeps{
			API: portal.tenant, Reporter: &portalstest.Reporter{}, Prompter: &portalstest.Prompter{}, TenantDomain: tenantDomain,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "resource servers")
		assert.Contains(t, err.Error(), "custom domain")
	})
}

func TestAudit_ThePortalIsNamedByItsIDOrItsSlug(t *testing.T) {
	t.Parallel()

	audit := func(portal auditedPortal, name string) (portals.AuditOutcome, error) {
		return portals.Audit(context.Background(), portals.AuditOptions{PortalID: name}, portals.AuditDeps{
			API: portal.tenant, Reporter: &portalstest.Reporter{}, Prompter: &portalstest.Prompter{}, TenantDomain: tenantDomain,
		})
	}

	t.Run("a slug finds the portal, and the result has its ID", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)

		outcome, err := audit(portal, "acme-account")

		require.NoError(t, err)
		assert.Equal(t, portal.portalID, outcome.Result.PortalID)
	})

	t.Run("a slug that no portal has says so", func(t *testing.T) {
		t.Parallel()

		_, err := audit(newAuditedPortal(t), "no-such-portal")

		var usage *portals.UsageError
		require.ErrorAs(t, err, &usage)
		assert.Contains(t, err.Error(), `No portal has the slug "no-such-portal"`)
	})

	t.Run("something that is neither an ID nor a slug is told before any request", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		before := len(portal.tenant.Calls())

		_, err := audit(portal, "Not A Portal!")

		var usage *portals.UsageError
		require.ErrorAs(t, err, &usage)
		assert.Contains(t, err.Error(), `"Not A Portal!" is neither a portal ID`)
		assert.Len(t, portal.tenant.Calls(), before, "the API would answer a malformed ID with a 400")
	})

	t.Run("an ID is read as it is", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		before := len(portal.tenant.Calls())

		_, err := audit(portal, "up_missing")

		assert.True(t, portals.IsStatus(err, http.StatusNotFound), "got %v", err)
		assert.NotContains(t, portal.tenant.Calls()[before:], "GET portals", "the list of portals is not needed for an ID")
	})
}

func TestAudit_WhatTheSessionCannotReadIsNotChecked(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		scope string
		check string
		// What breakIt does is make the portal wrong in the way the skipped rule would report, and return
		// the requests the session is refused.
		breakIt func(t *testing.T, portal auditedPortal) []string
		skipped string
	}{
		{
			name: "the custom domain", scope: "read:custom_domains", check: "custom domain", skipped: "P012",
			breakIt: func(_ *testing.T, portal auditedPortal) []string {
				portal.tenant.ModifyClient(portal.clientID, func(client *portals.Client) { client.Callbacks = nil })

				return []string{"GET custom-domains/default"}
			},
		},
		{
			name: "the resource servers", scope: "read:resource_servers", check: "resource servers", skipped: "P018",
			breakIt: func(_ *testing.T, portal auditedPortal) []string {
				for _, server := range portal.tenant.ResourceServers() {
					portal.tenant.DeleteResourceServer(server.ID)
				}

				return []string{"GET resource-servers"}
			},
		},
		{
			name: "the grants of the application", scope: "read:client_grants", check: "client grants", skipped: "P011",
			breakIt: func(_ *testing.T, portal auditedPortal) []string {
				portal.tenant.AddGrant(portals.ClientGrant{ID: "cgr_mgmt", ClientID: portal.clientID, Audience: "https://acme.us.auth0.com/api/v2/"})

				return []string{"GET client-grants"}
			},
		},
		{
			name: "the forms", scope: "read:forms", check: "forms", skipped: "P007",
			breakIt: func(_ *testing.T, portal auditedPortal) []string {
				var refused []string

				for _, form := range portal.tenant.Forms() {
					portal.tenant.DeleteForm(form.ID)

					refused = append(refused, "GET forms/"+form.ID)
				}

				return refused
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name+" is reported as not checked, and the rest of the audit runs", func(t *testing.T) {
			t.Parallel()

			portal := newAuditedPortal(t)
			for _, request := range test.breakIt(t, portal) {
				portal.tenant.Deny(request, "Insufficient scope, expected any of: "+test.scope)
			}

			// A finding the rules that were skipped would have made, and one they do not touch.
			portal.tenant.ModifyClient(portal.clientID, func(client *portals.Client) { client.RequireProofOfPossession = boolPointer(true) })

			outcome, reporter := portal.audit(t, portals.AuditOptions{Strict: true})

			ids := idsOf(outcome.Result.Findings)
			assert.NotContains(t, ids, test.skipped, "a rule that needs what it could not read does not run")
			assert.Contains(t, ids, "P023", "the rules that do not need it still run")

			require.Len(t, outcome.Result.Unchecked, 1)
			assert.Equal(t, test.check, outcome.Result.Unchecked[0].Check)
			assert.Contains(t, outcome.Result.Unchecked[0].Rules, test.skipped)
			assert.Contains(t, outcome.Result.Unchecked[0].Reason, test.scope)
			assert.True(t, reporter.Contains("warn", "Not checked"))
		})
	}

	t.Run("a read that is not refused for a scope still stops the audit", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.FailNext("GET custom-domains/default", http.StatusInternalServerError)

		_, err := portals.Audit(context.Background(), portals.AuditOptions{PortalID: portal.portalID}, portals.AuditDeps{
			API: portal.tenant, Reporter: &portalstest.Reporter{}, Prompter: &portalstest.Prompter{}, TenantDomain: tenantDomain,
		})

		require.Error(t, err)
	})

	t.Run("what could not be read does not fail the audit, even a strict one", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.Deny("GET custom-domains/default", "Insufficient scope, expected any of: read:custom_domains")

		outcome, _ := portal.audit(t, portals.AuditOptions{Strict: true})

		assert.False(t, outcome.Failed())
		assert.Len(t, outcome.Result.Unchecked, 1)
	})

	t.Run("a portal with nothing unread has no unchecked field in its JSON", func(t *testing.T) {
		t.Parallel()

		outcome, _ := newAuditedPortal(t).audit(t, portals.AuditOptions{})

		assert.NotContains(t, mustJSON(t, outcome.Result), "unchecked")
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
