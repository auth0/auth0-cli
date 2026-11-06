package portals

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// Severity tells how bad a finding is.
type Severity string

const (
	// SeverityError means the portal or a component is broken.
	SeverityError Severity = "error"
	// SeverityWarning means something is degraded or differs from the setup guide.
	SeverityWarning Severity = "warning"
)

// Remedy is how to fix a finding.
type Remedy struct {
	// Summary is what the fix does.
	Summary string `json:"summary"`
	// Command is the equivalent manual command.
	Command string `json:"command,omitempty"`

	// The change itself, set when the CLI can make it.
	apply func(ctx context.Context, api API) error
}

// Finding is something the audit found.
type Finding struct {
	ID       string   `json:"id"`
	Severity Severity `json:"severity"`
	Check    string   `json:"check"`
	Message  string   `json:"message"`
	// Subject is what the finding is about, such as a page, a form or an API. The checks that can repeat
	// an ID set it, so a fix matches its finding by ID and subject and never by the text of the message.
	Subject string  `json:"subject,omitempty"`
	Remedy  *Remedy `json:"remedy,omitempty"`
}

// canFix reports whether the CLI can fix the finding.
func (f Finding) canFix() bool {
	return f.Remedy != nil && f.Remedy.apply != nil
}

// key identifies a finding. The message can change when the portal changes (a count, a slug), so it
// never takes part in matching.
func (f Finding) key() string {
	return f.ID + ":" + f.Subject
}

// auditPortal checks a snapshot against what a portal made by setup looks like.
func auditPortal(current snapshot) []Finding {
	checker := &portalAudit{current: current}
	checker.pagesAndNavigation()
	checker.formsAndCustomCode()

	if !checker.application() {
		return checker.findings
	}

	checker.refreshToken()
	checker.organizationAndHeader()
	checker.apisTheComponentsCall()
	checker.connectionsAndSessionTransfer()

	return checker.findings
}

// portalAudit is one audit of a snapshot.
type portalAudit struct {
	current  snapshot
	findings []Finding
}

func (a *portalAudit) add(finding Finding) {
	a.findings = append(a.findings, finding)
}

func (a *portalAudit) portalID() string { return a.current.Portal.ID }

func (a *portalAudit) pageSlugs() []string {
	slugs := make([]string, 0, len(a.current.Portal.pages()))
	for _, page := range a.current.Portal.pages() {
		slugs = append(slugs, page.Slug)
	}

	return slugs
}

func (a *portalAudit) sidebarPages() []string {
	var pages []string

	for _, item := range a.current.Portal.sidebar() {
		slug, _ := item.Config["page_slug"].(string)
		pages = append(pages, slug)
	}

	return pages
}

func (a *portalAudit) usedComponents() []Component {
	return allComponents(a.current.Portal.pages())
}

func (a *portalAudit) uses(needle string) bool {
	return slices.ContainsFunc(a.usedComponents(), func(component Component) bool {
		return strings.Contains(component.Type, needle)
	})
}

func (a *portalAudit) pagesAndNavigation() {
	portal := a.current.Portal
	pages := portal.pages()
	pageSlugs := a.pageSlugs()
	sidebarPages := a.sidebarPages()
	home := portal.homePage()

	if home == "" || !slices.Contains(pageSlugs, home) {
		a.add(a.homePageFinding(home))
	} else if !slices.Contains(sidebarPages, home) {
		title := home
		for _, page := range pages {
			if page.Slug == home {
				title = page.Title
			}
		}

		a.add(Finding{
			ID:       "P002",
			Severity: SeverityWarning,
			Check:    "home page in navigation",
			Message:  fmt.Sprintf("The home page %q is not in the sidebar, so users cannot get back to it.", home),
			Remedy: &Remedy{
				Summary: fmt.Sprintf("Add %q to the sidebar", title),
				apply: func(ctx context.Context, api API) error {
					return addSidebarLink(ctx, api, a.current, home, title, "house")
				},
			},
		})
	}

	// One finding for the whole sidebar, since one fix converts all of its links.
	legacy := 0

	for _, item := range portal.sidebar() {
		if isLegacyLink(item) {
			legacy++
		}
	}

	if legacy > 0 {
		a.add(Finding{
			ID:       "P003",
			Severity: SeverityError,
			Check:    "navigation shape",
			Message:  fmt.Sprintf("%d sidebar link(s) use the legacy shape.", legacy),
			Remedy: &Remedy{
				Summary: "Convert the sidebar links to the :v2 link types",
				apply:   a.convertSidebar,
			},
		})
	}

	for _, item := range portal.sidebar() {
		target, isText := item.Config["page_slug"].(string)
		if isText && !slices.Contains(pageSlugs, target) {
			a.add(Finding{
				ID:       "P004",
				Subject:  target,
				Severity: SeverityError,
				Check:    "sidebar link target",
				Message:  fmt.Sprintf("A sidebar link points to the page %q, which does not exist.", target),
				Remedy:   &Remedy{Summary: "Point the link at an existing page, or remove it"},
			})
		}
	}

	for _, page := range pages {
		a.pageFindings(page, sidebarPages)
	}
}

