package portals_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/portals"
	"github.com/auth0/auth0-cli/internal/portals/portalstest"
)

// fixOnly applies the fix of the finding with the ID, which must be reported and fixable.
func (p auditedPortal) fixOnly(t *testing.T, id string) portals.FixSummary {
	t.Helper()

	finding := findingNamed(t, p.findings(t), id)
	require.True(t, finding.CanFix(), "%s should have an automatic fix", id)

	summary, err := portals.FixFindings(context.Background(), p.tenant, &portalstest.Reporter{}, tenantDomain, p.portalID, []portals.Finding{finding})
	require.NoError(t, err)

	return summary
}

func (p auditedPortal) portal(t *testing.T) portals.Portal {
	t.Helper()

	portal, err := p.tenant.ReadPortal(context.Background(), p.portalID)
	require.NoError(t, err)

	return portal
}

func (p auditedPortal) client(t *testing.T, id string) portals.Client {
	t.Helper()

	client, err := p.tenant.ReadClient(context.Background(), id)
	require.NoError(t, err)

	return client
}

func TestFix_Remedies(t *testing.T) {
	t.Parallel()

	t.Run("P003: converting a legacy sidebar link leaves only :v2 links", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.setSidebar(legacyLink("profile"))

		portal.fixOnly(t, "P003")

		links := portal.portal(t).Navigation.Sidebar.Components
		assert.Equal(t, []portals.SidebarComponent{sidebarLink("Profile", "profile", "user")}, links)
	})

	t.Run("P003: an external legacy link becomes an external :v2 link", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.setSidebar(
			legacyLink("profile"),
			portals.SidebarComponent{Type: "sidebar:component:auth0:external_link", Config: map[string]any{"label": "Help", "url": "https://help.acme.com", "icon": "life-buoy"}},
		)

		portal.fixOnly(t, "P003")

		assert.Equal(t, portals.SidebarComponent{
			Type:   "sidebar:component:auth0:external_link:v2",
			Config: map[string]any{"label": "Help", "href": "https://help.acme.com", "icon": "life-buoy"},
		}, portal.portal(t).Navigation.Sidebar.Components[1])
	})

	t.Run("P003: a legacy link without an icon gets no icon, because the API rejects a null one", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.setSidebar(portals.SidebarComponent{
			Type:   "sidebar:component:auth0:internal_link",
			Config: map[string]any{"label": "Profile", "to": "profile"},
		})

		portal.fixOnly(t, "P003")

		assert.Equal(t, map[string]any{"label": "Profile", "page_slug": "profile"}, portal.portal(t).Navigation.Sidebar.Components[0].Config)
	})

	t.Run("P003: a legacy link keeps the settings it has besides the ones it renames", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		legacy := legacyLink("profile")
		legacy.Config["badge"] = "new"
		portal.setSidebar(legacy)

		portal.fixOnly(t, "P003")

		assert.Equal(t, map[string]any{"label": "Profile", "page_slug": "profile", "icon": "user", "badge": "new"}, portal.portal(t).Navigation.Sidebar.Components[0].Config)
	})

	t.Run("P003: the address of a legacy link becomes the slug of the page", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.setSidebar(legacyLink("/profile"))

		portal.fixOnly(t, "P003")

		assert.Equal(t, "profile", portal.portal(t).Navigation.Sidebar.Components[0].Config["page_slug"])
	})

	t.Run("P003: a link of another type is not converted, and does not count as legacy", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		other := portals.SidebarComponent{Type: "sidebar:component:auth0:section_header", Config: map[string]any{"label": "Account"}}
		portal.setSidebar(legacyLink("profile"), other)

		finding := findingNamed(t, portal.findings(t), "P003")
		assert.Contains(t, finding.Message, "1 sidebar link(s)")

		portal.fixOnly(t, "P003")

		assert.Equal(t, other, portal.portal(t).Navigation.Sidebar.Components[1])
	})

	t.Run("P003: the fields of the navigation the package does not know survive the conversion", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)

		var navigation portals.Navigation
		require.NoError(t, json.Unmarshal([]byte(`{"footer":{"text":"Acme"},"sidebar":{"collapsed":true,"components":[
			{"type":"sidebar:component:auth0:internal_link","config":{"label":"Profile","to":"profile","icon":"user"},"visible":false}]}}`), &navigation))
		portal.tenant.ModifyPortal(portal.portalID, func(p *portals.Portal) { p.Navigation = &navigation })

		portal.fixOnly(t, "P003")

		sent, err := json.Marshal(portal.portal(t).Navigation)
		require.NoError(t, err)
		assert.JSONEq(t, `{"footer":{"text":"Acme"},"sidebar":{"collapsed":true,"components":[
			{"type":"sidebar:component:auth0:internal_link:v2","config":{"label":"Profile","page_slug":"profile","icon":"user"},"visible":false}]}}`, string(sent))
	})

	t.Run("P001: setting the first page as the home page keeps the pages as they were", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		before := portal.portal(t).Pages.Standard
		portal.tenant.ModifyPortal(portal.portalID, func(p *portals.Portal) { p.Pages.Default = "" })

		portal.fixOnly(t, "P001")

		after := portal.portal(t).Pages
		assert.Equal(t, "profile", after.Default)
		assert.Equal(t, before, after.Standard)
	})

	t.Run("P011: removing the Management API grant deletes it from the application", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.AddGrant(portals.ClientGrant{ID: "cgr_mgmt", ClientID: portal.clientID, Audience: "https://acme.us.auth0.com/api/v2/", Scope: []string{"read:users"}})

		portal.fixOnly(t, "P011")

		for _, grant := range portal.tenant.Grants() {
			assert.NotEqual(t, "cgr_mgmt", grant.ID)
		}
	})

	t.Run("P010: turning off client credentials keeps the other grants", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.ModifyClient(portal.clientID, func(client *portals.Client) {
			client.GrantTypes = append(client.GrantTypes, "client_credentials")
		})

		portal.fixOnly(t, "P010")

		grants := portal.client(t, portal.clientID).GrantTypes
		assert.NotContains(t, grants, "client_credentials")
		assert.Contains(t, grants, "authorization_code")
		assert.Contains(t, grants, "refresh_token")
	})

	t.Run("P023: turning DPoP off on the application", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.ModifyClient(portal.clientID, func(client *portals.Client) { client.RequireProofOfPossession = boolPointer(true) })

		portal.fixOnly(t, "P023")

		assert.False(t, *portal.client(t, portal.clientID).RequireProofOfPossession)
	})

	t.Run("P012: adding the callback keeps the ones that were there", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.ModifyClient(portal.clientID, func(client *portals.Client) { client.Callbacks = []string{"https://other/callback"} })

		portal.fixOnly(t, "P012")

		assert.Equal(t,
			[]string{"https://other/callback", "https://acme.us.auth0.com/portals/auth/callback"},
			portal.client(t, portal.clientID).Callbacks,
		)
	})

	t.Run("P013: turning rotation off keeps the policies and the lifetimes", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.ModifyClient(portal.clientID, func(client *portals.Client) { client.RefreshToken.RotationType = "rotating" })

		portal.fixOnly(t, "P013")

		token := portal.client(t, portal.clientID).RefreshToken
		assert.Equal(t, "non-rotating", token.RotationType)
		assert.Equal(t, 3600, *token.IdleTokenLifetime)
		assert.Equal(t, 86400, *token.TokenLifetime)
		require.Len(t, token.Policies, 1)
		assert.Equal(t, me, token.Policies[0].Audience)
	})

	t.Run("P014: giving the refresh token an idle expiry", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.ModifyClient(portal.clientID, func(client *portals.Client) {
			client.RefreshToken.InfiniteIdleTokenLifetime = boolPointer(true)
			client.RefreshToken.IdleTokenLifetime = nil
		})

		portal.fixOnly(t, "P014")

		token := portal.client(t, portal.clientID).RefreshToken
		assert.False(t, *token.InfiniteIdleTokenLifetime)
		assert.Equal(t, 3600, *token.IdleTokenLifetime)
	})

	t.Run("P025: giving the refresh token a total lifetime keeps the idle one", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.ModifyClient(portal.clientID, func(client *portals.Client) {
			client.RefreshToken.InfiniteTokenLifetime = boolPointer(true)
			client.RefreshToken.TokenLifetime = nil
		})

		portal.fixOnly(t, "P025")

		token := portal.client(t, portal.clientID).RefreshToken
		assert.False(t, *token.InfiniteTokenLifetime)
		assert.Equal(t, 86400, *token.TokenLifetime)
		assert.Equal(t, 3600, *token.IdleTokenLifetime)
		assert.False(t, *token.InfiniteIdleTokenLifetime)
	})

	t.Run("P020: adding the policy of an API keeps the one it has", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.ModifyClient(portal.clientID, func(client *portals.Client) { client.RefreshToken.Policies = nil })

		portal.fixOnly(t, "P020")

		policies := portal.client(t, portal.clientID).RefreshToken.Policies
		require.Len(t, policies, 1)
		assert.Equal(t, me, policies[0].Audience)
		assert.Equal(t, portals.MyAccountScopes(), policies[0].Scope)
	})

	t.Run("P019: granting access to an API a component calls", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		for _, grant := range portal.tenant.Grants() {
			if grant.ClientID == portal.clientID && grant.Audience == me {
				portal.tenant.DeleteGrant(grant.ID)
			}
		}

		portal.fixOnly(t, "P019")

		found := false

		for _, grant := range portal.tenant.Grants() {
			if grant.ClientID == portal.clientID && grant.Audience == me {
				found = true

				assert.Equal(t, portals.MyAccountScopes(), grant.Scope)
			}
		}

		assert.True(t, found)
	})

	t.Run("P019: a grant cut to fewer scopes gets the missing ones back and keeps the others", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		for _, grant := range portal.tenant.Grants() {
			if grant.ClientID == portal.clientID && grant.Audience == me {
				require.NoError(t, portal.tenant.UpdateClientGrantScope(context.Background(), grant.ID, []string{"read:me:factors", "read:me:something_new"}))
			}
		}

		portal.fixOnly(t, "P019")

		for _, grant := range portal.tenant.Grants() {
			if grant.ClientID == portal.clientID && grant.Audience == me {
				assert.ElementsMatch(t, append(portals.MyAccountScopes(), "read:me:something_new"), grant.Scope)
			}
		}
	})

	t.Run("P020: a policy cut to fewer scopes gets the missing ones back", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.ModifyClient(portal.clientID, func(client *portals.Client) {
			client.RefreshToken.Policies[0].Scope = []string{"read:me:factors"}
		})

		portal.fixOnly(t, "P020")

		policies := portal.client(t, portal.clientID).RefreshToken.Policies
		require.Len(t, policies, 1)
		assert.ElementsMatch(t, portals.MyAccountScopes(), policies[0].Scope)
	})

	t.Run("P018: creating the resource server of an API a component calls", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.AddResourceServer(portals.ResourceServer{ID: "rs_x", Identifier: "https://elsewhere/"})

		for _, server := range portal.tenant.ResourceServers() {
			if server.Identifier == me {
				portal.tenant.DeleteResourceServer(server.ID)
			}
		}

		portal.fixOnly(t, "P018")

		identifiers := []string{}
		for _, server := range portal.tenant.ResourceServers() {
			identifiers = append(identifiers, server.Identifier)
		}

		assert.Contains(t, identifiers, me)
	})

	t.Run("P016: allowing organizations on the application", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.ModifyPortal(portal.portalID, func(p *portals.Portal) {
			p.Pages.Standard[1].Components = append(p.Pages.Standard[1].Components, section(leaf("page:component:auth0:my_organization:details_edit", map[string]any{})))
		})
		portal.tenant.ModifyClient(portal.clientID, func(client *portals.Client) { client.OrganizationUsage = "deny" })
		// The organization components also need their API, which is fixed on its own.
		require.Contains(t, idsOf(portal.findings(t)), "P016")

		portal.fixOnly(t, "P016")

		assert.Equal(t, "allow", portal.client(t, portal.clientID).OrganizationUsage)
	})
}

