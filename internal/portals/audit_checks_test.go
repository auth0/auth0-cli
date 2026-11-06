package portals_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/portals"
)

const (
	me    = "https://acme.us.auth0.com/me/"
	myOrg = "https://acme.us.auth0.com/my-org/"
)

func sidebarLink(label, slug, icon string) portals.SidebarComponent {
	return portals.SidebarComponent{
		Type:   "sidebar:component:auth0:internal_link:v2",
		Config: map[string]any{"label": label, "page_slug": slug, "icon": icon},
	}
}

func section(children ...map[string]any) portals.Component {
	items := make([]any, 0, len(children))
	for _, child := range children {
		items = append(items, child)
	}

	return portals.Component{
		Type:   "page:component:auth0:structure:section",
		Config: map[string]any{"variant": "card", "children": items},
	}
}

func leaf(componentType string, config map[string]any) map[string]any {
	return map[string]any{"type": componentType, "config": config}
}

func intPointer(value int) *int { return &value }

func boolPointer(value bool) *bool { return &value }

// healthySnapshot is a healthy consumer portal, shaped like one created by setup.
func healthySnapshot() portals.AuditSnapshot {
	return portals.AuditSnapshot{
		TenantDomain: tenantDomain,
		Portal: portals.Portal{
			ID:     "up_1",
			Slug:   "acme",
			Client: &portals.PortalClient{ClientID: "cli_1"},
			Navigation: &portals.Navigation{Sidebar: &portals.Sidebar{Components: []portals.SidebarComponent{
				sidebarLink("Profile", "profile", "user"),
				sidebarLink("Security", "security", "shield-check"),
			}}},
			Pages: &portals.Pages{
				Default: "profile",
				Standard: []portals.Page{
					{Title: "Profile", Slug: "profile", Components: []portals.Component{
						section(leaf("page:component:auth0:form", map[string]any{"form_id": "ap_1", "completion_message": "Saved"})),
					}},
					{Title: "Security", Slug: "security", Components: []portals.Component{
						section(leaf("page:component:auth0:my_account:mfa_management", map[string]any{})),
					}},
				},
			},
		},
		Application: &portals.Client{ID: "cli_1", ClientSettings: portals.ClientSettings{
			GrantTypes:        []string{"authorization_code", "refresh_token"},
			Callbacks:         []string{"https://acme.us.auth0.com/portals/auth/callback"},
			OrganizationUsage: "allow",
			IPHeaderTrusted:   boolPointer(true),
			SessionTransfer:   &portals.SessionTransfer{AllowedAuthenticationMethods: []string{"query"}},
			RefreshToken: &portals.RefreshToken{
				RotationType:              "non-rotating",
				IdleTokenLifetime:         intPointer(3600),
				TokenLifetime:             intPointer(86400),
				InfiniteIdleTokenLifetime: boolPointer(false),
				Policies:                  []portals.RefreshTokenPolicy{{Audience: me, Scope: []string{"read:me:factors"}}},
			},
		}},
		Grants:                 []portals.ClientGrant{{Audience: me, Scope: []string{"read:me:factors"}}},
		ResourceServers:        []string{me},
		Forms:                  []portals.FormStatus{{ID: "ap_1", Exists: true}},
		ApplicationConnections: []portals.Connection{{ID: "con_db", Name: "Username-Password-Authentication"}},
	}
}

func idsOf(findings []portals.Finding) []string {
	ids := make([]string, 0, len(findings))
	for _, finding := range findings {
		ids = append(ids, finding.ID)
	}

	return ids
}

func findingNamed(t *testing.T, findings []portals.Finding, id string) portals.Finding {
	t.Helper()

	index := slices.IndexFunc(findings, func(finding portals.Finding) bool { return finding.ID == id })
	require.GreaterOrEqual(t, index, 0, "%s should be reported, got %v", id, idsOf(findings))

	return findings[index]
}

func TestAudit_AHealthyConsumerPortalHasNoFindings(t *testing.T) {
	t.Parallel()

	assert.Empty(t, portals.AuditFindings(healthySnapshot()))
}

