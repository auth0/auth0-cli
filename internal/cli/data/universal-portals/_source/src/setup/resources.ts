import { ApiClient, ApiError, listAll } from "../api.js";
import type { CreatePortalRequest } from "../portal-model.js";
import type { FormDefinition, FormKey } from "../forms.js";
import { buildPortalClient, buildFormsVaultClient, PortalClientInput } from "./payload.js";

// Every ensure* helper looks for its resource by a stable key before it
// creates one, so rerunning setup after a failure reuses what already exists.

export type Status = "created" | "reused" | "updated";

export function describe(status: Status): string {
  return status === "reused" ? "already exists" : status === "updated" ? "updated" : "created";
}

export interface Ensured<T> {
  value: T;
  status: Status;
}

export class NameInUseError extends Error {
  constructor(readonly name: string) {
    super(
      `An application named "${name}" already exists and was not created by this setup. Rename or delete it, then rerun.`,
    );
    this.name = "NameInUseError";
  }
}

export class SlugTakenError extends Error {
  constructor(readonly slug: string) {
    super(`The portal slug "${slug}" is already used by another portal.`);
    this.name = "SlugTakenError";
  }
}

interface ClientRecord {
  client_id: string;
  name?: string;
  client_metadata?: Record<string, string>;
  session_transfer?: { allowed_authentication_methods?: string[] };
}

interface ResourceServerRecord {
  id: string;
  identifier: string;
}

interface GrantRecord {
  id: string;
  audience: string;
  scope?: string[];
}

interface VaultRecord {
  id: string;
  name: string;
}

export interface PortalRecord {
  id: string;
  slug: string;
  client?: { client_id: string };
}

export async function ensureResourceServer(
  api: ApiClient,
  input: { name: string; identifier: string },
): Promise<Ensured<{ id: string }>> {
  const existing = await findResourceServer(api, input.identifier);
  if (existing) return { value: { id: existing.id }, status: "reused" };

  try {
    const created = await api.request<ResourceServerRecord>("POST", "resource-servers", {
      name: input.name,
      identifier: input.identifier,
      skip_consent_for_verifiable_first_party_clients: true,
      token_dialect: "rfc9068_profile",
    });
    return { value: { id: created.id }, status: "created" };
  } catch (error) {
    if (error instanceof ApiError && error.status === 409) {
      const raced = await findResourceServer(api, input.identifier);
      if (raced) return { value: { id: raced.id }, status: "reused" };
    }
    throw error;
  }
}

async function findResourceServer(api: ApiClient, identifier: string): Promise<ResourceServerRecord | undefined> {
  return (await listAll<ResourceServerRecord>(api, "resource-servers", "resource_servers")).find((server) => server.identifier === identifier);
}

export async function ensurePortalClient(
  api: ApiClient,
  input: PortalClientInput,
): Promise<Ensured<{ clientId: string; clientSecret: string }>> {
  const existing = await findClientByName(api, input.name);
  const payload = buildPortalClient(input);

  if (existing) {
    if (existing.client_metadata?.portals_setup_slug !== input.slug) {
      throw new NameInUseError(input.name);
    }
    await api.request("PATCH", `clients/${existing.client_id}`, {
      callbacks: payload.callbacks,
      allowed_logout_urls: payload.allowed_logout_urls,
      grant_types: payload.grant_types,
      organization_usage: payload.organization_usage,
      organization_require_behavior: payload.organization_require_behavior,
      is_token_endpoint_ip_header_trusted: payload.is_token_endpoint_ip_header_trusted,
      refresh_token: payload.refresh_token,
      ...(payload.oidc_logout ? { oidc_logout: payload.oidc_logout } : {}),
      // Set only when missing, so a session transfer setting changed on purpose survives a rerun.
      ...(existing.session_transfer?.allowed_authentication_methods?.length
        ? {}
        : { session_transfer: payload.session_transfer }),
    });
    const clientSecret = await readClientSecret(api, existing.client_id);
    return { value: { clientId: existing.client_id, clientSecret }, status: "reused" };
  }

  const created = await api.request<{ client_id: string; client_secret: string }>("POST", "clients", payload);
  return { value: { clientId: created.client_id, clientSecret: created.client_secret }, status: "created" };
}

export async function findClientByName(api: ApiClient, name: string): Promise<ClientRecord | undefined> {
  const clients = await listAll<ClientRecord>(
    api,
    "clients?fields=client_id,name,client_metadata,session_transfer&include_fields=true",
    "clients",
  );
  return clients.find((client) => client.name === name);
}

// The client secret is only returned by GET /clients/{id} with read:client_keys.
export async function readClientSecret(api: ApiClient, clientId: string): Promise<string> {
  const client = await api.request<{ client_secret?: string }>(
    "GET",
    `clients/${encodeURIComponent(clientId)}?fields=client_id,client_secret&include_fields=true`,
  );
  if (!client.client_secret) {
    throw new Error(`Could not read the client secret for ${clientId}. The session needs read:client_keys.`);
  }
  return client.client_secret;
}

export async function ensureClientGrant(
  api: ApiClient,
  input: { clientId: string; audience: string; scope: string[]; subjectType?: "user" | "client" },
): Promise<Ensured<{ id: string }>> {
  const existing = (await listAll<GrantRecord>(api, `client-grants?client_id=${encodeURIComponent(input.clientId)}`, "client_grants")).find((grant) => grant.audience === input.audience);

  if (!existing) {
    const created = await api.request<GrantRecord>("POST", "client-grants", {
      client_id: input.clientId,
      audience: input.audience,
      scope: input.scope,
      ...(input.subjectType ? { subject_type: input.subjectType } : {}),
    });
    return { value: { id: created.id }, status: "created" };
  }

  if (sameScopes(existing.scope ?? [], input.scope)) {
    return { value: { id: existing.id }, status: "reused" };
  }
  await api.request("PATCH", `client-grants/${existing.id}`, { scope: input.scope });
  return { value: { id: existing.id }, status: "updated" };
}