// patchRecorder records what is sent to link an application to a portal.
type patchRecorder struct {
	*portalstest.Tenant
	patches []portals.PortalPatch
}

func (r *patchRecorder) UpdatePortal(ctx context.Context, id string, patch portals.PortalPatch) error {
	r.patches = append(r.patches, patch)

	return r.Tenant.UpdatePortal(ctx, id, patch)
}

func TestFix_AMissingApplication(t *testing.T) {
	t.Parallel()

	t.Run("P009: the fix creates the application and links it, with the secret", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.ModifyPortal(portal.portalID, func(p *portals.Portal) { p.Client = nil })
		portal.tenant.DeleteClient(portal.clientID)

		recorder := &patchRecorder{Tenant: portal.tenant}
		finding := findingNamed(t, portal.findings(t), "P009")

		_, err := portals.FixFindings(context.Background(), recorder, &portalstest.Reporter{}, tenantDomain, portal.portalID, []portals.Finding{finding})
		require.NoError(t, err)

		require.Len(t, recorder.patches, 1)
		link := recorder.patches[0].Client
		require.NotNil(t, link)
		assert.NotEqual(t, portal.clientID, link.ClientID)
		assert.NotEmpty(t, link.ClientSecret, "the secret is sent with the link")
		assert.Equal(t, "client_secret_post", link.TokenEndpointAuthMethod)
		assert.Equal(t, link.ClientID, portal.portal(t).Client.ClientID, "the portal points at the new application")
		assert.Len(t, portal.tenant.Clients(), 2, "the new application and the forms vault")
	})

	t.Run("P009: a link that fails after the application was created does not create a second one on rerun", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.ModifyPortal(portal.portalID, func(p *portals.Portal) { p.Client = nil })
		portal.tenant.DeleteClient(portal.clientID)
		portal.tenant.FailNext("PATCH portals/"+portal.portalID, http.StatusInternalServerError)

		first := findingNamed(t, portal.findings(t), "P009")
		failed, err := portals.FixFindings(context.Background(), portal.tenant, &portalstest.Reporter{}, tenantDomain, portal.portalID, []portals.Finding{first})
		require.NoError(t, err)
		assert.Equal(t, 1, failed.Failed)
		assert.Nil(t, portal.portal(t).Client, "the first link failed")

		portal.fixOnly(t, "P009")

		assert.Len(t, portal.tenant.Clients(), 2, "one application in total, besides the forms vault")
		assert.NotNil(t, portal.portal(t).Client)
	})

	t.Run("P009: a portal with organization components gets the business defaults", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.ModifyPortal(portal.portalID, func(p *portals.Portal) {
			p.Client = nil
			p.Pages.Standard[1].Components = append(p.Pages.Standard[1].Components, section(leaf("page:component:auth0:my_organization:details_edit", map[string]any{})))
		})
		portal.tenant.DeleteClient(portal.clientID)

		portal.fixOnly(t, "P009")

		created := portal.client(t, portal.portal(t).Client.ClientID)
		audiences := []string{}
		for _, policy := range created.RefreshToken.Policies {
			audiences = append(audiences, policy.Audience)
		}

		assert.ElementsMatch(t, []string{me, myOrg}, audiences)
	})
}