// One row for each rule: a healthy snapshot, one change, and the finding that change must produce.
func TestAudit_Rules(t *testing.T) {
	t.Parallel()

	organizationSection := section(leaf("page:component:auth0:my_organization:details_edit", map[string]any{}))

	rules := []struct {
		id           string
		name         string
		breakIt      func(s *portals.AuditSnapshot)
		wantSeverity portals.Severity
		wantFixable  bool
	}{
		{
			id: "P001", name: "no home page",
			breakIt:      func(s *portals.AuditSnapshot) { s.Portal.Pages.Default = "" },
			wantSeverity: portals.SeverityError, wantFixable: true,
		},
		{
			id: "P001", name: "a home page that matches no page",
			breakIt:      func(s *portals.AuditSnapshot) { s.Portal.Pages.Default = "missing" },
			wantSeverity: portals.SeverityError, wantFixable: true,
		},
		{
			id: "P002", name: "the home page missing from the sidebar",
			breakIt:      func(s *portals.AuditSnapshot) { s.Portal.Navigation.Sidebar.Components = nil },
			wantSeverity: portals.SeverityWarning, wantFixable: true,
		},
		{
			id: "P003", name: "a sidebar link in the legacy shape",
			breakIt: func(s *portals.AuditSnapshot) {
				s.Portal.Navigation.Sidebar.Components = []portals.SidebarComponent{
					{Type: "sidebar:component:auth0:internal_link", Config: map[string]any{"label": "Profile", "to": "profile"}},
				}
			},
			wantSeverity: portals.SeverityError, wantFixable: true,
		},
		{
			id: "P004", name: "a sidebar link to a page that does not exist",
			breakIt: func(s *portals.AuditSnapshot) {
				s.Portal.Navigation.Sidebar.Components = append(s.Portal.Navigation.Sidebar.Components, sidebarLink("Billing", "billing", "x"))
			},
			wantSeverity: portals.SeverityError,
		},
		{
			id: "P005", name: "a page that is not in the sidebar",
			breakIt: func(s *portals.AuditSnapshot) {
				s.Portal.Pages.Standard = append(s.Portal.Pages.Standard, portals.Page{Title: "Billing", Slug: "billing", Components: []portals.Component{organizationSection}})
			},
			wantSeverity: portals.SeverityWarning, wantFixable: true,
		},
		{
			id: "P006", name: "a page with no components",
			breakIt: func(s *portals.AuditSnapshot) {
				s.Portal.Pages.Standard = append(s.Portal.Pages.Standard, portals.Page{Title: "Billing", Slug: "billing"})
				s.Portal.Navigation.Sidebar.Components = append(s.Portal.Navigation.Sidebar.Components, sidebarLink("Billing", "billing", "x"))
			},
			wantSeverity: portals.SeverityWarning,
		},
		{
			id: "P007", name: "a form of the portal that does not exist",
			breakIt:      func(s *portals.AuditSnapshot) { s.Forms = []portals.FormStatus{{ID: "ap_1", Exists: false}} },
			wantSeverity: portals.SeverityError,
		},
		{
			id: "P008", name: "custom code without a custom domain",
			breakIt:      func(s *portals.AuditSnapshot) { s.Portal.CustomCode = &portals.CustomCode{Head: "<script></script>"} },
			wantSeverity: portals.SeverityWarning,
		},
		{
			id: "P009", name: "a portal with no application",
			breakIt: func(s *portals.AuditSnapshot) {
				s.Portal.Client = nil
				s.Application = nil
			},
			wantSeverity: portals.SeverityError, wantFixable: true,
		},
		{
			id: "P010", name: "client_credentials on the application",
			breakIt: func(s *portals.AuditSnapshot) {
				s.Application.GrantTypes = append(s.Application.GrantTypes, "client_credentials")
			},
			wantSeverity: portals.SeverityError, wantFixable: true,
		},
		{
			id: "P011", name: "a Management API grant on the application",
			breakIt: func(s *portals.AuditSnapshot) {
				s.Grants = append(s.Grants, portals.ClientGrant{ID: "cgr_mgmt", Audience: "https://acme.us.auth0.com/api/v2/", Scope: []string{"read:users"}})
			},
			wantSeverity: portals.SeverityError, wantFixable: true,
		},
		{
			id: "P011", name: "a Management API grant whose ID is not known has no automatic fix",
			breakIt: func(s *portals.AuditSnapshot) {
				s.Grants = append(s.Grants, portals.ClientGrant{Audience: "https://acme.us.auth0.com/api/v2/"})
			},
			wantSeverity: portals.SeverityError,
		},
		{
			id: "P012", name: "a missing callback URL",
			breakIt:      func(s *portals.AuditSnapshot) { s.Application.Callbacks = nil },
			wantSeverity: portals.SeverityError, wantFixable: true,
		},
		{
			id: "P013", name: "a rotating refresh token",
			breakIt:      func(s *portals.AuditSnapshot) { s.Application.RefreshToken.RotationType = "rotating" },
			wantSeverity: portals.SeverityError, wantFixable: true,
		},
		{
			id: "P014", name: "a refresh token with no idle expiry",
			breakIt: func(s *portals.AuditSnapshot) {
				s.Application.RefreshToken.InfiniteIdleTokenLifetime = boolPointer(true)
			},
			wantSeverity: portals.SeverityError, wantFixable: true,
		},
		{
			id: "P015", name: "refresh token lifetimes that differ from the guide",
			breakIt:      func(s *portals.AuditSnapshot) { s.Application.RefreshToken.IdleTokenLifetime = intPointer(7200) },
			wantSeverity: portals.SeverityWarning, wantFixable: true,
		},
		{
			id: "P016", name: "organization components without organization usage",
			breakIt: func(s *portals.AuditSnapshot) {
				s.Portal.Pages.Standard[0].Components = append(s.Portal.Pages.Standard[0].Components, organizationSection)
				s.Application.OrganizationUsage = "deny"
				s.ResourceServers = append(s.ResourceServers, myOrg)
				s.Grants = append(s.Grants, portals.ClientGrant{Audience: myOrg})
				s.Application.RefreshToken.Policies = append(s.Application.RefreshToken.Policies, portals.RefreshTokenPolicy{Audience: myOrg})
			},
			wantSeverity: portals.SeverityError, wantFixable: true,
		},
		{
			id: "P017", name: "an untrusted token endpoint IP header",
			breakIt:      func(s *portals.AuditSnapshot) { s.Application.IPHeaderTrusted = boolPointer(false) },
			wantSeverity: portals.SeverityWarning, wantFixable: true,
		},
		{
			id: "P018", name: "a used API with no resource server",
			breakIt:      func(s *portals.AuditSnapshot) { s.ResourceServers = nil },
			wantSeverity: portals.SeverityError, wantFixable: true,
		},
		{
			id: "P019", name: "a used API with no client grant",
			breakIt:      func(s *portals.AuditSnapshot) { s.Grants = nil },
			wantSeverity: portals.SeverityError, wantFixable: true,
		},
		{
			id: "P020", name: "a used API with no refresh token policy",
			breakIt:      func(s *portals.AuditSnapshot) { s.Application.RefreshToken.Policies = nil },
			wantSeverity: portals.SeverityError, wantFixable: true,
		},
		{
			id: "P021", name: "no connection for the application",
			breakIt:      func(s *portals.AuditSnapshot) { s.ApplicationConnections = nil },
			wantSeverity: portals.SeverityError,
		},
		{
			id: "P022", name: "an organization with no connection in common with the application",
			breakIt: func(s *portals.AuditSnapshot) {
				s.Organization = &portals.SnapshotOrganization{ID: "org_1", Name: "acme", Connections: []string{"con_other"}}
			},
			wantSeverity: portals.SeverityError,
		},
		{
			id: "P023", name: "DPoP on the application",
			breakIt:      func(s *portals.AuditSnapshot) { s.Application.RequireProofOfPossession = boolPointer(true) },
			wantSeverity: portals.SeverityError, wantFixable: true,
		},
		{
			id: "P024", name: "session transfer not enabled",
			breakIt: func(s *portals.AuditSnapshot) {
				s.Application.SessionTransfer = &portals.SessionTransfer{AllowedAuthenticationMethods: []string{}}
			},
			wantSeverity: portals.SeverityWarning,
		},
	}

	for _, rule := range rules {
		t.Run(rule.id+": "+rule.name, func(t *testing.T) {
			t.Parallel()

			current := healthySnapshot()
			rule.breakIt(&current)

			finding := findingNamed(t, portals.AuditFindings(current), rule.id)

			assert.Equal(t, rule.wantSeverity, finding.Severity)
			assert.Equal(t, rule.wantFixable, finding.CanFix(), "whether the CLI can fix it")
			assert.NotEmpty(t, finding.Check)
			assert.NotEmpty(t, finding.Message)
			require.NotNil(t, finding.Remedy)
			assert.NotEmpty(t, finding.Remedy.Summary)
		})
	}
}

