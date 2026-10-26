import { strict as assert } from "node:assert";
import { test } from "node:test";

import { ApiError, HttpMethod, toApiError } from "../src/api.js";
import type { Prompter } from "../src/prompt.js";
import { BusinessAccessError } from "../src/setup/business-types.js";
import { MY_ORGANIZATION_SCOPES } from "../src/setup/payload.js";
import { chooseTestLink, ORGANIZATION_PLACEHOLDER, resolveBusinessAccess } from "../src/setup/business.js";
import { Ui } from "../src/ui.js";
import { FakeTenant } from "./fake-tenant.js";
import { captureOutput } from "./capture-output.js";

// Answers the business questions from scripts. choose() takes the option whose
// label contains the given text, which keeps the script readable.
function scripted(answers: { ask?: string[]; choose?: string[]; confirm?: boolean[] }): Prompter {
  const asks = [...(answers.ask ?? [])];
  const chooses = [...(answers.choose ?? [])];
  const confirms = [...(answers.confirm ?? [])];
  return {
    async ask() {
      const next = asks.shift();
      if (next === undefined) throw new Error("unexpected question");
      return next;
    },
    async confirm() {
      const next = confirms.shift();
      if (next === undefined) throw new Error("unexpected confirmation");
      return next;
    },
    async checklist(): Promise<never> {
      throw new Error("unexpected question");
    },
    async choose<T>(_question: string, options: { label: string; value: T }[]): Promise<T> {
      const label = chooses.shift();
      const match = options.find((option) => option.label.includes(label ?? "\u0000"));
      if (!match) throw new Error(`no option matching "${label}" in: ${options.map((o) => o.label).join(" | ")}`);
      return match.value;
    },
  };
}

function ui(): Ui {
  return new Ui(captureOutput().stream, false, true);
}

function seedUser(tenant: FakeTenant, id: string, email: string): void {
  tenant.users.set(id, { user_id: id, email, name: email.split("@")[0] });
}

// A tenant where enabling a connection on the new organization conflicts.
class ConflictingConnectionTenant extends FakeTenant {
  override async request<T>(method: HttpMethod, path: string, body?: unknown): Promise<T> {
    if (method === "POST" && path.endsWith("/enabled_connections")) {
      throw toApiError(409, { message: "The connection is already enabled." });
    }
    return super.request<T>(method, path, body);
  }
}

test("a conflict after the organization exists is not treated as a name clash", async () => {
  const tenant = new ConflictingConnectionTenant();
  seedUser(tenant, "auth0|a", "jane@acme.com");
  tenant.connections.set("con_db", { id: "con_db", name: "Username-Password-Authentication", strategy: "auth0" });

  await assert.rejects(
    resolveBusinessAccess(
      { userQuery: "auth0|a" },
      {
        api: tenant,
        ui: ui(),
        prompt: scripted({ ask: ["Acme Corp", "acme-corp"], confirm: [true], choose: ["None of these"] }),
        interactive: true,
        tenantDomain: "acme.us.auth0.com",
      },
    ),
    ApiError,
  );
  assert.equal(tenant.organizations.size, 1);
});

test("skipping the test user links to the organization ID the user types", async () => {
  const link = await chooseTestLink(
    {},
    { api: new FakeTenant(), ui: ui(), prompt: scripted({ ask: ["", "org_42"] }), interactive: true, tenantDomain: "acme.us.auth0.com" },
  );
  assert.deepEqual(link, { organizationId: "org_42", isPlaceholder: false });
});

test("skipping the test user and the organization keeps the placeholder in the link", async () => {
  const link = await chooseTestLink(
    {},
    { api: new FakeTenant(), ui: ui(), prompt: scripted({ ask: ["", ORGANIZATION_PLACEHOLDER] }), interactive: true, tenantDomain: "acme.us.auth0.com" },
  );
  assert.deepEqual(link, { organizationId: ORGANIZATION_PLACEHOLDER, isPlaceholder: true });
});

