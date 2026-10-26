import {
  MY_ACCOUNT_SCOPES,
  MY_ORGANIZATION_SCOPES,
  myAccountAudience,
  myOrganizationAudience,
  REFRESH_TOKEN_LIFETIMES,
} from "../setup/payload.js";
import { addSidebarLink, createPortalApplication, isLegacyLink, patchPages, patchRefresh, toV2Link } from "./remedies.js";
import { hostOf } from "./snapshot.js";
import type { ApiClient } from "../api.js";
import type { ComponentShape, PortalSnapshot } from "./snapshot.js";

// Each check reads the snapshot and returns findings. Errors break the portal
// or a component, warnings are degraded or against the setup guide.
export type Severity = "error" | "warning";

// How to fix a finding. `summary` is what the fix does. `command` is the
// equivalent manual command. `apply` is set when the CLI can make the change.
export interface Remedy {
  summary: string;
  command?: string;
  apply?: (api: ApiClient) => Promise<unknown>;
}

export interface Finding {
  id: string;
  severity: Severity;
  check: string;
  message: string;
  // What the finding is about (a page, form or API). Checks that can repeat an ID
  // set it, so a fix matches its finding by ID and subject, never by message text.
  subject?: string;
  remedy?: Remedy;
}

export function auditPortal(snapshot: PortalSnapshot): Finding[] {
  const findings: Finding[] = [];
  const add = (finding: Finding) => findings.push(finding);
  const portalId = snapshot.portal.id;
  const pages = snapshot.portal.pages?.standard ?? [];
  const pageSlugs = new Set(pages.map((page) => page.slug));
  const sidebar = snapshot.portal.navigation?.sidebar?.components ?? [];
  const sidebarPages = new Set(sidebar.map((item) => String(item.config?.page_slug ?? "")));
  const usedComponents = allComponents(pages);
  const usesAccount = usedComponents.some((c) => c.type.includes("my_account"));
  const usesOrganization = usedComponents.some((c) => c.type.includes("my_organization"));

  // Pages and navigation
  const home = snapshot.portal.pages?.default;
  if (!home || !pageSlugs.has(home)) {
    const first = pages[0];
    add({
      id: "P001",
      severity: "error",
      check: "home page",
      message: home
        ? `The home page "${home}" does not match any page, so users land on an error.`
        : "No home page is set, so users land on an error.",
      remedy: first
        ? {
            summary: `Set "${first.title}" as the home page`,
            command: `auth0 api patch "portals/${portalId}" --data '{"pages":{"default":"${first.slug}","standard":[...]}}'`,
            apply: (api) => patchPages(api, portalId, snapshot, first.slug),
          }
        : {
            summary: "Add a page and set it as the home page",
            command: `auth0 api patch "portals/${portalId}" --data '{"pages":{"default":"<page-slug>","standard":[...]}}'`,
          },
    });
  } else if (!sidebarPages.has(home)) {
    const page = pages.find((candidate) => candidate.slug === home);
    add({
      id: "P002",
      severity: "warning",
      check: "home page in navigation",
      message: `The home page "${home}" is not in the sidebar, so users cannot get back to it.`,
      remedy: {
        summary: `Add "${page?.title ?? home}" to the sidebar`,
        apply: (api) => addSidebarLink(api, portalId, snapshot, home, page?.title ?? home, "house"),
      },
    });
  }

  // One finding for the whole sidebar, since one fix converts all of its links.
  const legacyLinks = sidebar.filter(isLegacyLink);
  if (legacyLinks.length > 0) {
    add({
      id: "P003",
      severity: "error",
      check: "navigation shape",
      message: `${legacyLinks.length} sidebar link(s) use the legacy shape.`,
      remedy: {
        summary: "Convert the sidebar links to the :v2 link types",
        apply: (api) =>
          api.request("PATCH", `portals/${encodeURIComponent(portalId)}`, {
            navigation: { sidebar: { components: sidebar.map(toV2Link) } },
          }),
      },
    });
  }

  for (const item of sidebar) {
    const target = item.config?.page_slug;
    if (typeof target === "string" && !pageSlugs.has(target)) {
      add({
        id: "P004",
        subject: target,
        severity: "error",
        check: "sidebar link target",
        message: `A sidebar link points to the page "${target}", which does not exist.`,
        remedy: { summary: "Point the link at an existing page, or remove it" },
      });
    }
  }

  for (const page of pages) {
    if (!sidebarPages.has(page.slug)) {
      add({
        id: "P005",
        subject: page.slug,
        severity: "warning",
        check: "page in navigation",
        message: `The page "${page.title}" is not in the sidebar, so users cannot reach it.`,
        remedy: {
          summary: `Add "${page.title}" to the sidebar`,
          apply: (api) => addSidebarLink(api, portalId, snapshot, page.slug, page.title, "file"),
        },
      });
    }
    if ((page.components ?? []).length === 0) {
      add({
        id: "P006",
        subject: page.slug,
        severity: "warning",
        check: "empty page",
        message: `The page "${page.title}" has no components.`,
        remedy: { summary: "Add components to the page, or remove it" },
      });
    }
  }

  for (const formId of Object.keys(snapshot.forms)) {
    if (!snapshot.forms[formId]) {
      add({
        id: "P007",
        subject: formId,
        severity: "error",
        check: "form exists",
        message: `A form on the portal (${formId}) does not exist in the tenant, so its component fails.`,
        remedy: { summary: "Recreate the form, or remove the component from the page" },
      });
    }
  }

  if (snapshot.portal.custom_code?.head && !snapshot.customDomain) {
    add({
      id: "P008",
      severity: "warning",
      check: "custom code",
      message: "Custom code is set, but the tenant has no ready custom domain, so it is never injected.",
      remedy: { summary: "Set up a custom domain, or remove the custom code" },
    });
  }

  // Application
  const app = snapshot.application;
  if (!app) {
    add({
      id: "P009",
      severity: "error",
      check: "application",
      message: "The portal has no application linked to it.",
      remedy: {
        summary: "Create the portal application with the portal defaults and link it to the portal",
        apply: (api) => createPortalApplication(api, snapshot, usesOrganization),
      },
    });
    return findings;
  }
  const clientId = app.client_id;

  if (app.grant_types?.includes("client_credentials")) {
    add({
      id: "P010",
      severity: "error",
      check: "client credentials",
      message: "The portal application has the client_credentials grant, which the setup guide says must be off.",
      remedy: {
        summary: "Turn off the client_credentials grant",
        apply: (api) =>
          api.request("PATCH", `clients/${encodeURIComponent(clientId)}`, {
            grant_types: (app.grant_types ?? []).filter((grant) => grant !== "client_credentials"),
          }),
      },
    });
  }

  if (app.require_proof_of_possession) {
    add({
      id: "P023",
      severity: "error",
      check: "DPoP",
      message: "DPoP is enabled on the portal application. Portals are confidential clients and do not use DPoP.",
      remedy: {
        summary: "Turn DPoP off on the portal application",
        apply: (api) =>
          api.request("PATCH", `clients/${encodeURIComponent(clientId)}`, { require_proof_of_possession: false }),
      },
    });
  }

  // One finding for all Management API grants, since one fix removes them all.
  const managementGrants = snapshot.grants.filter((candidate) => candidate.audience.endsWith("/api/v2/"));
  if (managementGrants.length > 0) {
    const ids = managementGrants.map((grant) => grant.id).filter((id): id is string => Boolean(id));
    add({
      id: "P011",
      severity: "error",
      check: "management api grant",
      message: "The portal application has a Management API grant, which the setup guide says must not be there.",
      remedy: {
        summary: "Remove the Management API grant from the portal application",
        apply: ids.length === managementGrants.length
          ? async (api) => {
              for (const id of ids) await api.request("DELETE", `client-grants/${encodeURIComponent(id)}`);
            }
          : undefined,
        command: ids.length === managementGrants.length ? undefined : "auth0 api get \"client-grants?client_id=" + clientId + "\"",
      },
    });
  }

  const callback = `https://${hostOf(snapshot)}/portals/auth/callback`;
  if (!app.callbacks?.includes(callback)) {
    add({
      id: "P012",
      severity: "error",
      check: "callback url",
      message: `The callback URL ${callback} is missing, so sign-in to the portal fails.`,
      remedy: {
        summary: "Add the portal callback URL",
        apply: (api) =>
          api.request("PATCH", `clients/${encodeURIComponent(clientId)}`, {
            callbacks: [...(app.callbacks ?? []), callback],
          }),
      },
    });
  }

  const refresh = app.refresh_token;
  if (refresh?.rotation_type && refresh.rotation_type !== "non-rotating") {
    add({
      id: "P013",
      severity: "error",
      check: "refresh token rotation",
      message: "Refresh token rotation is on, and portals do not support it. Components fail intermittently.",
      remedy: {
        summary: "Turn refresh token rotation off",
        apply: (api) => patchRefresh(api, app, { rotation_type: "non-rotating" }),
      },
    });
  }
  if (refresh?.infinite_idle_token_lifetime) {
    add({
      id: "P014",
      severity: "error",
      check: "refresh token idle lifetime",
      message: "The refresh token has no idle expiry, so the portal session never ends by inactivity.",
      remedy: {
        summary: `Set the idle lifetime to ${REFRESH_TOKEN_LIFETIMES.idleSeconds} seconds`,
        apply: (api) => patchRefresh(api, app, { infinite_idle_token_lifetime: false, idle_token_lifetime: REFRESH_TOKEN_LIFETIMES.idleSeconds }),
      },
    });
  } else if (
    refresh &&
    (refresh.idle_token_lifetime !== REFRESH_TOKEN_LIFETIMES.idleSeconds ||
      refresh.token_lifetime !== REFRESH_TOKEN_LIFETIMES.absoluteSeconds)
  ) {
    add({
      id: "P015",
      severity: "warning",
      check: "refresh token lifetimes",
      message: `The refresh token lifetimes differ from the guide (${REFRESH_TOKEN_LIFETIMES.idleSeconds}s idle, ${REFRESH_TOKEN_LIFETIMES.absoluteSeconds}s total).`,
      remedy: {
        summary: "Set the lifetimes to the guide's values",
        apply: (api) =>
          patchRefresh(api, app, {
            idle_token_lifetime: REFRESH_TOKEN_LIFETIMES.idleSeconds,
            token_lifetime: REFRESH_TOKEN_LIFETIMES.absoluteSeconds,
          }),
      },
    });
  }

  if (app.organization_usage !== "allow" && usesOrganization) {
    add({
      id: "P016",
      severity: "error",
      check: "organization usage",
      message: "The portal has organization components, but the application does not allow organizations.",
      remedy: {
        summary: 'Allow organizations on the application',
        apply: (api) => api.request("PATCH", `clients/${encodeURIComponent(clientId)}`, { organization_usage: "allow" }),
      },
    });
  }

  if (app.is_token_endpoint_ip_header_trusted === false) {
    add({
      id: "P017",
      severity: "warning",
      check: "token endpoint IP header",
      message: "The guide says to trust the token endpoint IP header, and it is off.",
      remedy: {
        summary: "Trust the token endpoint IP header",
        apply: (api) =>
          api.request("PATCH", `clients/${encodeURIComponent(clientId)}`, { is_token_endpoint_ip_header_trusted: true }),
      },
    });
  }

  // Resources the components call. A component without its API grant or its
  // refresh-token policy fails when the portal loads it.
  const needed: { audience: string; name: string; scope: string[]; used: boolean }[] = [
    { audience: myAccountAudience(snapshot.tenantDomain), name: "My Account API", scope: MY_ACCOUNT_SCOPES, used: usesAccount },
    {
      audience: myOrganizationAudience(snapshot.tenantDomain),
      name: "My Organization API",
      scope: MY_ORGANIZATION_SCOPES,
      used: usesOrganization,
    },
  ];
  for (const api of needed.filter((entry) => entry.used)) {
    const usedBy = componentsOn(usedComponents, api.name)
      .map((component) => componentLabel(component))
      .join(", ");
    if (!snapshot.resourceServers.includes(api.audience)) {
      add({
        id: "P018",
        subject: api.audience,
        severity: "error",
        check: "resource server",
        message: `The components (${usedBy}) need the ${api.name}, but the tenant has no resource server for ${api.audience}.`,
        remedy: {
          summary: `Create the ${api.name} resource server`,
          apply: (client) =>
            client.request("POST", "resource-servers", {
              name: `Auth0 ${api.name}`,
              identifier: api.audience,
              skip_consent_for_verifiable_first_party_clients: true,
              token_dialect: "rfc9068_profile",
            }),
        },
      });
    }
    if (!snapshot.grants.some((grant) => grant.audience === api.audience)) {
      add({
        id: "P019",
        subject: api.audience,
        severity: "error",
        check: "client grant",
        message: `The components (${usedBy}) need the ${api.name}, but the application has no client grant for it.`,
        remedy: {
          summary: `Grant the application access to the ${api.name}`,
          apply: (client) =>
            client.request("POST", "client-grants", {
              client_id: clientId,
              audience: api.audience,
              scope: api.scope,
              subject_type: "user",
            }),
        },
      });
    }
    if (!refresh?.policies?.some((policy) => policy.audience === api.audience)) {
      add({
        id: "P020",
        subject: api.audience,
        severity: "error",
        check: "refresh token policy",
        message: `The components (${usedBy}) need the ${api.name}, but the refresh token has no policy for it, so the portal cannot get its tokens.`,
        remedy: {
          summary: `Add the ${api.name} to the multi-resource refresh token`,
          apply: (client) =>
            patchRefresh(client, app, {
              policies: [...(refresh?.policies ?? []), { audience: api.audience, scope: api.scope }],
            }),
        },
      });
    }
  }

  if (snapshot.applicationConnections.length === 0) {
    add({
      id: "P021",
      severity: "error",
      check: "connections",
      message: "No connection is enabled for the application, so nobody can sign in.",
      remedy: {
        summary: "Enable a connection for the application, or promote one to the tenant level",
        command: `auth0 api patch "connections/<connection-id>/clients" --data '[{"client_id":"${clientId}","status":true}]'`,
      },
    });
  }

  if (snapshot.organization) {
    const shared = snapshot.organization.connections.some((id) =>
      snapshot.applicationConnections.some((connection) => connection.id === id),
    );
    if (!shared) {
      add({
        id: "P022",
        severity: "error",
        check: "organization connections",
        message: `Organization "${snapshot.organization.name}" has no connection in common with the application, so its members cannot sign in.`,
        remedy: {
          summary: "Enable a connection that the application and the organization share",
          command: `auth0 api post "organizations/${snapshot.organization.id}/enabled_connections" --data '{"connection_id":"<connection-id>","assign_membership_on_login":false}'`,
        },
      });
    }
  }

  if (!app.session_transfer?.allowed_authentication_methods?.length) {
    add({
      id: "P024",
      severity: "warning",
      check: "session transfer",
      message: "Session transfer is not enabled, so a native app cannot open the portal with a session.",
      remedy: {
        summary: "Allow session transfer on the portal application",
        command: `auth0 api patch "clients/${clientId}" --data '{"session_transfer":{"allowed_authentication_methods":["query"],"enforce_device_binding":"ip"}}'`,
      },
    });
  }

  return findings;
}

function componentsOn(components: ComponentShape[], apiName: string): ComponentShape[] {
  const needle = apiName === "My Account API" ? "my_account" : "my_organization";
  return components.filter((component) => component.type.includes(needle));
}

function componentLabel(component: ComponentShape): string {
  const name = component.type.split(":").pop() ?? component.type;
  return name.replace(/_/g, " ");
}

function allComponents(pages: { components?: ComponentShape[] }[]): ComponentShape[] {
  const all: ComponentShape[] = [];
  const visit = (components: ComponentShape[] = []) => {
    for (const component of components) {
      all.push(component);
      visit(component.config?.children);
    }
  };
  for (const page of pages) visit(page.components);
  return all;
}