func TestAudit_TheRulesThatAreNotBrokenStayQuiet(t *testing.T) {
	t.Parallel()

	t.Run("an organization that shares a connection with the application passes", func(t *testing.T) {
		t.Parallel()

		current := healthySnapshot()
		current.Organization = &portals.SnapshotOrganization{ID: "org_1", Name: "acme", Connections: []string{"con_db"}}

		assert.Empty(t, portals.AuditFindings(current))
	})

	t.Run("a custom domain that is ready makes the callback the one on that domain", func(t *testing.T) {
		t.Parallel()

		current := healthySnapshot()
		current.CustomDomain = &portals.CustomDomain{Domain: "auth.acme.com", Status: "ready"}

		finding := findingNamed(t, portals.AuditFindings(current), "P012")
		assert.Contains(t, finding.Message, "https://auth.acme.com/portals/auth/callback")
	})

	t.Run("a custom domain that is ready but has no name is not where the portal is served", func(t *testing.T) {
		t.Parallel()

		current := healthySnapshot()
		current.CustomDomain = &portals.CustomDomain{Status: "ready"}

		// Setup uses the tenant domain for it, so the callback of a portal that setup made is right.
		assert.NotContains(t, idsOf(portals.AuditFindings(current)), "P012")
	})

	t.Run("custom code without a custom domain says the tenant has none", func(t *testing.T) {
		t.Parallel()

		current := healthySnapshot()
		current.Portal.CustomCode = &portals.CustomCode{Head: "<script></script>"}

		finding := findingNamed(t, portals.AuditFindings(current), "P008")
		assert.Contains(t, finding.Message, "the tenant has no custom domain")
	})

	t.Run("custom code is fine once the tenant has a custom domain", func(t *testing.T) {
		t.Parallel()

		current := healthySnapshot()
		current.Portal.CustomCode = &portals.CustomCode{Head: "<script></script>"}
		current.CustomDomain = &portals.CustomDomain{Domain: "auth.acme.com", Status: "pending_verification"}
		current.Application.Callbacks = []string{"https://acme.us.auth0.com/portals/auth/callback"}

		assert.NotContains(t, idsOf(portals.AuditFindings(current)), "P008")
	})

	t.Run("without an application the checks that need it do not run", func(t *testing.T) {
		t.Parallel()

		current := healthySnapshot()
		current.Application = nil
		current.Portal.Client = nil
		current.Grants = nil
		current.ApplicationConnections = nil

		assert.Equal(t, []string{"P009"}, idsOf(portals.AuditFindings(current)))
	})

	t.Run("a rotating token with no idle lifetime set at all is only a P013 and P015", func(t *testing.T) {
		t.Parallel()

		current := healthySnapshot()
		current.Application.RefreshToken = &portals.RefreshToken{RotationType: "rotating", Policies: current.Application.RefreshToken.Policies}

		assert.Equal(t, []string{"P013", "P015"}, idsOf(portals.AuditFindings(current)))
	})

	t.Run("an idle lifetime that never ends is P014 and not also P015", func(t *testing.T) {
		t.Parallel()

		current := healthySnapshot()
		current.Application.RefreshToken.InfiniteIdleTokenLifetime = boolPointer(true)
		current.Application.RefreshToken.IdleTokenLifetime = nil

		assert.Equal(t, []string{"P014"}, idsOf(portals.AuditFindings(current)))
	})

	t.Run("the home page missing from the sidebar is not reported when the home page does not exist", func(t *testing.T) {
		t.Parallel()

		current := healthySnapshot()
		current.Portal.Pages.Default = "missing"
		current.Portal.Navigation.Sidebar.Components = nil

		assert.NotContains(t, idsOf(portals.AuditFindings(current)), "P002")
	})

	t.Run("findings that share an ID are told apart by their subject", func(t *testing.T) {
		t.Parallel()

		current := healthySnapshot()
		current.Portal.Pages.Standard = append(current.Portal.Pages.Standard,
			portals.Page{Slug: "terms", Title: "Terms"}, portals.Page{Slug: "faq", Title: "FAQ"})

		var empty []portals.Finding

		for _, finding := range portals.AuditFindings(current) {
			if finding.ID == "P006" {
				empty = append(empty, finding)
			}
		}

		require.Len(t, empty, 2)
		assert.NotEqual(t, empty[0].Key(), empty[1].Key())
	})
}