test("a chosen test user links to their organization and keeps their access", async () => {
  const tenant = new FakeTenant();
  seedUser(tenant, "auth0|a", "jane@acme.com");
  tenant.organizations.set("org_1", { id: "org_1", name: "acme" });
  tenant.members.set("org_1", new Set(["auth0|a"]));
  tenant.orgConnections.set("org_1", ["con_db"]);

  const link = await chooseTestLink(
    { userQuery: "jane@acme.com" },
    { api: tenant, ui: ui(), prompt: scripted({}), interactive: false, tenantDomain: "acme.us.auth0.com" },
  );
  assert.equal(link.isPlaceholder, false);
  assert.equal(link.organizationId, "org_1");
  assert.equal(link.access?.userId, "auth0|a");
});

test("an email lookup keeps only the exact address, not a longer one that contains it", async () => {
  const tenant = new FakeTenant();
  seedUser(tenant, "auth0|a", "jane@acme.com");
  seedUser(tenant, "auth0|b", "jane@acme.com.au");
  tenant.organizations.set("org_1", { id: "org_1", name: "acme" });
  tenant.members.set("org_1", new Set(["auth0|a"]));
  tenant.orgConnections.set("org_1", ["con_db"]);

  const access = await resolveBusinessAccess(
    { userQuery: "jane@acme.com" },
    { api: tenant, ui: ui(), prompt: scripted({}), interactive: false, tenantDomain: "acme.us.auth0.com" },
  );
  assert.equal(access.userId, "auth0|a");
});

test("an email that matches several users lets the user pick one", async () => {
  const tenant = new FakeTenant();
  seedUser(tenant, "auth0|a", "jane@acme.com");
  seedUser(tenant, "google-oauth2|b", "jane@acme.com");
  tenant.organizations.set("org_1", { id: "org_1", name: "acme" });
  tenant.members.set("org_1", new Set(["google-oauth2|b"]));
  tenant.orgConnections.set("org_1", ["con_db"]);

  const access = await resolveBusinessAccess(
    { userQuery: "jane@acme.com" },
    { api: tenant, ui: ui(), prompt: scripted({ confirm: [false], choose: ["google-oauth2|b"] }), interactive: true, tenantDomain: "acme.us.auth0.com" },
  );
  assert.equal(access.userId, "google-oauth2|b");
});

test("from a duplicate-email list, trying another email or ID looks the user up again", async () => {
  const tenant = new FakeTenant();
  seedUser(tenant, "auth0|a", "jane@acme.com");
  seedUser(tenant, "auth0|b", "jane@acme.com");
  seedUser(tenant, "auth0|other", "other@acme.com");
  tenant.organizations.set("org_1", { id: "org_1", name: "acme" });
  tenant.members.set("org_1", new Set(["auth0|other"]));
  tenant.orgConnections.set("org_1", ["con_db"]);

  const access = await resolveBusinessAccess(
    { userQuery: "jane@acme.com" },
    {
      api: tenant,
      ui: ui(),
      prompt: scripted({ confirm: [false], ask: ["auth0|other"], choose: ["Try another", "auth0|other"] }),
      interactive: true,
      tenantDomain: "acme.us.auth0.com",
    },
  );
  assert.equal(access.userId, "auth0|other");
});

test("an unknown email asks again instead of stopping", async () => {
  const tenant = new FakeTenant();
  seedUser(tenant, "auth0|a", "jane@acme.com");
  tenant.organizations.set("org_1", { id: "org_1", name: "acme" });
  tenant.members.set("org_1", new Set(["auth0|a"]));
  tenant.orgConnections.set("org_1", ["con_db"]);

  const access = await resolveBusinessAccess(
    { userQuery: "nobody@acme.com" },
    { api: tenant, ui: ui(), prompt: scripted({ confirm: [false], ask: ["jane@acme.com"] }), interactive: true, tenantDomain: "acme.us.auth0.com" },
  );
  assert.equal(access.userId, "auth0|a");
});