function sameScopes(left: string[], right: string[]): boolean {
  const a = [...new Set(left)].sort();
  const b = [...new Set(right)].sort();
  return a.length === b.length && a.every((scope, index) => scope === b[index]);
}

export async function ensureFormsVaultClient(
  api: ApiClient,
  name: string,
): Promise<Ensured<{ clientId: string; clientSecret: string }>> {
  const existing = await findClientByName(api, name);
  if (existing) {
    if (existing.client_metadata?.auth0_portals_role !== "forms-vault") {
      throw new NameInUseError(name);
    }
    const clientSecret = await readClientSecret(api, existing.client_id);
    return { value: { clientId: existing.client_id, clientSecret }, status: "reused" };
  }
  const created = await api.request<{ client_id: string; client_secret: string }>(
    "POST",
    "clients",
    buildFormsVaultClient(name),
  );
  return { value: { clientId: created.client_id, clientSecret: created.client_secret }, status: "created" };
}

export interface VaultSetup {
  domain: string;
  clientId: string;
  clientSecret: string;
}

// The vault API uses snake_case keys, unlike the rest of this module.
function vaultSetupPayload(setup: VaultSetup): Record<string, string> {
  return {
    domain: setup.domain,
    client_id: setup.clientId,
    client_secret: setup.clientSecret,
    type: "OAUTH_APP",
  };
}

export async function ensureVaultConnection(
  api: ApiClient,
  name: string,
  setup: VaultSetup,
): Promise<Ensured<{ id: string }>> {
  const existing = (await listAll<VaultRecord>(api, "flows/vault/connections", "connections")).find((connection) => connection.name === name);
  if (existing) return { value: { id: existing.id }, status: "reused" };

  const created = await api.request<VaultRecord>("POST", "flows/vault/connections", {
    app_id: "AUTH0",
    name,
    setup: vaultSetupPayload(setup),
  });
  return { value: { id: created.id }, status: "created" };
}

// A PATCH without setup keeps the old session. Sending setup again makes the
// vault mint a new session with the grants in force at that moment.
export async function refreshVaultConnection(api: ApiClient, vaultId: string, setup: VaultSetup): Promise<void> {
  await api.request("PATCH", `flows/vault/connections/${encodeURIComponent(vaultId)}`, {
    setup: vaultSetupPayload(setup),
  });
}

export interface FormRef {
  id: string;
  flowId?: string;
}

// The import response lists the flows it created, each with its ID.
function flowIdFromImport(bundle: { flows?: { id?: string }[] | Record<string, { id?: string }> }): string | undefined {
  return Object.values(bundle.flows ?? {}).find((flow) => flow.id)?.id;
}

// A form's FLOW nodes reference the flow they run by its ID.
function flowIdFromNodes(form: { nodes?: { type?: string; config?: { flow_id?: string } }[] }): string | undefined {
  return form.nodes?.find((node) => node.type === "FLOW" && node.config?.flow_id)?.config?.flow_id;
}

export async function ensureForms(
  api: ApiClient,
  forms: FormDefinition[],
): Promise<Record<FormKey, Ensured<FormRef>>> {
  const existing = await listAll<{ id: string; name: string }>(api, "forms", "forms");
  const result = {} as Record<FormKey, Ensured<FormRef>>;

  for (const form of forms) {
    const match = existing.find((candidate) => candidate.name === form.name);
    if (match) {
      const detail = await api.request<{ nodes?: { type?: string; config?: { flow_id?: string } }[] }>(
        "GET",
        `forms/${encodeURIComponent(match.id)}`,
      );
      result[form.key] = { value: { id: match.id, flowId: flowIdFromNodes(detail) }, status: "reused" };
      continue;
    }
    const imported = await api.request<{
      form: { id: string; nodes?: { type?: string; config?: { flow_id?: string } }[] };
      flows?: { id?: string }[];
    }>("POST", "forms/import", form.document);
    const flowId = flowIdFromImport(imported) ?? flowIdFromNodes(imported.form);
    result[form.key] = { value: { id: imported.form.id, flowId }, status: "created" };
  }
  return result;
}

// The portal with this slug, read in full (with its application), or undefined when the slug is free.
export async function findPortalBySlug(api: ApiClient, slug: string): Promise<PortalRecord | undefined> {
  const listed = (await listAll<PortalRecord>(api, "portals", "portals")).find((portal) => portal.slug === slug);
  if (!listed) return undefined;
  return api.request<PortalRecord>("GET", `portals/${encodeURIComponent(listed.id)}`);
}

export async function ensurePortal(
  api: ApiClient,
  request: CreatePortalRequest,
): Promise<Ensured<{ id: string; slug: string }>> {
  const existing = await findPortalBySlug(api, request.slug);

  if (existing) {
    if (existing.client?.client_id !== request.client.client_id) {
      throw new SlugTakenError(request.slug);
    }
    return { value: { id: existing.id, slug: existing.slug }, status: "reused" };
  }

  try {
    const created = await api.request<PortalRecord>("POST", "portals", request);
    return { value: { id: created.id, slug: created.slug }, status: "created" };
  } catch (error) {
    if (error instanceof ApiError && error.status === 409) throw new SlugTakenError(request.slug);
    throw error;
  }
}