func TestAudit_ManagementAPIGrantsAreOneFinding(t *testing.T) {
	t.Parallel()

	current := healthySnapshot()
	current.Grants = append(current.Grants,
		portals.ClientGrant{ID: "cgr_1", Audience: "https://acme.us.auth0.com/api/v2/"},
		portals.ClientGrant{ID: "cgr_2", Audience: "https://acme.us.auth0.com/api/v2/"},
	)

	var found int

	for _, finding := range portals.AuditFindings(current) {
		if finding.ID == "P011" {
			found++
		}
	}

	assert.Equal(t, 1, found)
}

// Turning client_credentials off would leave the application with no grant at all, and an update
// cannot send an empty list, so the fix would report success and change nothing.
func TestAudit_ClientCredentialsAsTheOnlyGrantIsFixedByHand(t *testing.T) {
	t.Parallel()

	current := healthySnapshot()
	current.Application.GrantTypes = []string{"client_credentials"}

	finding := findingNamed(t, portals.AuditFindings(current), "P010")

	assert.False(t, finding.CanFix(), "the CLI has no grant to leave the application with")
	require.NotNil(t, finding.Remedy)
	assert.Equal(t,
		`auth0 api patch "clients/cli_1" --data '{"grant_types":["authorization_code","refresh_token"]}'`,
		finding.Remedy.Command,
	)
}

func TestAudit_FindingsAreJSON(t *testing.T) {
	t.Parallel()

	current := healthySnapshot()
	current.Application.GrantTypes = append(current.Application.GrantTypes, "client_credentials")
	current.ApplicationConnections = nil

	findings := portals.AuditFindings(current)

	assert.Equal(t, []string{"P010", "P021"}, idsOf(findings))
	assert.JSONEq(t, `[
		{"id":"P010","severity":"error","check":"client credentials",
		 "message":"The portal application has the client_credentials grant, which the setup guide says must be off.",
		 "remedy":{"summary":"Turn off the client_credentials grant"}},
		{"id":"P021","severity":"error","check":"connections",
		 "message":"No connection is enabled for the application, so nobody can sign in.",
		 "remedy":{"summary":"Enable a connection for the application, or promote one to the tenant level",
		  "command":"auth0 api patch \"connections/<connection-id>/clients\" --data '[{\"client_id\":\"cli_1\",\"status\":true}]'"}}
	]`, mustJSON(t, findings))
}