test("without a terminal an email that matches several users stops with the IDs to use", async () => {
  const tenant = new FakeTenant();
  seedUser(tenant, "auth0|a", "jane@acme.com");
  seedUser(tenant, "auth0|b", "jane@acme.com");

  await assert.rejects(
    resolveBusinessAccess({ userQuery: "jane@acme.com" }, { api: tenant, ui: ui(), prompt: scripted({}), interactive: false, tenantDomain: "acme.us.auth0.com" }),
    (error: unknown) => {
      assert.ok(error instanceof BusinessAccessError);
      assert.match(error.fixes.join(" "), /auth0\|a, auth0\|b/);
      return true;
    },
  );
});

test("a user with no organization can create one, with a database connection and themselves as a member", async () => {
  const tenant = new FakeTenant();
  seedUser(tenant, "auth0|a", "jane@acme.com");
  tenant.connections.set("con_db", { id: "con_db", name: "Username-Password-Authentication", strategy: "auth0" });

  const access = await resolveBusinessAccess(
    { userQuery: "jane@acme.com" },
    {
      api: tenant,
      ui: ui(),
      prompt: scripted({ ask: ["Acme Admins", "acme-admins"], choose: ["None of these"], confirm: [true, false] }),
      interactive: true,
      tenantDomain: "acme.us.auth0.com",
    },
  );

  const org = [...tenant.organizations.values()].find((o) => o.display_name === "Acme Admins");
  assert.ok(org, "the organization was created");
  assert.equal(access.organizationId, org.id);
  assert.deepEqual([...(tenant.orgConnections.get(org.id) ?? [])], ["con_db"]);
  assert.ok(tenant.members.get(org.id)?.has("auth0|a"));
});

test("declining the organization confirmation creates nothing", async () => {
  const tenant = new FakeTenant();
  seedUser(tenant, "auth0|a", "jane@acme.com");
  tenant.connections.set("con_db", { id: "con_db", name: "Username-Password-Authentication", strategy: "auth0" });

  await assert.rejects(
    resolveBusinessAccess(
      { userQuery: "jane@acme.com" },
      {
        api: tenant,
        ui: ui(),
        prompt: scripted({ ask: ["Acme Admins", "acme-admins"], choose: ["None of these"], confirm: [false] }),
        interactive: true,
        tenantDomain: "acme.us.auth0.com",
      },
    ),
    BusinessAccessError,
  );
  assert.equal(tenant.organizations.size, 0);
});

test("a taken organization name asks for another name, with a suggestion", async () => {
  const tenant = new FakeTenant();
  seedUser(tenant, "auth0|a", "jane@acme.com");
  tenant.organizations.set("org_taken", { id: "org_taken", name: "acme-admins", display_name: "Acme" });
  tenant.connections.set("con_db", { id: "con_db", name: "Username-Password-Authentication", strategy: "auth0" });

  const access = await resolveBusinessAccess(
    { userQuery: "jane@acme.com" },
    {
      api: tenant,
      ui: ui(),
      prompt: scripted({ ask: ["Acme Admins", "acme-admins", "acme-admins-2"], choose: ["None of these"], confirm: [true, true, false] }),
      interactive: true,
      tenantDomain: "acme.us.auth0.com",
    },
  );

  const created = tenant.organizations.get(access.organizationId);
  assert.equal(created?.name, "acme-admins-2");
  assert.equal(created?.display_name, "Acme Admins");
});

test("an organization name with invalid characters asks again", async () => {
  const tenant = new FakeTenant();
  seedUser(tenant, "auth0|a", "jane@acme.com");
  tenant.connections.set("con_db", { id: "con_db", name: "Username-Password-Authentication", strategy: "auth0" });

  const access = await resolveBusinessAccess(
    { userQuery: "jane@acme.com" },
    {
      api: tenant,
      ui: ui(),
      prompt: scripted({ ask: ["Acme Admins", "Acme Admins!", "acme-admins"], choose: ["None of these"], confirm: [true, false] }),
      interactive: true,
      tenantDomain: "acme.us.auth0.com",
    },
  );
  assert.equal(tenant.organizations.get(access.organizationId)?.name, "acme-admins");
});

