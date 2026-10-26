import { ApiClient, ApiError, HttpMethod, toApiError } from "../src/api.js";

// The shape of each ID the Management API returns. IDs come from a counter, so a
// run is repeatable and its output can be compared byte for byte.
const ID_SHAPES: Record<string, { prefix: string; length: number }> = {
  rs: { prefix: "", length: 24 },
  cli: { prefix: "", length: 32 },
  cgr: { prefix: "cgr_", length: 16 },
};
const DEFAULT_ID_SHAPE = { prefix: "", length: 22 };

// Like the real list endpoints: with page or per_page in the query, a list comes back
// one page at a time. Page is 0-based and per_page defaults to 50 (unverified on the
// live API, which is why the tests use more items than that).
function pageOf(method: HttpMethod, rawPath: string, result: unknown): unknown {
  const params = new URLSearchParams(rawPath.split("?")[1] ?? "");
  if (method !== "GET" || (!params.has("page") && !params.has("per_page"))) return result;
  const page = Number(params.get("page") ?? 0);
  const perPage = Number(params.get("per_page") ?? 50);
  const slice = (items: unknown[]) => items.slice(page * perPage, (page + 1) * perPage);
  if (Array.isArray(result)) return slice(result);
  if (result && typeof result === "object") {
    const key = Object.keys(result).find((name) => Array.isArray((result as Record<string, unknown>)[name]));
    if (key) return { ...result, [key]: slice((result as Record<string, unknown[]>)[key]) };
  }
  return result;
}

// In-memory stand-in for the Management API routes setup uses. It enforces
// the rules setup depends on: duplicate resource server identifiers and
// portal slugs conflict, and a client secret is only returned by the
// single-client read.
export class FakeTenant implements ApiClient {
  readonly clients = new Map<string, Record<string, any>>();
  readonly grants = new Map<string, Record<string, any>>();
  readonly resourceServers = new Map<string, Record<string, any>>();
  readonly vaults = new Map<string, Record<string, any>>();
  readonly forms = new Map<string, Record<string, any>>();
  readonly portals = new Map<string, Record<string, any>>();
  readonly users = new Map<string, Record<string, any>>();
  readonly organizations = new Map<string, Record<string, any>>();
  // organization id -> user ids that are members
  readonly members = new Map<string, Set<string>>();
  // organization id -> connection ids enabled for it
  readonly orgConnections = new Map<string, string[]>();
  // "orgId:userId" -> role ids assigned in that organization
  readonly orgRoles = new Map<string, string[]>();
  readonly roles = new Map<string, Record<string, any>>();
  readonly connections = new Map<string, Record<string, any>>();
  customDomain?: { domain: string; status: string };
  private denials = new Map<string, string>();
  readonly calls: string[] = [];
  private nextId = 1;
  private failures = new Map<string, number>();

  // Makes the next call to "METHOD path" fail with the given status, once.
  failNext(key: string, status = 500): void {
    this.failures.set(key, status);
  }

  // Every call to "METHOD path" fails with the Management API's scope error.
  deny(key: string, message: string): void {
    this.denials.set(key, message);
  }

  async request<T = unknown>(method: HttpMethod, path: string, body?: unknown): Promise<T> {
    const key = `${method} ${path.split("?")[0]}`;
    this.calls.push(`${method} ${path}`);
    const denial = this.denials.get(key);
    if (denial !== undefined) {
      throw new ApiError(403, "insufficient_scope", denial, { statusCode: 403, message: denial, errorCode: "insufficient_scope" });
    }
    // Like the real API, a create with no body is a validation error, not a
    // success. Preflight relies on this to tell a present scope from a missing one.
    if (method === "POST" && body !== undefined && Object.keys(body as object).length === 0) {
      throw toApiError(400, { message: "Payload validation error: 'Missing required property'" });
    }
    const failStatus = this.failures.get(key);
    if (failStatus !== undefined) {
      this.failures.delete(key);
      throw toApiError(failStatus, { message: "simulated failure" });
    }
    return pageOf(method, path, this.route(method, path, body)) as T;
  }

  private id(prefix: string): string {
    this.nextId += 1;
    const shape = ID_SHAPES[prefix] ?? { ...DEFAULT_ID_SHAPE, prefix: `${prefix}_` };
    return shape.prefix + String(this.nextId).padStart(shape.length, "0");
  }

