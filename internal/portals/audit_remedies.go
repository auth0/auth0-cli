package portals

import (
	"context"
	"fmt"
	"maps"
	"strings"
)

// The fixes the audit can apply. Each one changes the portal or its application in the tenant.
// The detection of what is wrong is in audit_checks.go.

const (
	internalLinkBase = "sidebar:component:auth0:internal_link"
	externalLinkBase = "sidebar:component:auth0:external_link"

	internalLinkType = internalLinkBase + ":v2"
	externalLinkType = externalLinkBase + ":v2"
)

// isLegacyLink reports whether a sidebar link has the legacy shape: it used "to" for pages and "url"
// for external links, or it has no :v2 type. Only the links to a page and the external links have one;
// any other type is left as it is.
func isLegacyLink(item SidebarComponent) bool {
	base := strings.TrimSuffix(item.Type, ":v2")
	if base != internalLinkBase && base != externalLinkBase {
		return false
	}

	_, hasTo := item.Config["to"]
	_, hasURL := item.Config["url"]

	return !strings.HasSuffix(item.Type, ":v2") || hasTo || hasURL
}

// toV2Link converts a legacy sidebar link to the :v2 types. The settings it does not rename are kept, and
// a setting it does not have is not added, because the API rejects a null one.
func toV2Link(item SidebarComponent) SidebarComponent {
	if !isLegacyLink(item) {
		return item
	}

	config := maps.Clone(item.Config)
	if config == nil {
		config = map[string]any{}
	}

	delete(config, "to")
	delete(config, "url")

	_, hasURL := item.Config["url"]
	_, hasHref := item.Config["href"]

	if hasURL || hasHref || strings.HasPrefix(item.Type, externalLinkBase) {
		if href := firstOf(item.Config["href"], item.Config["url"]); href != nil {
			config["href"] = href
		}

		return SidebarComponent{Type: externalLinkType, Config: config, extra: item.extra}
	}

	if page := firstOf(item.Config["page_slug"], item.Config["to"]); page != nil {
		config["page_slug"] = pageSlugOf(page)
	}

	return SidebarComponent{Type: internalLinkType, Config: config, extra: item.extra}
}

// pageSlugOf turns the address a legacy link used for a page ("/profile") into the slug of the page.
func pageSlugOf(page any) any {
	address, isText := page.(string)
	if !isText {
		return page
	}

	return strings.Trim(address, "/")
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
// with its rotation type, so the current settings are merged with the change. A flag that keeps a
// lifetime infinite is copied as it is, because the finding that owns it changes it.
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

		if current.ExpirationType != "" {
			token.ExpirationType = current.ExpirationType
		}

		token.InfiniteTokenLifetime = current.InfiniteTokenLifetime
		token.InfiniteIdleTokenLifetime = current.InfiniteIdleTokenLifetime

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

	patch := PortalPatch{Navigation: current.Portal.withSidebar(components)}
	if err := api.UpdatePortal(ctx, current.Portal.ID, patch); err != nil {
		return fmt.Errorf("add %q to the sidebar of portal %q: %w", label, current.Portal.Slug, err)
	}

	return nil
}