test("a user ID that matches nobody asks again instead of stopping", async () => {
  const tenant = new FakeTenant();
  seedUser(tenant, "auth0|a", "jane@acme.com");
  tenant.organizations.set("org_1", { id: "org_1", name: "acme" });
  tenant.members.set("org_1", new Set(["auth0|a"]));
  tenant.orgConnections.set("org_1", ["con_db"]);

  const access = await resolveBusinessAccess(
    { userQuery: "auth0|nobody" },
    { api: tenant, ui: ui(), prompt: scripted({ confirm: [false], ask: ["auth0|a"] }), interactive: true, tenantDomain: "acme.us.auth0.com" },
  );
  assert.equal(access.userId, "auth0|a");
});

test("a user in several organizations can pick the one the portal is for", async () => {
  const tenant = new FakeTenant();
  seedUser(tenant, "auth0|a", "jane@acme.com");
  for (const [id, name] of [["org_1", "acme"], ["org_2", "labs"]] as const) {
    tenant.organizations.set(id, { id, name });
    tenant.members.set(id, new Set(["auth0|a"]));
    tenant.orgConnections.set(id, ["con_db"]);
  }

  const access = await resolveBusinessAccess(
    { userQuery: "auth0|a" },
    { api: tenant, ui: ui(), prompt: scripted({ confirm: [false], choose: ["labs"] }), interactive: true, tenantDomain: "acme.us.auth0.com" },
  );
  assert.equal(access.organizationId, "org_2");
});

test("a user in no organization can join an existing one after confirmation", async () => {
  const tenant = new FakeTenant();
  seedUser(tenant, "auth0|a", "jane@acme.com");
  tenant.organizations.set("org_1", { id: "org_1", name: "acme" });
  tenant.orgConnections.set("org_1", ["con_db"]);

  const access = await resolveBusinessAccess(
    { userQuery: "auth0|a" },
    { api: tenant, ui: ui(), prompt: scripted({ choose: ["acme"], confirm: [true, false] }), interactive: true, tenantDomain: "acme.us.auth0.com" },
  );
  assert.equal(access.organizationId, "org_1");
  assert.ok(tenant.members.get("org_1")?.has("auth0|a"), "the user was added as a member");
});

test("declining to add a user to an organization adds nobody", async () => {
  const tenant = new FakeTenant();
  seedUser(tenant, "auth0|a", "jane@acme.com");
  tenant.organizations.set("org_1", { id: "org_1", name: "acme" });
  tenant.orgConnections.set("org_1", ["con_db"]);

  await assert.rejects(
    resolveBusinessAccess(
      { userQuery: "auth0|a" },
      { api: tenant, ui: ui(), prompt: scripted({ choose: ["acme"], confirm: [false] }), interactive: true, tenantDomain: "acme.us.auth0.com" },
    ),
    BusinessAccessError,
  );
  assert.equal(tenant.members.get("org_1")?.has("auth0|a") ?? false, false);
});

test("a user with no organization role can be given one after confirmation", async () => {
  const tenant = new FakeTenant();
  seedUser(tenant, "auth0|a", "jane@acme.com");
  tenant.organizations.set("org_1", { id: "org_1", name: "acme" });
  tenant.members.set("org_1", new Set(["auth0|a"]));
  tenant.orgConnections.set("org_1", ["con_db"]);
  tenant.roles.set("rol_admin", { id: "rol_admin", name: "Organization - Admin", permissions: MY_ORGANIZATION_SCOPES });

  const access = await resolveBusinessAccess(
    { userQuery: "auth0|a" },
    { api: tenant, ui: ui(), prompt: scripted({ choose: ["Organization - Admin"], confirm: [true] }), interactive: true, tenantDomain: "acme.us.auth0.com" },
  );
  assert.equal(access.hasOrganizationRole, true);
  assert.deepEqual(tenant.orgRoles.get("org_1:auth0|a"), ["rol_admin"]);
});