func TestFix_Rounds(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("applying the errors repairs a home page, a missing refresh policy, and rotation", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.ModifyPortal(portal.portalID, func(p *portals.Portal) { p.Pages.Default = "" })
		portal.tenant.ModifyClient(portal.clientID, func(client *portals.Client) {
			client.RefreshToken.RotationType = "rotating"
			client.RefreshToken.Policies = nil
		})

		summary, err := portals.FixFindings(ctx, portal.tenant, &portalstest.Reporter{}, tenantDomain, portal.portalID, portals.FixableFindings(portal.findings(t)))
		require.NoError(t, err)

		assert.GreaterOrEqual(t, summary.Applied, 3)

		outcome, _ := portal.audit(t, portals.AuditOptions{})
		assert.Zero(t, outcome.Result.Errors, "%v", idsOf(outcome.Result.Findings))
	})

	t.Run("a fix that is already in place is skipped, not applied twice", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.ModifyPortal(portal.portalID, func(p *portals.Portal) { p.Pages.Default = "" })
		stale := findingNamed(t, portal.findings(t), "P001")
		// The home page is set by hand meanwhile.
		portal.tenant.ModifyPortal(portal.portalID, func(p *portals.Portal) { p.Pages.Default = "profile" })

		reporter := &portalstest.Reporter{}
		summary, err := portals.FixFindings(ctx, portal.tenant, reporter, tenantDomain, portal.portalID, []portals.Finding{stale})

		require.NoError(t, err)
		assert.Equal(t, 1, summary.AlreadyFixed)
		assert.Zero(t, summary.Applied)
		assert.True(t, reporter.Contains("item", "P001 home page | already fixed"))
	})

	t.Run("a fix still applies when the message of the finding changed after the audit", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.setSidebar(legacyLink("profile"))
		finding := findingNamed(t, portal.findings(t), "P003")

		// A second legacy link changes the count in the message, and not the problem.
		portal.setSidebar(legacyLink("profile"), legacyLink("account"))
		summary, err := portals.FixFindings(ctx, portal.tenant, &portalstest.Reporter{}, tenantDomain, portal.portalID, []portals.Finding{finding})

		require.NoError(t, err)
		assert.Equal(t, 1, summary.Applied)
		assert.Zero(t, summary.AlreadyFixed)
	})

	t.Run("warnings are reported but never offered for fixing", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.ModifyClient(portal.clientID, func(client *portals.Client) { client.IPHeaderTrusted = boolPointer(false) })

		findings := portal.findings(t)

		assert.Empty(t, portals.FixableFindings(findings))
		assert.Equal(t, portals.SeverityWarning, findingNamed(t, findings, "P017").Severity)
	})

	t.Run("the errors a fix exposes are fixed in the next round", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.ModifyPortal(portal.portalID, func(p *portals.Portal) { p.Client = nil })
		portal.tenant.DeleteClient(portal.clientID)

		var exposed []string

		summary, err := portals.FixFindingRounds(
			ctx, portal.tenant, &portalstest.Reporter{}, tenantDomain, portal.portalID,
			portals.FixableFindings(portal.findings(t)),
			func(next []portals.Finding) ([]portals.Finding, error) {
				exposed = append(exposed, idsOf(next)...)
				return next, nil
			},
		)

		require.NoError(t, err)
		assert.Equal(t, []string{"P019"}, exposed, "the new application has no grant")
		assert.Equal(t, 2, summary.Applied)
		assert.NotContains(t, idsOf(portal.findings(t)), "P019")
	})

	t.Run("the errors a fix exposes are left alone when the person declines them", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.ModifyPortal(portal.portalID, func(p *portals.Portal) { p.Client = nil })
		portal.tenant.DeleteClient(portal.clientID)

		_, err := portals.FixFindingRounds(
			ctx, portal.tenant, &portalstest.Reporter{}, tenantDomain, portal.portalID,
			portals.FixableFindings(portal.findings(t)),
			func([]portals.Finding) ([]portals.Finding, error) { return nil, nil },
		)

		require.NoError(t, err)
		assert.Contains(t, idsOf(portal.findings(t)), "P019", "the exposed grant error is still reported")
	})
}