func (a *portalAudit) homePageFinding(home string) Finding {
	pages := a.current.Portal.pages()
	message := "No home page is set, so users land on an error."

	if home != "" {
		message = fmt.Sprintf("The home page %q does not match any page, so users land on an error.", home)
	}

	finding := Finding{ID: "P001", Severity: SeverityError, Check: "home page", Message: message}

	if len(pages) == 0 {
		finding.Remedy = &Remedy{
			Summary: "Add a page and set it as the home page",
			Command: fmt.Sprintf(`auth0 api patch "portals/%s" --data '{"pages":{"default":"<page-slug>","standard":[...]}}'`, a.portalID()),
		}

		return finding
	}

	first := pages[0]
	finding.Remedy = &Remedy{
		Summary: fmt.Sprintf("Set %q as the home page", first.Title),
		Command: fmt.Sprintf(`auth0 api patch "portals/%s" --data '{"pages":{"default":"%s","standard":[...]}}'`, a.portalID(), first.Slug),
		apply: func(ctx context.Context, api API) error {
			return patchPages(ctx, api, a.current, first.Slug)
		},
	}

	return finding
}

// convertSidebar converts every link of the sidebar to the :v2 types.
func (a *portalAudit) convertSidebar(ctx context.Context, api API) error {
	converted := make([]SidebarComponent, 0, len(a.current.Portal.sidebar()))
	for _, item := range a.current.Portal.sidebar() {
		converted = append(converted, toV2Link(item))
	}

	patch := PortalPatch{Navigation: &Navigation{Sidebar: &Sidebar{Components: converted}}}
	if err := api.UpdatePortal(ctx, a.portalID(), patch); err != nil {
		return fmt.Errorf("convert the sidebar of portal %q: %w", a.current.Portal.Slug, err)
	}

	return nil
}

func (a *portalAudit) pageFindings(page Page, sidebarPages []string) {
	if !slices.Contains(sidebarPages, page.Slug) {
		a.add(Finding{
			ID:       "P005",
			Subject:  page.Slug,
			Severity: SeverityWarning,
			Check:    "page in navigation",
			Message:  fmt.Sprintf("The page %q is not in the sidebar, so users cannot reach it.", page.Title),
			Remedy: &Remedy{
				Summary: fmt.Sprintf("Add %q to the sidebar", page.Title),
				apply: func(ctx context.Context, api API) error {
					return addSidebarLink(ctx, api, a.current, page.Slug, page.Title, "file")
				},
			},
		})
	}

	if len(page.Components) == 0 {
		a.add(Finding{
			ID:       "P006",
			Subject:  page.Slug,
			Severity: SeverityWarning,
			Check:    "empty page",
			Message:  fmt.Sprintf("The page %q has no components.", page.Title),
			Remedy:   &Remedy{Summary: "Add components to the page, or remove it"},
		})
	}
}

func (a *portalAudit) formsAndCustomCode() {
	for _, form := range a.current.Forms {
		if !form.Exists {
			a.add(Finding{
				ID:       "P007",
				Subject:  form.ID,
				Severity: SeverityError,
				Check:    "form exists",
				Message:  fmt.Sprintf("A form on the portal (%s) does not exist in the tenant, so its component fails.", form.ID),
				Remedy:   &Remedy{Summary: "Recreate the form, or remove the component from the page"},
			})
		}
	}

	if customCode := a.current.Portal.CustomCode; customCode != nil && customCode.Head != "" && a.current.CustomDomain == nil {
		a.add(Finding{
			ID:       "P008",
			Severity: SeverityWarning,
			Check:    "custom code",
			Message:  "Custom code is set, but the tenant has no ready custom domain, so it is never injected.",
			Remedy:   &Remedy{Summary: "Set up a custom domain, or remove the custom code"},
		})
	}
}