test("declining the role assignment keeps going, and the user has no role", async () => {
  const tenant = new FakeTenant();
  seedUser(tenant, "auth0|a", "jane@acme.com");
  tenant.organizations.set("org_1", { id: "org_1", name: "acme" });
  tenant.members.set("org_1", new Set(["auth0|a"]));
  tenant.orgConnections.set("org_1", ["con_db"]);
  tenant.roles.set("rol_admin", { id: "rol_admin", name: "Organization - Admin", permissions: MY_ORGANIZATION_SCOPES });

  const access = await resolveBusinessAccess(
    { userQuery: "auth0|a" },
    { api: tenant, ui: ui(), prompt: scripted({ confirm: [false] }), interactive: true, tenantDomain: "acme.us.auth0.com" },
  );
  assert.equal(access.hasOrganizationRole, false);
  assert.equal(tenant.orgRoles.get("org_1:auth0|a"), undefined);
});

test("a role that lacks an organization scope is a warning that names the missing scopes", async () => {
  const tenant = new FakeTenant();
  seedUser(tenant, "auth0|a", "jane@acme.com");
  tenant.organizations.set("org_1", { id: "org_1", name: "acme" });
  tenant.members.set("org_1", new Set(["auth0|a"]));
  tenant.orgConnections.set("org_1", ["con_db"]);
  tenant.roles.set("rol_reader", {
    id: "rol_reader",
    name: "Organization - Reader",
    permissions: ["read:my_org:details", "read:my_org:domains"],
  });
  tenant.orgRoles.set("org_1:auth0|a", ["rol_reader"]);

  const captured = captureOutput();
  const access = await resolveBusinessAccess(
    { userQuery: "auth0|a" },
    { api: tenant, ui: new Ui(captured.stream, false, false), prompt: scripted({}), interactive: false, tenantDomain: "acme.us.auth0.com" },
  );
  assert.equal(access.hasOrganizationRole, false);
  assert.match(captured.text(), /update:my_org:details/);
  assert.match(captured.text(), /delete:my_org:domains/);
});

test("a role with read but no write scopes warns that changes cannot be saved", async () => {
  const tenant = new FakeTenant();
  seedUser(tenant, "auth0|a", "jane@acme.com");
  tenant.organizations.set("org_1", { id: "org_1", name: "acme" });
  tenant.members.set("org_1", new Set(["auth0|a"]));
  tenant.orgConnections.set("org_1", ["con_db"]);
  const reads = MY_ORGANIZATION_SCOPES.filter((scope) => scope.startsWith("read:"));
  tenant.roles.set("rol_viewer", { id: "rol_viewer", name: "Organization - Viewer", permissions: reads });
  tenant.orgRoles.set("org_1:auth0|a", ["rol_viewer"]);

  const captured = captureOutput();
  await resolveBusinessAccess(
    { userQuery: "auth0|a" },
    { api: tenant, ui: new Ui(captured.stream, false, false), prompt: scripted({}), interactive: false, tenantDomain: "acme.us.auth0.com" },
  );
  assert.match(captured.text(), /lacks write permissions/);
  assert.doesNotMatch(captured.text(), /lacks read permissions/);
});