func TestFix_AFixThatFails(t *testing.T) {
	t.Parallel()

	t.Run("fixes that fail for a missing scope are summed up once, with the login command", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.ModifyPortal(portal.portalID, func(p *portals.Portal) { p.Pages.Default = "" })
		portal.tenant.Deny("PATCH portals/"+portal.portalID, "Insufficient scope, expected any of: update:portals")
		reporter := &portalstest.Reporter{}

		summary, err := portals.FixFindingRounds(
			context.Background(), portal.tenant, reporter, tenantDomain, portal.portalID,
			portals.FixableFindings(portal.findings(t)),
			func([]portals.Finding) ([]portals.Finding, error) { return nil, nil },
		)

		require.NoError(t, err)
		assert.Positive(t, summary.Failed)
		assert.Equal(t, []string{"update:portals"}, summary.MissingScopes)
		assert.True(t, reporter.Contains("error", "fix(es) failed, missing: update:portals"))
		assert.True(t, reporter.Contains("detail", "Signed in as a user: auth0 login --scopes "))
		assert.True(t, reporter.Contains("detail", "update:portals"))
	})

	t.Run("a failure that is not a scope is shown with the fix it belongs to, and the rest continue", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.ModifyPortal(portal.portalID, func(p *portals.Portal) { p.Pages.Default = "" })
		portal.tenant.ModifyClient(portal.clientID, func(client *portals.Client) { client.RefreshToken.RotationType = "rotating" })
		portal.tenant.FailNext("PATCH portals/"+portal.portalID, http.StatusInternalServerError)
		reporter := &portalstest.Reporter{}

		summary, err := portals.FixFindings(
			context.Background(), portal.tenant, reporter, tenantDomain, portal.portalID,
			portals.FixableFindings(portal.findings(t)),
		)

		require.NoError(t, err)
		assert.Equal(t, 1, summary.Failed)
		assert.Equal(t, 1, summary.Applied, "the fix of the rotation still ran")
		assert.True(t, reporter.Contains("error", "P001 home page: "))
		assert.Empty(t, summary.MissingScopes)
	})

	t.Run("a fix that failed fails the audit, even when the audit after it sees no error", func(t *testing.T) {
		t.Parallel()

		outcome := portals.AuditOutcome{Result: portals.AuditResult{Failed: false}, FixesFailed: 1}

		assert.True(t, outcome.Failed())
		assert.False(t, portals.AuditOutcome{}.Failed())
	})
}

