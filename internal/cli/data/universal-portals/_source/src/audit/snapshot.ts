import { ApiClient, ApiError, listAll } from "../api.js";

// Everything the audit needs about one portal, read from the tenant. The checks
// work on this snapshot only, so they can be tested without a tenant.
export interface PortalSnapshot {
  tenantDomain: string;
  portal: PortalShape;
  application?: ApplicationShape;
  grants: { id?: string; audience: string; scope?: string[] }[];
  resourceServers: string[];
  // Form ID to whether the form exists in the tenant.
  forms: Record<string, boolean>;
  // Connections enabled on the application.
  applicationConnections: { id: string; name?: string }[];
  customDomain?: { domain: string; status: string };
  organization?: { id: string; name: string; connections: string[] };
}

export interface PortalShape {
  id: string;
  slug: string;
  name?: string;
  client?: { client_id: string };
  navigation?: { sidebar?: { components?: SidebarShape[] } };
  pages?: { default?: string; standard?: PageShape[] };
  custom_code?: { head?: string };
}

export interface SidebarShape {
  type: string;
  config?: Record<string, unknown>;
}

export interface PageShape {
  title: string;
  slug: string;
  components?: ComponentShape[];
}

export interface ComponentShape {
  type: string;
  config?: { children?: ComponentShape[]; form_id?: string; [key: string]: unknown };
}

export interface ApplicationShape {
  client_id: string;
  name?: string;
  app_type?: string;
  grant_types?: string[];
  callbacks?: string[];
  allowed_logout_urls?: string[];
  organization_usage?: string;
  is_token_endpoint_ip_header_trusted?: boolean;
  require_proof_of_possession?: boolean;
  session_transfer?: { allowed_authentication_methods?: string[] | null };
  refresh_token?: {
    rotation_type?: string;
    idle_token_lifetime?: number;
    token_lifetime?: number;
    infinite_idle_token_lifetime?: boolean;
    policies?: { audience: string; scope?: string[] }[];
  };
}

export interface SnapshotOptions {
  organizationId?: string;
}

export async function fetchPortalSnapshot(
  api: ApiClient,
  portalId: string,
  tenantDomain: string,
  options: SnapshotOptions = {},
): Promise<PortalSnapshot> {
  const portal = await api.request<PortalShape>("GET", `portals/${encodeURIComponent(portalId)}`);

  let application: ApplicationShape | undefined;
  let grants: PortalSnapshot["grants"] = [];
  let applicationConnections: PortalSnapshot["applicationConnections"] = [];
  const clientId = portal.client?.client_id;
  if (clientId) application = await readApplication(api, clientId);
  if (application && clientId) {
    grants = await listAll<PortalSnapshot["grants"][number]>(api, `client-grants?client_id=${encodeURIComponent(clientId)}`, "client_grants");
    applicationConnections = await listAll<{ id: string; name?: string }>(api, `clients/${encodeURIComponent(clientId)}/connections`, "connections");
  }

  const resourceServers = (await listAll<{ identifier: string }>(api, "resource-servers", "resource_servers")).map((server) => server.identifier);

  const forms: Record<string, boolean> = {};
  for (const formId of formIdsOf(portal)) {
    forms[formId] = await formExists(api, formId);
  }

  const customDomain = await readCustomDomain(api);
  const organization = options.organizationId ? await readOrganization(api, options.organizationId) : undefined;

  return {
    tenantDomain,
    portal,
    application,
    grants,
    resourceServers,
    forms,
    applicationConnections,
    customDomain,
    organization,
  };
}

export function formIdsOf(portal: PortalShape): string[] {
  const ids: string[] = [];
  const visit = (components: ComponentShape[] = []) => {
    for (const component of components) {
      if (component.type === "page:component:auth0:form" && component.config?.form_id) {
        ids.push(component.config.form_id);
      }
      visit(component.config?.children);
    }
  };
  for (const page of portal.pages?.standard ?? []) visit(page.components);
  return [...new Set(ids)];
}

// A portal can still point at an application that was deleted. That is a finding
// (P009), not a failure of the audit, so a 404 here means no application.
async function readApplication(api: ApiClient, clientId: string): Promise<ApplicationShape | undefined> {
  try {
    return await api.request<ApplicationShape>("GET", `clients/${encodeURIComponent(clientId)}`);
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) return undefined;
    throw error;
  }
}

// The API answers 400 for an ID it cannot match to a form (the ID fails its schema),
// which is the same finding as a deleted form (P007), so both mean the form is missing.
async function formExists(api: ApiClient, formId: string): Promise<boolean> {
  try {
    await api.request("GET", `forms/${encodeURIComponent(formId)}`);
    return true;
  } catch (error) {
    if (error instanceof ApiError && (error.status === 404 || error.status === 400)) return false;
    throw error;
  }
}

export async function readCustomDomain(api: ApiClient): Promise<PortalSnapshot["customDomain"]> {
  try {
    const custom = await api.request<{ domain: string; status: string }>("GET", "custom-domains/default");
    return custom;
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) return undefined;
    throw error;
  }
}

async function readOrganization(api: ApiClient, organizationId: string): Promise<PortalSnapshot["organization"]> {
  const org = await api.request<{ id: string; name: string }>("GET", `organizations/${encodeURIComponent(organizationId)}`);
  const connections = (await listAll<{ connection_id: string }>(api, `organizations/${encodeURIComponent(organizationId)}/enabled_connections`, "enabled_connections")).map((connection) => connection.connection_id);
  return { id: org.id, name: org.name, connections };
}

// The host the portal is served from: the custom domain when it is ready.
export function hostOf(snapshot: PortalSnapshot): string {
  return snapshot.customDomain?.status === "ready" ? snapshot.customDomain.domain : snapshot.tenantDomain;
}