test("with no role that grants the missing permissions, the offer is to create the portal role, and declining keeps the user without one", async () => {
  const tenant = new FakeTenant();
  seedUser(tenant, "auth0|a", "jane@acme.com");
  tenant.organizations.set("org_1", { id: "org_1", name: "acme" });
  tenant.members.set("org_1", new Set(["auth0|a"]));
  tenant.orgConnections.set("org_1", ["con_db"]);
  tenant.roles.set("rol_reader", {
    id: "rol_reader",
    name: "Organization - Reader",
    permissions: ["read:my_org:details", "read:my_org:domains"],
  });

  const access = await resolveBusinessAccess(
    { userQuery: "auth0|a" },
    { api: tenant, ui: ui(), prompt: scripted({ confirm: [false] }), interactive: true, tenantDomain: "acme.us.auth0.com" },
  );
  assert.equal(access.hasOrganizationRole, false);
  assert.equal(tenant.orgRoles.get("org_1:auth0|a"), undefined);
  assert.equal([...tenant.roles.values()].some((role) => role.name === "Universal Portals - Admin"), false);
});

test("accepting the offer creates Universal Portals - Admin with only the portal permissions and assigns it", async () => {
  const tenant = new FakeTenant();
  seedUser(tenant, "auth0|a", "jane@acme.com");
  tenant.organizations.set("org_1", { id: "org_1", name: "acme" });
  tenant.members.set("org_1", new Set(["auth0|a"]));
  tenant.orgConnections.set("org_1", ["con_db"]);

  const access = await resolveBusinessAccess(
    { userQuery: "auth0|a" },
    { api: tenant, ui: ui(), prompt: scripted({ confirm: [true] }), interactive: true, tenantDomain: "acme.us.auth0.com" },
  );
  const role = [...tenant.roles.values()].find((candidate) => candidate.name === "Universal Portals - Admin");
  assert.equal(access.hasOrganizationRole, true);
  assert.ok(role, "the role was created");
  assert.deepEqual([...role.permissions].sort(), [...MY_ORGANIZATION_SCOPES].sort());
  assert.deepEqual(tenant.orgRoles.get("org_1:auth0|a"), [role.id]);
});

test("a role that grants every organization scope is accepted without an offer", async () => {
  const tenant = new FakeTenant();
  seedUser(tenant, "auth0|a", "jane@acme.com");
  tenant.organizations.set("org_1", { id: "org_1", name: "acme" });
  tenant.members.set("org_1", new Set(["auth0|a"]));
  tenant.orgConnections.set("org_1", ["con_db"]);
  tenant.roles.set("rol_admin", { id: "rol_admin", name: "Organization - Admin", permissions: MY_ORGANIZATION_SCOPES });
  tenant.orgRoles.set("org_1:auth0|a", ["rol_admin"]);

  const access = await resolveBusinessAccess(
    { userQuery: "auth0|a" },
    { api: tenant, ui: ui(), prompt: scripted({}), interactive: true, tenantDomain: "acme.us.auth0.com" },
  );
  assert.equal(access.hasOrganizationRole, true);
});

test("without a terminal, a user who matches nobody stops and says why", async () => {
  const tenant = new FakeTenant();
  await assert.rejects(
    resolveBusinessAccess({ userQuery: "nobody@acme.com" }, { api: tenant, ui: ui(), prompt: scripted({}), interactive: false, tenantDomain: "acme.us.auth0.com" }),
    (error: unknown) => {
      assert.ok(error instanceof BusinessAccessError);
      assert.match(error.message, /No user matches "nobody@acme.com"/);
      return true;
    },
  );
});

test("without a terminal, an organization the user is not in stops with the command to add them", async () => {
  const tenant = new FakeTenant();
  seedUser(tenant, "auth0|a", "jane@acme.com");
  tenant.organizations.set("org_1", { id: "org_1", name: "acme" });
  tenant.members.set("org_1", new Set(["auth0|a"]));
  tenant.organizations.set("org_2", { id: "org_2", name: "labs" });

  await assert.rejects(
    resolveBusinessAccess(
      { userQuery: "auth0|a", organization: "org_2" },
      { api: tenant, ui: ui(), prompt: scripted({}), interactive: false, tenantDomain: "acme.us.auth0.com" },
    ),
    (error: unknown) => {
      assert.ok(error instanceof BusinessAccessError);
      assert.match(error.fixes.join(" "), /organizations\/org_2\/members/);
      return true;
    },
  );
});