func TestAudit_Fixing(t *testing.T) {
	t.Parallel()

	t.Run("--fix applies every error without asking, and audits again", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.withLegacyProfileLink()

		outcome, reporter := portal.audit(t, portals.AuditOptions{Fix: true})

		assert.Empty(t, outcome.Result.Findings)
		assert.False(t, outcome.Failed())
		assert.True(t, reporter.Contains("section", "Fix"))
		assert.True(t, reporter.Contains("section", "Checking the portal again"))
		assert.True(t, reporter.Contains("fixed", "P003 navigation shape | Convert the sidebar links to the :v2 link types"))
	})

	t.Run("--fix with nothing to fix changes nothing", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		before := len(portal.tenant.Writes())

		_, reporter := portal.audit(t, portals.AuditOptions{Fix: true})

		assert.Len(t, portal.tenant.Writes(), before)
		assert.False(t, reporter.Contains("section", "Fix"))
	})

	t.Run("without --fix and without a terminal it says how to fix, and changes nothing", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.withLegacyProfileLink()

		outcome, reporter := portal.audit(t, portals.AuditOptions{})

		assert.True(t, outcome.Failed())
		assert.True(t, reporter.Contains("detail", "Run with --fix to apply the errors the CLI can fix."))
		assert.Equal(t, "sidebar:component:auth0:internal_link", portal.portal(t).Navigation.Sidebar.Components[0].Type)
	})

	t.Run("a fix that failed fails the run", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.withLegacyProfileLink()
		portal.tenant.FailNext("PATCH portals/"+portal.portalID, http.StatusInternalServerError)

		outcome, _ := portal.audit(t, portals.AuditOptions{Fix: true})

		assert.Equal(t, 1, outcome.FixesFailed)
		assert.True(t, outcome.Failed())
	})

	t.Run("the result says which fix was applied and which failed", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.withLegacyProfileLink()
		portal.tenant.ModifyClient(portal.clientID, func(client *portals.Client) { client.RequireProofOfPossession = boolPointer(true) })
		portal.tenant.FailNext("PATCH portals/"+portal.portalID, http.StatusInternalServerError)

		outcome, _ := portal.audit(t, portals.AuditOptions{Fix: true})

		require.Len(t, outcome.Result.Fixes, 2)
		assert.Equal(t, portals.FixRecord{ID: "P003", Check: "navigation shape", Status: "failed", Error: outcome.Result.Fixes[0].Error}, outcome.Result.Fixes[0])
		assert.Contains(t, outcome.Result.Fixes[0].Error, "convert the sidebar of portal")
		assert.Equal(t, portals.FixRecord{ID: "P023", Check: "DPoP", Status: "applied"}, outcome.Result.Fixes[1])
	})

	t.Run("a result without fixes has no fixes field in its JSON", func(t *testing.T) {
		t.Parallel()

		outcome, _ := newAuditedPortal(t).audit(t, portals.AuditOptions{Fix: true})

		assert.NotContains(t, mustJSON(t, outcome.Result), "fixes")
	})
}