// application checks the application of the portal, and reports false when there is none, because the
// checks that follow need it.
func (a *portalAudit) application() bool {
	app := a.current.Application
	if app == nil {
		a.add(Finding{
			ID:       "P009",
			Severity: SeverityError,
			Check:    "application",
			Message:  "The portal has no application linked to it.",
			Remedy: &Remedy{
				Summary: "Create the portal application with the portal defaults and link it to the portal",
				apply: func(ctx context.Context, api API) error {
					return createPortalApplication(ctx, api, a.current, a.uses("my_organization"))
				},
			},
		})

		return false
	}

	clientID := app.ID

	if slices.Contains(app.GrantTypes, "client_credentials") {
		remaining := slices.DeleteFunc(slices.Clone(app.GrantTypes), func(grant string) bool { return grant == "client_credentials" })

		remedy := &Remedy{
			Summary: "Turn off the client_credentials grant",
			apply: func(ctx context.Context, api API) error {
				return updateApplication(ctx, api, clientID, ClientSettings{GrantTypes: remaining})
			},
		}

		// An update never sends an empty list, so with no grant left the change would be lost silently.
		if len(remaining) == 0 {
			remedy = &Remedy{
				Summary: "Replace the client_credentials grant with the grants the portal signs in with",
				Command: fmt.Sprintf(`auth0 api patch "clients/%s" --data '{"grant_types":["authorization_code","refresh_token"]}'`, clientID),
			}
		}

		a.add(Finding{
			ID:       "P010",
			Severity: SeverityError,
			Check:    "client credentials",
			Message:  "The portal application has the client_credentials grant, which the setup guide says must be off.",
			Remedy:   remedy,
		})
	}

	if app.RequireProofOfPossession != nil && *app.RequireProofOfPossession {
		a.add(Finding{
			ID:       "P023",
			Severity: SeverityError,
			Check:    "DPoP",
			Message:  "DPoP is enabled on the portal application. Portals are confidential clients and do not use DPoP.",
			Remedy: &Remedy{
				Summary: "Turn DPoP off on the portal application",
				apply: func(ctx context.Context, api API) error {
					return updateApplication(ctx, api, clientID, ClientSettings{RequireProofOfPossession: boolPointer(false)})
				},
			},
		})
	}

	a.managementGrants(clientID)

	callback := fmt.Sprintf("https://%s/portals/auth/callback", a.current.host())
	if !slices.Contains(app.Callbacks, callback) {
		callbacks := append(slices.Clone(app.Callbacks), callback)

		a.add(Finding{
			ID:       "P012",
			Severity: SeverityError,
			Check:    "callback url",
			Message:  fmt.Sprintf("The callback URL %s is missing, so sign-in to the portal fails.", callback),
			Remedy: &Remedy{
				Summary: "Add the portal callback URL",
				apply: func(ctx context.Context, api API) error {
					return updateApplication(ctx, api, clientID, ClientSettings{Callbacks: callbacks})
				},
			},
		})
	}

	return true
}

func updateApplication(ctx context.Context, api API, clientID string, settings ClientSettings) error {
	if err := api.UpdateClient(ctx, clientID, settings); err != nil {
		return fmt.Errorf("update application %q: %w", clientID, err)
	}

	return nil
}

// managementGrants reports the Management API grants of the application in one finding, since one fix
// removes them all.
func (a *portalAudit) managementGrants(clientID string) {
	var ids []string

	count := 0

	for _, grant := range a.current.Grants {
		if !strings.HasSuffix(grant.Audience, "/api/v2/") {
			continue
		}

		count++

		if grant.ID != "" {
			ids = append(ids, grant.ID)
		}
	}

	if count == 0 {
		return
	}

	remedy := &Remedy{Summary: "Remove the Management API grant from the portal application"}

	if len(ids) == count {
		remedy.apply = func(ctx context.Context, api API) error {
			for _, id := range ids {
				if err := api.DeleteClientGrant(ctx, id); err != nil {
					return fmt.Errorf("delete client grant %q: %w", id, err)
				}
			}

			return nil
		}
	} else {
		remedy.Command = fmt.Sprintf(`auth0 api get "client-grants?client_id=%s"`, clientID)
	}

	a.add(Finding{
		ID:       "P011",
		Severity: SeverityError,
		Check:    "management api grant",
		Message:  "The portal application has a Management API grant, which the setup guide says must not be there.",
		Remedy:   remedy,
	})
}

