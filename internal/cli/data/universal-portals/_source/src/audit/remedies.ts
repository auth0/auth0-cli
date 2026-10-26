import type { ApiClient } from "../api.js";
import { ensurePortalClient } from "../setup/resources.js";
import { PORTAL_APP_SUFFIX, REFRESH_TOKEN_LIFETIMES } from "../setup/payload.js";
import { ApplicationShape, PortalSnapshot, SidebarShape, hostOf } from "./snapshot.js";

// The fixes the audit can apply. Each one changes the portal or its application in
// the tenant. Detection stays in checks.ts.

// Legacy sidebar links used "to" for pages and "url" for external links, or had no :v2 type.
export function isLegacyLink(item: SidebarShape): boolean {
  return !item.type.endsWith(":v2") || item.config?.to !== undefined || item.config?.url !== undefined;
}

// Converts the legacy sidebar links to the :v2 types.
export function toV2Link(item: SidebarShape): SidebarShape {
  if (!isLegacyLink(item)) return item;
  const external = item.config?.url !== undefined || item.config?.href !== undefined || item.type.includes("external");
  const config: Record<string, unknown> = { label: item.config?.label, icon: item.config?.icon };
  if (external) {
    config.href = item.config?.href ?? item.config?.url;
    return { type: "sidebar:component:auth0:external_link:v2", config };
  }
  config.page_slug = item.config?.page_slug ?? item.config?.to;
  return { type: "sidebar:component:auth0:internal_link:v2", config };
}

// Creates the application with the same defaults setup uses (or reuses the one an
// earlier run created), then links it to the portal. The template is business when
// the portal has organization components. The secret is only returned on create, so
// the link has to follow in the same step, and a rerun finds the application by name.
export async function createPortalApplication(api: ApiClient, snapshot: PortalSnapshot, usesOrganization: boolean): Promise<unknown> {
  const client = await ensurePortalClient(api, {
    name: `${snapshot.portal.name ?? snapshot.portal.slug} ${PORTAL_APP_SUFFIX}`,
    slug: snapshot.portal.slug,
    template: usesOrganization ? "business" : "consumer",
    tenantDomain: snapshot.tenantDomain,
    portalHost: hostOf(snapshot),
    isCustomDomain: snapshot.customDomain?.status === "ready",
  });
  return api.request("PATCH", `portals/${encodeURIComponent(snapshot.portal.id)}`, {
    client: {
      client_id: client.value.clientId,
      client_secret: client.value.clientSecret,
      token_endpoint_auth_method: "client_secret_post",
    },
  });
}

// The refresh token has to be sent whole, with its rotation type, so the
// current settings are merged with the change.
export function patchRefresh(api: ApiClient, app: ApplicationShape, changes: Record<string, unknown>): Promise<unknown> {
  const current = app.refresh_token ?? {};
  return api.request("PATCH", `clients/${encodeURIComponent(app.client_id)}`, {
    refresh_token: {
      rotation_type: current.rotation_type ?? "non-rotating",
      expiration_type: "expiring",
      token_lifetime: current.token_lifetime ?? REFRESH_TOKEN_LIFETIMES.absoluteSeconds,
      idle_token_lifetime: current.idle_token_lifetime ?? REFRESH_TOKEN_LIFETIMES.idleSeconds,
      policies: current.policies ?? [],
      ...changes,
    },
  });
}

// The portal's pages are replaced as a whole, so the page list is sent with
// the new home page.
export function patchPages(api: ApiClient, portalId: string, snapshot: PortalSnapshot, home: string): Promise<unknown> {
  return api.request("PATCH", `portals/${encodeURIComponent(portalId)}`, {
    pages: { default: home, standard: snapshot.portal.pages?.standard ?? [] },
  });
}

export function addSidebarLink(
  api: ApiClient,
  portalId: string,
  snapshot: PortalSnapshot,
  pageSlug: string,
  label: string,
  icon: string,
): Promise<unknown> {
  const components = snapshot.portal.navigation?.sidebar?.components ?? [];
  return api.request("PATCH", `portals/${encodeURIComponent(portalId)}`, {
    navigation: {
      sidebar: {
        components: [
          ...components,
          { type: "sidebar:component:auth0:internal_link:v2", config: { label, page_slug: pageSlug, icon } },
        ],
      },
    },
  });
}