// fixMenu answers the fix menu by position (0 fix all, 1 choose, 2 skip), and keeps the checked
// errors whose IDs are listed.
func fixMenu(position int, keep ...string) *portalstest.Prompter {
	return &portalstest.Prompter{
		ChooseFunc: func(string, []portals.Choice) (int, error) { return position, nil },
		ChecklistFunc: func(_ string, items []portals.ChecklistItem) ([]int, error) {
			var picked []int

			for index, item := range items {
				if len(keep) == 0 || containsPrefix(keep, item.Label) {
					picked = append(picked, index)
				}
			}

			return picked, nil
		},
	}
}

func containsPrefix(ids []string, label string) bool {
	for _, id := range ids {
		if strings.HasPrefix(label, id+" ") {
			return true
		}
	}

	return false
}

func (p auditedPortal) auditInTerminal(t *testing.T, prompter portals.Prompter) (portals.AuditOutcome, *portalstest.Reporter, error) {
	t.Helper()

	reporter := &portalstest.Reporter{}
	outcome, err := portals.Audit(context.Background(), portals.AuditOptions{PortalID: p.portalID}, portals.AuditDeps{
		API: p.tenant, Reporter: reporter, Prompter: prompter, Interactive: true, TenantDomain: tenantDomain,
	})

	return outcome, reporter, err
}