func (a *portalAudit) refreshToken() {
	app := *a.current.Application
	refresh := app.RefreshToken

	if refresh != nil && refresh.RotationType != "" && refresh.RotationType != "non-rotating" {
		a.add(Finding{
			ID:       "P013",
			Severity: SeverityError,
			Check:    "refresh token rotation",
			Message:  "Refresh token rotation is on, and portals do not support it. Components fail intermittently.",
			Remedy: &Remedy{
				Summary: "Turn refresh token rotation off",
				apply: func(ctx context.Context, api API) error {
					return patchRefresh(ctx, api, app, func(token *RefreshToken) { token.RotationType = "non-rotating" })
				},
			},
		})
	}

	switch {
	case refresh != nil && refresh.InfiniteIdleTokenLifetime != nil && *refresh.InfiniteIdleTokenLifetime:
		a.add(Finding{
			ID:       "P014",
			Severity: SeverityError,
			Check:    "refresh token idle lifetime",
			Message:  "The refresh token has no idle expiry, so the portal session never ends by inactivity.",
			Remedy: &Remedy{
				Summary: fmt.Sprintf("Set the idle lifetime to %d seconds", refreshTokenIdleSeconds),
				apply: func(ctx context.Context, api API) error {
					return patchRefresh(ctx, api, app, func(token *RefreshToken) {
						token.InfiniteIdleTokenLifetime = boolPointer(false)
						token.IdleTokenLifetime = intPointer(refreshTokenIdleSeconds)
					})
				},
			},
		})
	case refresh != nil && (!sameInt(refresh.IdleTokenLifetime, refreshTokenIdleSeconds) || !sameInt(refresh.TokenLifetime, refreshTokenAbsoluteSeconds)):
		a.add(Finding{
			ID:       "P015",
			Severity: SeverityWarning,
			Check:    "refresh token lifetimes",
			Message: fmt.Sprintf(
				"The refresh token lifetimes differ from the guide (%ds idle, %ds total).",
				refreshTokenIdleSeconds, refreshTokenAbsoluteSeconds,
			),
			Remedy: &Remedy{
				Summary: "Set the lifetimes to the guide's values",
				apply: func(ctx context.Context, api API) error {
					return patchRefresh(ctx, api, app, func(token *RefreshToken) {
						token.IdleTokenLifetime = intPointer(refreshTokenIdleSeconds)
						token.TokenLifetime = intPointer(refreshTokenAbsoluteSeconds)
					})
				},
			},
		})
	}
}

func sameInt(value *int, want int) bool {
	return value != nil && *value == want
}

func (a *portalAudit) organizationAndHeader() {
	app := *a.current.Application
	clientID := app.ID

	if app.OrganizationUsage != "allow" && a.uses("my_organization") {
		a.add(Finding{
			ID:       "P016",
			Severity: SeverityError,
			Check:    "organization usage",
			Message:  "The portal has organization components, but the application does not allow organizations.",
			Remedy: &Remedy{
				Summary: "Allow organizations on the application",
				apply: func(ctx context.Context, api API) error {
					return updateApplication(ctx, api, clientID, ClientSettings{OrganizationUsage: "allow"})
				},
			},
		})
	}

	if app.IPHeaderTrusted != nil && !*app.IPHeaderTrusted {
		a.add(Finding{
			ID:       "P017",
			Severity: SeverityWarning,
			Check:    "token endpoint IP header",
			Message:  "The guide says to trust the token endpoint IP header, and it is off.",
			Remedy: &Remedy{
				Summary: "Trust the token endpoint IP header",
				apply: func(ctx context.Context, api API) error {
					return updateApplication(ctx, api, clientID, ClientSettings{IPHeaderTrusted: boolPointer(true)})
				},
			},
		})
	}
}

