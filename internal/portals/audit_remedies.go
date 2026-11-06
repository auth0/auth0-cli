package portals

import (
	"context"
	"fmt"
	"strings"
)

// The fixes the audit can apply. Each one changes the portal or its application in the tenant.
// The detection of what is wrong is in audit_checks.go.

const (
	internalLinkType = "sidebar:component:auth0:internal_link:v2"
	externalLinkType = "sidebar:component:auth0:external_link:v2"
)

// isLegacyLink reports whether a sidebar link has the legacy shape: it used "to" for pages and "url"
// for external links, or it has no :v2 type.
func isLegacyLink(item SidebarComponent) bool {
	_, hasTo := item.Config["to"]
	_, hasURL := item.Config["url"]

	return !strings.HasSuffix(item.Type, ":v2") || hasTo || hasURL
}

// toV2Link converts a legacy sidebar link to the :v2 types.
func toV2Link(item SidebarComponent) SidebarComponent {
	if !isLegacyLink(item) {
		return item
	}

	_, hasURL := item.Config["url"]
	_, hasHref := item.Config["href"]
	external := hasURL || hasHref || strings.Contains(item.Type, "external")

	config := map[string]any{"label": item.Config["label"], "icon": item.Config["icon"]}

	if external {
		config["href"] = firstOf(item.Config["href"], item.Config["url"])

		return SidebarComponent{Type: externalLinkType, Config: config}
	}

	config["page_slug"] = firstOf(item.Config["page_slug"], item.Config["to"])

	return SidebarComponent{Type: internalLinkType, Config: config}
}

// firstOf returns the first value that is set.
func firstOf(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}

	return nil
}

// createPortalApplication creates the application with the same defaults setup uses, or reuses the one
// an earlier run created, and links it to the portal. The template is business when the portal has
// organization components. The secret is only returned on create, so the link has to follow in the
// same step, and a rerun finds the application by name.
func createPortalApplication(ctx context.Context, api API, current snapshot, usesOrganization bool) error {
	name := current.Portal.Name
	if name == "" {
		name = current.Portal.Slug
	}

	template := TemplateConsumer
	if usesOrganization {
		template = TemplateBusiness
	}

	client, err := ensurePortalClient(ctx, api, portalApplication{
		name:           portalName(name),
		slug:           current.Portal.Slug,
		template:       template,
		tenantDomain:   current.TenantDomain,
		host:           current.host(),
		isCustomDomain: current.CustomDomain != nil && current.CustomDomain.IsReady(),
	})
	if err != nil {
		return err
	}

	link := &PortalClient{
		ClientID:                client.value.clientID,
		ClientSecret:            client.value.clientSecret,
		TokenEndpointAuthMethod: "client_secret_post",
	}

	if err := api.UpdatePortal(ctx, current.Portal.ID, PortalPatch{Client: link}); err != nil {
		return fmt.Errorf("link the application to portal %q: %w", current.Portal.Slug, err)
	}

	return nil
}

// patchRefresh changes the refresh token of the application. The refresh token has to be sent whole,
// with its rotation type, so the current settings are merged with the change.
func patchRefresh(ctx context.Context, api API, app Client, change func(token *RefreshToken)) error {
	token := RefreshToken{
		RotationType:      "non-rotating",
		ExpirationType:    "expiring",
		TokenLifetime:     intPointer(refreshTokenAbsoluteSeconds),
		IdleTokenLifetime: intPointer(refreshTokenIdleSeconds),
	}

	if current := app.RefreshToken; current != nil {
		if current.RotationType != "" {
			token.RotationType = current.RotationType
		}

		if current.TokenLifetime != nil {
			token.TokenLifetime = current.TokenLifetime
		}

		if current.IdleTokenLifetime != nil {
			token.IdleTokenLifetime = current.IdleTokenLifetime
		}

		token.Policies = append([]RefreshTokenPolicy(nil), current.Policies...)
	}

	change(&token)

	if err := api.UpdateClient(ctx, app.ID, ClientSettings{RefreshToken: &token}); err != nil {
		return fmt.Errorf("update the refresh token of application %q: %w", app.ID, err)
	}

	return nil
}

// patchPages replaces the pages of the portal as a whole, so the page list is sent with the new home page.
func patchPages(ctx context.Context, api API, current snapshot, home string) error {
	pages := &Pages{Default: home, Standard: current.Portal.pages()}

	if err := api.UpdatePortal(ctx, current.Portal.ID, PortalPatch{Pages: pages}); err != nil {
		return fmt.Errorf("set the home page of portal %q: %w", current.Portal.Slug, err)
	}

	return nil
}

// addSidebarLink adds a link to a page at the end of the sidebar.
func addSidebarLink(ctx context.Context, api API, current snapshot, pageSlug, label, icon string) error {
	components := append([]SidebarComponent(nil), current.Portal.sidebar()...)
	components = append(components, SidebarComponent{
		Type:   internalLinkType,
		Config: map[string]any{"label": label, "page_slug": pageSlug, "icon": icon},
	})

	patch := PortalPatch{Navigation: &Navigation{Sidebar: &Sidebar{Components: components}}}
	if err := api.UpdatePortal(ctx, current.Portal.ID, patch); err != nil {
		return fmt.Errorf("add %q to the sidebar of portal %q: %w", label, current.Portal.Slug, err)
	}

	return nil
}