  private route(method: HttpMethod, rawPath: string, body: any): unknown {
    const [path, query = ""] = rawPath.split("?");
    const parts = path.split("/").map((part) => decodeURIComponent(part));

    if (method === "GET" && path === "resource-servers") {
      return { resource_servers: [...this.resourceServers.values()] };
    }
    if (method === "POST" && path === "resource-servers") {
      if ([...this.resourceServers.values()].some((server) => server.identifier === body.identifier)) {
        throw toApiError(409, { message: "The resource server identifier already exists" });
      }
      const server = { id: this.id("rs"), ...body };
      this.resourceServers.set(server.id, server);
      return server;
    }

    if (method === "GET" && path === "clients") {
      return { clients: [...this.clients.values()].map(({ client_secret: _secret, ...rest }) => rest) };
    }
    if (method === "POST" && path === "clients") {
      const client = {
        client_id: this.id("cli"),
        client_secret: String(this.nextId).padStart(64, "s"),
        ...body,
      };
      this.clients.set(client.client_id, client);
      return client;
    }
    if (parts[0] === "clients" && parts.length === 2) {
      const client = this.clients.get(parts[1]);
      if (!client) throw toApiError(404, { message: "not found" });
      if (method === "GET") {
        return query.includes("client_secret") ? { client_id: client.client_id, client_secret: client.client_secret } : client;
      }
      if (method === "PATCH") {
        Object.assign(client, body);
        return client;
      }
    }

    if (method === "GET" && path === "client-grants") {
      const clientId = new URLSearchParams(query).get("client_id");
      return [...this.grants.values()].filter((grant) => grant.client_id === clientId);
    }
    if (method === "POST" && path === "client-grants") {
      const grant = { id: this.id("cgr"), ...body };
      this.grants.set(grant.id, grant);
      return grant;
    }
    if (method === "DELETE" && parts[0] === "client-grants" && parts.length === 2) {
      if (!this.grants.has(parts[1])) throw toApiError(404, { message: "The grant does not exist" });
      this.grants.delete(parts[1]);
      return undefined;
    }
    if (method === "PATCH" && parts[0] === "client-grants" && parts.length === 2) {
      Object.assign(this.grants.get(parts[1]) ?? {}, body);
      return this.grants.get(parts[1]);
    }

    if (method === "GET" && path === "flows/vault/connections") {
      return { connections: [...this.vaults.values()] };
    }
    if (method === "POST" && path === "flows/vault/connections") {
      const vault = { id: this.id("ac"), name: body.name, setup: body.setup };
      this.vaults.set(vault.id, vault);
      return vault;
    }
    if (method === "PATCH" && parts[0] === "flows" && parts[1] === "vault" && parts[2] === "connections" && parts.length === 4) {
      const vault = this.vaults.get(parts[3]);
      if (!vault) throw toApiError(404, { message: "Connection not found" });
      vault.setup = body.setup;
      vault.minted_with_scopes = this.scopesFor(vault.setup.client_id);
      return vault;
    }

    if (method === "GET" && path === "forms") {
      return [...this.forms.values()];
    }
    if (method === "POST" && path === "forms/import") {
      const form = { id: this.id("ap"), name: body.form.name, document: body, flowId: this.id("af") };
      this.forms.set(form.id, form);
      return {
        form: { id: form.id, name: form.name },
        flows: { "#FLOW-1#": { id: form.flowId, name: "Flow" } },
      };
    }
    if (method === "GET" && parts[0] === "forms" && parts.length === 2) {
      if (!/^ap_[A-Za-z0-9]{22}$/.test(parts[1])) throw toApiError(400, { message: "The provided input does not satisfy the expected schema" });
      const form = this.forms.get(parts[1]);
      if (!form) throw toApiError(404, { message: "Form not found" });
      return { id: form.id, name: form.name, nodes: [{ type: "FLOW", config: { flow_id: form.flowId } }] };
    }
    if (method === "POST" && path === "forms/export") {
      const form = this.forms.get(body.form_id);
      if (!form) throw toApiError(404, { message: "Form not found" });
      return { form: { id: form.id, name: form.name }, flows: { "#FLOW-1#": { id: form.flowId, name: "Flow" } } };
    }

    if (method === "GET" && path === "portals") {
      return { portals: [...this.portals.values()].map(({ id, slug, name }) => ({ id, slug, name })) };
    }
    if (method === "GET" && parts[0] === "portals" && parts.length === 2) {
      const portal = this.portals.get(parts[1]);
      if (!portal) throw toApiError(404, { message: "not found" });
      return portal;
    }
    if (method === "PATCH" && parts[0] === "portals" && parts.length === 2) {
      const portal = this.portals.get(parts[1]);
      if (!portal) throw toApiError(404, { message: "not found" });
      Object.assign(portal, body);
      return portal;
    }
    if (method === "POST" && path === "portals") {
      if ([...this.portals.values()].some((portal) => portal.slug === body.slug)) {
        throw new ApiError(409, "portal_conflict", "Slug already in use", {});
      }
      const portal = { id: this.id("up"), ...body, client: { client_id: body.client.client_id } };
      this.portals.set(portal.id, portal);
      return portal;
    }

    if (method === "GET" && path === "custom-domains/default") {
      if (!this.customDomain) throw toApiError(404, { message: "Not Found" });
      return this.customDomain;
    }

    // The search matches a phrase anywhere in the email, like the real search.
    if (method === "GET" && path === "users") {
      const phrase = /^email:"(.*)"$/.exec(new URLSearchParams(query).get("q") ?? "")?.[1];
      const unescaped = phrase?.replace(/\\(.)/g, "$1") ?? "";
      return [...this.users.values()].filter((user) => String(user.email ?? "").includes(unescaped));
    }
    if (method === "GET" && parts[0] === "users" && parts.length === 2) {
      const user = this.users.get(parts[1]);
      if (!user) throw toApiError(404, { message: "The user does not exist." });
      return user;
    }
    if (method === "GET" && parts[0] === "users" && parts[2] === "organizations") {
      return [...this.organizations.values()].filter((org) => this.members.get(org.id)?.has(parts[1]));
    }
    if (method === "GET" && path === "organizations") {
      return [...this.organizations.values()];
    }
    if (method === "POST" && path === "organizations") {
      if ([...this.organizations.values()].some((org) => org.name === body.name)) {
        throw toApiError(409, { message: "The organization name is already in use." });
      }
      const org = { id: this.id("org"), name: body.name, display_name: body.display_name };
      this.organizations.set(org.id, org);
      return org;
    }
    // Like the real endpoint: pages are taken by checkpoint (take, from, and a next token), and
    // page and per_page are rejected.
    if (method === "GET" && parts[0] === "clients" && parts[2] === "connections") {
      const params = new URLSearchParams(query);
      if (params.has("page") || params.has("per_page")) {
        throw toApiError(400, {
          message: "Query validation error: 'Additional properties not allowed: per_page,page'.",
          errorCode: "invalid_query_string",
        });
      }
      const all = [...this.connections.values()];
      const take = Number(params.get("take") ?? 50);
      const start = params.has("from") ? all.findIndex((connection) => connection.id === params.get("from")) : 0;
      const next = all[start + take];
      return { connections: all.slice(start, start + take), ...(next ? { next: next.id } : {}) };
    }
    if (method === "GET" && path === "connections") {
      return [...this.connections.values()];
    }
    if (method === "POST" && parts[0] === "organizations" && parts[2] === "enabled_connections") {
      const list = this.orgConnections.get(parts[1]) ?? [];
      this.orgConnections.set(parts[1], [...list, body.connection_id]);
      return {};
    }
    if (method === "POST" && parts[0] === "organizations" && parts[2] === "members" && parts.length === 3) {
      const members = this.members.get(parts[1]) ?? new Set<string>();
      for (const userId of body.members) members.add(userId);
      this.members.set(parts[1], members);
      return {};
    }
    if (method === "GET" && parts[0] === "organizations" && parts[2] === "enabled_connections") {
      return (this.orgConnections.get(parts[1]) ?? []).map((connection_id) => ({ connection_id }));
    }
    if (parts[0] === "organizations" && parts[2] === "members" && parts[4] === "roles") {
      const key = `${parts[1]}:${parts[3]}`;
      if (method === "GET") {
        return (this.orgRoles.get(key) ?? []).map((id) => this.roles.get(id)).filter(Boolean);
      }
      if (method === "POST") {
        this.orgRoles.set(key, [...(this.orgRoles.get(key) ?? []), ...body.roles]);
        return {};
      }
    }
    if (method === "POST" && path === "roles") {
      const role = { id: this.id("rol"), name: body.name, description: body.description, permissions: [] as string[] };
      this.roles.set(role.id, role);
      return role;
    }
    if (method === "POST" && parts[0] === "roles" && parts[2] === "permissions") {
      const role = this.roles.get(parts[1]);
      if (!role) throw toApiError(404, { message: "The role does not exist" });
      role.permissions = [...(role.permissions ?? []), ...body.permissions.map((permission: { permission_name: string }) => permission.permission_name)];
      return {};
    }
    if (method === "GET" && parts[0] === "roles" && parts[2] === "permissions") {
      const role = this.roles.get(parts[1]);
      return { permissions: (role?.permissions ?? []).map((permission_name: string) => ({ permission_name })) };
    }
    if (method === "GET" && path === "roles") {
      return [...this.roles.values()];
    }

    throw toApiError(404, { message: `FakeTenant has no route for ${method} ${path}` });
  }

  // Scopes the vault connection's token would carry, taken from its grant.
  private scopesFor(clientId: string): string[] {
    const grant = [...this.grants.values()].find((g) => g.client_id === clientId && g.audience?.endsWith("/api/v2/"));
    return grant?.scope ?? [];
  }
}