// apisTheComponentsCall checks the resources the components call. A component without the grant of its
// API or its refresh token policy fails when the portal loads it.
func (a *portalAudit) apisTheComponentsCall() {
	app := *a.current.Application
	tenant := a.current.TenantDomain

	needed := []struct {
		audience string
		name     string
		needle   string
		scope    []string
	}{
		{audience: myAccountAudience(tenant), name: "My Account API", needle: "my_account", scope: myAccountScopes},
		{audience: myOrganizationAudience(tenant), name: "My Organization API", needle: "my_organization", scope: myOrganizationScopes},
	}

	for _, api := range needed {
		if !a.uses(api.needle) {
			continue
		}

		var usedBy []string

		for _, component := range a.usedComponents() {
			if strings.Contains(component.Type, api.needle) {
				usedBy = append(usedBy, componentLabel(component))
			}
		}

		components := strings.Join(usedBy, ", ")

		if !slices.Contains(a.current.ResourceServers, api.audience) {
			a.add(Finding{
				ID:       "P018",
				Subject:  api.audience,
				Severity: SeverityError,
				Check:    "resource server",
				Message: fmt.Sprintf(
					"The components (%s) need the %s, but the tenant has no resource server for %s.",
					components, api.name, api.audience,
				),
				Remedy: &Remedy{
					Summary: "Create the " + api.name + " resource server",
					apply: func(ctx context.Context, client API) error {
						_, err := client.CreateResourceServer(ctx, ResourceServerSpec{
							Name:       "Auth0 " + api.name,
							Identifier: api.audience,
							SkipConsentForVerifiableFirstPartyClients: true,
							TokenDialect: "rfc9068_profile",
						})
						if err != nil {
							return fmt.Errorf("create the %s resource server: %w", api.name, err)
						}

						return nil
					},
				},
			})
		}

		if !slices.ContainsFunc(a.current.Grants, func(grant ClientGrant) bool { return grant.Audience == api.audience }) {
			a.add(Finding{
				ID:       "P019",
				Subject:  api.audience,
				Severity: SeverityError,
				Check:    "client grant",
				Message: fmt.Sprintf(
					"The components (%s) need the %s, but the application has no client grant for it.",
					components, api.name,
				),
				Remedy: &Remedy{
					Summary: "Grant the application access to the " + api.name,
					apply: func(ctx context.Context, client API) error {
						_, err := client.CreateClientGrant(ctx, ClientGrantSpec{
							ClientID:    app.ID,
							Audience:    api.audience,
							Scope:       api.scope,
							SubjectType: "user",
						})
						if err != nil {
							return fmt.Errorf("grant the application access to the %s: %w", api.name, err)
						}

						return nil
					},
				},
			})
		}

		if app.RefreshToken == nil || !slices.ContainsFunc(app.RefreshToken.Policies, func(policy RefreshTokenPolicy) bool { return policy.Audience == api.audience }) {
			a.add(Finding{
				ID:       "P020",
				Subject:  api.audience,
				Severity: SeverityError,
				Check:    "refresh token policy",
				Message: fmt.Sprintf(
					"The components (%s) need the %s, but the refresh token has no policy for it, so the portal cannot get its tokens.",
					components, api.name,
				),
				Remedy: &Remedy{
					Summary: fmt.Sprintf("Add the %s to the multi-resource refresh token", api.name),
					apply: func(ctx context.Context, client API) error {
						return patchRefresh(ctx, client, app, func(token *RefreshToken) {
							token.Policies = append(token.Policies, RefreshTokenPolicy{Audience: api.audience, Scope: slices.Clone(api.scope)})
						})
					},
				},
			})
		}
	}
}

func (a *portalAudit) connectionsAndSessionTransfer() {
	app := *a.current.Application

	if len(a.current.ApplicationConnections) == 0 {
		a.add(Finding{
			ID:       "P021",
			Severity: SeverityError,
			Check:    "connections",
			Message:  "No connection is enabled for the application, so nobody can sign in.",
			Remedy: &Remedy{
				Summary: "Enable a connection for the application, or promote one to the tenant level",
				Command: fmt.Sprintf(`auth0 api patch "connections/<connection-id>/clients" --data '[{"client_id":"%s","status":true}]'`, app.ID),
			},
		})
	}

	if organization := a.current.Organization; organization != nil {
		shared := slices.ContainsFunc(organization.Connections, func(id string) bool {
			return slices.ContainsFunc(a.current.ApplicationConnections, func(connection Connection) bool { return connection.ID == id })
		})

		if !shared {
			a.add(Finding{
				ID:       "P022",
				Severity: SeverityError,
				Check:    "organization connections",
				Message: fmt.Sprintf(
					"Organization %q has no connection in common with the application, so its members cannot sign in.",
					organization.Name,
				),
				Remedy: &Remedy{
					Summary: "Enable a connection that the application and the organization share",
					Command: fmt.Sprintf(
						`auth0 api post "organizations/%s/enabled_connections" --data '{"connection_id":"<connection-id>","assign_membership_on_login":false}'`,
						organization.ID,
					),
				},
			})
		}
	}

	if app.SessionTransfer == nil || len(app.SessionTransfer.AllowedAuthenticationMethods) == 0 {
		a.add(Finding{
			ID:       "P024",
			Severity: SeverityWarning,
			Check:    "session transfer",
			Message:  "Session transfer is not enabled, so a native app cannot open the portal with a session.",
			Remedy: &Remedy{
				Summary: "Allow session transfer on the portal application",
				Command: fmt.Sprintf(
					`auth0 api patch "clients/%s" --data '{"session_transfer":{"allowed_authentication_methods":["query"],"enforce_device_binding":"ip"}}'`,
					app.ID,
				),
			},
		})
	}
}