func TestAudit_TheFixMenu(t *testing.T) {
	t.Parallel()

	t.Run("fix all applies every error and the portal passes", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.withLegacyProfileLink()

		outcome, reporter, err := portal.auditInTerminal(t, fixMenu(0))

		require.NoError(t, err)
		assert.False(t, outcome.Failed())
		assert.True(t, reporter.Contains("detail", "Warnings are not changed. They may be intentional."))

		for _, link := range portal.portal(t).Navigation.Sidebar.Components {
			assert.True(t, strings.HasSuffix(link.Type, ":v2"))
		}
	})

	t.Run("choosing errors applies only the checked ones", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.withLegacyProfileLink()
		portal.tenant.ModifyPortal(portal.portalID, func(p *portals.Portal) { p.Client = nil })
		portal.tenant.DeleteClient(portal.clientID)
		clientsBefore := len(portal.tenant.Clients())

		outcome, _, err := portal.auditInTerminal(t, fixMenu(1, "P003"))

		require.NoError(t, err)
		assert.True(t, outcome.Failed(), "the unchecked application error is still reported")
		assert.Nil(t, portal.portal(t).Client, "the unchecked error was not fixed")
		assert.Len(t, portal.tenant.Clients(), clientsBefore, "no application was created")

		for _, link := range portal.portal(t).Navigation.Sidebar.Components {
			assert.True(t, strings.HasSuffix(link.Type, ":v2"))
		}
	})

	t.Run("the checklist offers every error, checked, with what the fix does", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.withLegacyProfileLink()
		portal.tenant.ModifyClient(portal.clientID, func(client *portals.Client) { client.RefreshToken.RotationType = "rotating" })

		var items []portals.ChecklistItem

		prompter := fixMenu(1)
		prompter.ChecklistFunc = func(_ string, offered []portals.ChecklistItem) ([]int, error) {
			items = offered
			return nil, nil
		}

		_, _, err := portal.auditInTerminal(t, prompter)

		require.NoError(t, err)
		assert.Equal(t, []portals.ChecklistItem{
			{Label: "P003 Convert the sidebar links to the :v2 link types", Checked: true},
			{Label: "P013 Turn refresh token rotation off", Checked: true},
		}, items)
	})

	t.Run("skipping leaves the portal as it is", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.withLegacyProfileLink()
		writes := len(portal.tenant.Writes())

		outcome, _, err := portal.auditInTerminal(t, fixMenu(2))

		require.NoError(t, err)
		assert.True(t, outcome.Failed(), "the error is still reported")
		assert.Len(t, portal.tenant.Writes(), writes)
	})

	t.Run("the menu is not offered when there is nothing to fix", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)

		_, _, err := portal.auditInTerminal(t, &portalstest.Prompter{})

		require.NoError(t, err, "a question would have failed the test")
	})

	t.Run("a cancel at the menu is passed on, and nothing was changed", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.withLegacyProfileLink()
		writes := len(portal.tenant.Writes())
		cancelling := &portalstest.Prompter{ChooseFunc: func(string, []portals.Choice) (int, error) { return 0, portals.ErrCancelled }}

		_, _, err := portal.auditInTerminal(t, cancelling)

		require.True(t, errors.Is(err, portals.ErrCancelled))
		assert.Len(t, portal.tenant.Writes(), writes)
	})

	t.Run("the errors a fix exposes are asked about again", func(t *testing.T) {
		t.Parallel()

		portal := newAuditedPortal(t)
		portal.tenant.ModifyPortal(portal.portalID, func(p *portals.Portal) { p.Client = nil })
		portal.tenant.DeleteClient(portal.clientID)

		menus := 0
		prompter := fixMenu(0)
		prompter.ChooseFunc = func(string, []portals.Choice) (int, error) {
			menus++
			return 0, nil
		}

		_, reporter, err := portal.auditInTerminal(t, prompter)

		require.NoError(t, err)
		assert.Equal(t, 2, menus, "once for the application, and once for its grant")
		assert.True(t, reporter.Contains("detail", "These errors appeared after the fixes above."))
	})
}
