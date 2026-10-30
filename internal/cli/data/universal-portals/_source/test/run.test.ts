import { strict as assert } from "node:assert";
import { test } from "node:test";

import { CancelledError, type Prompter } from "../src/prompt.js";
import { Ui } from "../src/ui.js";
import { BusinessAccessError } from "../src/setup/business-types.js";
import { MissingScopesError, runSetup, SetupDeps, SetupOptions } from "../src/setup/run.js";
import { FORMS_VAULT_NAME } from "../src/setup/vault.js";
import { FORMS_VAULT_SCOPES, MY_ACCOUNT_SCOPES, MY_ORGANIZATION_SCOPES } from "../src/setup/payload.js";
import { CLIENT_SECRET_PLACEHOLDER, planWrites } from "../src/dry-run.js";
import { FakeTenant } from "./fake-tenant.js";
import { captureOutput } from "./capture-output.js";

const DOMAIN = "acme.us.auth0.com";

function harness(
  tenant: FakeTenant,
  overrides: Partial<SetupDeps> = {},
): { deps: SetupDeps; output: () => string } {
  const captured = captureOutput();
  const ui = new Ui(captured.stream, false, false);
  const prompt: Prompter = {
    async ask(_question, defaultValue) {
      return defaultValue ?? "";
    },
    async confirm() {
      return false;
    },
    async checklist(): Promise<never> {
      throw new Error("unexpected question");
    },
    async choose<T>(_question: string, options: { value: T }[]): Promise<T> {
      return options[0].value;
    },
  };
  return {
    deps: { api: tenant, ui, prompt, interactive: false, tenantDomain: DOMAIN, ...overrides },
    output: () => captured.text(),
  };
}

function seedBusinessUser(tenant: FakeTenant, options: { withOrg: boolean; withRole: boolean }): void {
  tenant.users.set("auth0|user1", { user_id: "auth0|user1", email: "jane@acme.com", name: "Jane" });
  tenant.organizations.set("org_1", { id: "org_1", name: "acme-corp" });
  tenant.orgConnections.set("org_1", ["con_db"]);
  tenant.roles.set("rol_admin", { id: "rol_admin", name: "Organization - Admin", permissions: MY_ORGANIZATION_SCOPES });
  if (options.withOrg) tenant.members.set("org_1", new Set(["auth0|user1"]));
  if (options.withRole) tenant.orgRoles.set("org_1:auth0|user1", ["rol_admin"]);
}

const CONSUMER: SetupOptions = {
  template: "consumer",
  name: "Acme Account",
  slug: "acme-account",
};

test("a consumer portal is created end to end and the output never shows a full secret", async () => {
  const tenant = new FakeTenant();
  const { deps, output } = harness(tenant);
  const result = await runSetup(CONSUMER, deps);

  assert.equal(result.portal.status, "created");
  assert.equal(result.portal.url, `https://${DOMAIN}/portals/acme-account`);
  assert.equal(tenant.portals.size, 1);
  assert.equal(tenant.forms.size, 3);
  assert.equal(tenant.vaults.size, 1);

  const secret = [...tenant.clients.values()].find((client) => client.name?.startsWith("Acme Account"))?.client_secret;
  assert.ok(secret);
  assert.ok(!output().includes(secret), "the full client secret must never be printed");
});

test("a consumer portal is linked to a client with only the My Account policy and no client credentials", async () => {
  const tenant = new FakeTenant();
  await runSetup(CONSUMER, harness(tenant).deps);

  const appClient = [...tenant.clients.values()].find((client) => client.name?.startsWith("Acme Account"));
  assert.ok(appClient);
  assert.ok(!appClient.grant_types.includes("client_credentials"), "the portal app must not have client_credentials");
  assert.equal(appClient.refresh_token.rotation_type, "non-rotating");
  assert.equal(appClient.refresh_token.idle_token_lifetime, 3600);
  assert.equal(appClient.refresh_token.token_lifetime, 86400);
  assert.deepEqual(
    appClient.refresh_token.policies.map((policy: { audience: string }) => policy.audience),
    [`https://${DOMAIN}/me/`],
  );
});

test("a rerun reuses every resource and creates nothing", async () => {
  const tenant = new FakeTenant();
  await runSetup(CONSUMER, harness(tenant).deps);
  const before = { clients: tenant.clients.size, grants: tenant.grants.size, forms: tenant.forms.size, portals: tenant.portals.size };

  const rerun = await runSetup(CONSUMER, harness(tenant).deps);

  assert.equal(rerun.portal.status, "reused");
  assert.equal(rerun.application.status, "reused");
  assert.equal(rerun.vault.status, "reused");
  assert.deepEqual(
    Object.values(rerun.forms).map((form) => form.status),
    ["reused", "reused", "reused"],
  );
  assert.deepEqual(
    { clients: tenant.clients.size, grants: tenant.grants.size, forms: tenant.forms.size, portals: tenant.portals.size },
    before,
  );
});

test("a failure partway through is recovered by rerunning the same command", async () => {
  const tenant = new FakeTenant();
  tenant.failNext("POST forms/import", 500);

  await assert.rejects(runSetup(CONSUMER, harness(tenant).deps), /simulated failure/);
  assert.equal(tenant.portals.size, 0, "the portal is the last step, so it must not exist yet");

  const rerun = await runSetup(CONSUMER, harness(tenant).deps);
  assert.equal(rerun.portal.status, "created");
  assert.equal(rerun.application.status, "reused");
  assert.equal(tenant.clients.size, 2, "one portal app and one forms vault app");
});

test("a taken slug fails without prompting when there is no terminal", async () => {
  const tenant = new FakeTenant();
  await runSetup(CONSUMER, harness(tenant).deps);
  tenant.clients.clear();

  await assert.rejects(runSetup(CONSUMER, harness(tenant).deps), /already used by another portal/);
});

test("a taken slug stops before the application, grants or vault are created", async () => {
  const tenant = new FakeTenant();
  tenant.portals.set("up_other", { id: "up_other", slug: "acme-account", name: "Other", client: { client_id: "cli_x" } });

  await assert.rejects(runSetup(CONSUMER, harness(tenant).deps), /already used by another portal/);
  assert.equal(tenant.clients.size, 0);
  assert.equal(tenant.grants.size, 0);
  assert.equal(tenant.vaults.size, 0);
});

test("an unknown test user stops before the application, grants or vault are created", async () => {
  const tenant = new FakeTenant();
  const business = { ...CONSUMER, template: "business" as const, name: "Acme Admin", slug: "acme-admin", userQuery: "nobody@acme.com" };

  await assert.rejects(runSetup(business, harness(tenant).deps), /No user matches/);
  assert.equal(tenant.clients.size, 0);
  assert.equal(tenant.grants.size, 0);
  assert.equal(tenant.vaults.size, 0);
  assert.equal(tenant.portals.size, 0);
});

test("a cancel at the test user question stops before anything is created", async () => {
  const tenant = new FakeTenant();
  const { deps } = harness(tenant, {
    interactive: true,
    prompt: {
      async ask() {
        throw new CancelledError();
      },
      async confirm() {
        return false;
      },
      async checklist(): Promise<never> {
        throw new Error("unexpected question");
      },
      async choose<T>(_question: string, options: { value: T }[]): Promise<T> {
        return options[0].value;
      },
    },
  });

  await assert.rejects(runSetup({ ...CONSUMER, template: "business", name: "Acme Admin", slug: "acme-admin" }, deps), CancelledError);
  assert.equal(tenant.clients.size, 0);
  assert.equal(tenant.portals.size, 0);
});

test("a taken slug is re-prompted when a terminal is available", async () => {
  const tenant = new FakeTenant();
  tenant.portals.set("up_other", { id: "up_other", slug: "acme-account", name: "Other", client: { client_id: "cli_x" } });

  const { deps } = harness(tenant, {
    interactive: true,
    prompt: {
      async ask() {
        return "acme-account-2";
      },
      async confirm() {
        return false;
      },
      async checklist(): Promise<never> {
        throw new Error("unexpected question");
      },
      async choose<T>(_q: string, options: { value: T }[]): Promise<T> {
        return options[0].value;
      },
    },
  });
  const result = await runSetup(CONSUMER, deps);
  assert.equal(result.portal.slug, "acme-account-2");
});

test("missing scopes stop setup before anything is created and name what to grant", async () => {
  const tenant = new FakeTenant();
  tenant.deny("POST portals", "Insufficient scope, expected any of: create:portals");
  tenant.deny("POST forms/import", "Insufficient scope, expected all of: create:forms,create:flows");

  await assert.rejects(runSetup(CONSUMER, harness(tenant).deps), (error: unknown) => {
    assert.ok(error instanceof MissingScopesError);
    assert.deepEqual(error.scopes, ["create:flows", "create:forms", "create:portals"]);
    return true;
  });
  assert.equal(tenant.clients.size, 0, "preflight must run before any create");
});

test("a custom domain is used for the portal URL and the back-channel logout", async () => {
  const tenant = new FakeTenant();
  tenant.customDomain = { domain: "auth.acme.com", status: "ready" };
  const result = await runSetup(CONSUMER, harness(tenant).deps);

  assert.equal(result.portal.url, "https://auth.acme.com/portals/acme-account");
  const appClient = [...tenant.clients.values()].find((client) => client.name?.startsWith("Acme Account"));
  assert.deepEqual(appClient?.callbacks, ["https://auth.acme.com/portals/auth/callback"]);
  assert.ok(appClient?.oidc_logout, "back-channel logout is set on a custom domain");
});

test("the forms vault is refreshed with setup when its grant gains a scope", async () => {
  const tenant = new FakeTenant();
  await runSetup(CONSUMER, harness(tenant).deps);

  const vaultApp = [...tenant.clients.values()].find((client) => client.client_metadata?.auth0_portals_role === "forms-vault");
  assert.ok(vaultApp);
  const grant = [...tenant.grants.values()].find((g) => g.client_id === vaultApp.client_id);
  assert.ok(grant);
  grant.scope = ["update:users"];

  const rerun = await runSetup(CONSUMER, harness(tenant).deps);
  const vault = [...tenant.vaults.values()].find((v) => v.name === FORMS_VAULT_NAME);
  assert.equal(rerun.vault.status, "reused");
  assert.deepEqual(vault?.minted_with_scopes, FORMS_VAULT_SCOPES);
});

test("business setup creates the portal with the organization in its URL", async () => {
  const tenant = new FakeTenant();
  seedBusinessUser(tenant, { withOrg: true, withRole: true });
  const result = await runSetup(
    { ...CONSUMER, template: "business", name: "Acme Admin", slug: "acme-admin", userQuery: "jane@acme.com" },
    harness(tenant).deps,
  );

  assert.equal(result.portal.url, `https://${DOMAIN}/portals/acme-admin?organization=org_1`);
  assert.equal(result.organization?.hasRole, true);
  const appClient = [...tenant.clients.values()].find((client) => client.name?.startsWith("Acme Admin"));
  assert.deepEqual(
    appClient?.refresh_token.policies.map((policy: { audience: string }) => policy.audience).sort(),
    [`https://${DOMAIN}/me/`, `https://${DOMAIN}/my-org/`].sort(),
  );
  const orgGrant = [...tenant.grants.values()].find((g) => g.audience === `https://${DOMAIN}/my-org/`);
  assert.deepEqual(orgGrant?.scope, MY_ORGANIZATION_SCOPES);
  const accountGrant = [...tenant.grants.values()].find((g) => g.audience === `https://${DOMAIN}/me/`);
  assert.deepEqual(accountGrant?.scope, MY_ACCOUNT_SCOPES);
});

test("business setup stops for a user who is in no organization when there is no terminal", async () => {
  const tenant = new FakeTenant();
  seedBusinessUser(tenant, { withOrg: false, withRole: false });

  await assert.rejects(
    runSetup({ ...CONSUMER, template: "business", userQuery: "jane@acme.com" }, harness(tenant).deps),
    (error: unknown) => {
      assert.ok(error instanceof BusinessAccessError);
      assert.match(error.fixes.join(" "), /auth0 api post \"organizations\/<org-id>\/members\"/);
      return true;
    },
  );
  assert.equal(tenant.portals.size, 0, "no portal is created before the test user is checked");
});

test("business setup adds a user to an organization only after confirmation", async () => {
  const tenant = new FakeTenant();
  seedBusinessUser(tenant, { withOrg: false, withRole: false });

  const { deps } = harness(tenant, {
    interactive: true,
    prompt: {
      async ask() {
        return "";
      },
      async confirm() {
        return true;
      },
      async checklist(): Promise<never> {
        throw new Error("unexpected question");
      },
      async choose<T>(_q: string, options: { value: T }[]): Promise<T> {
        return options[0].value;
      },
    },
  });
  const result = await runSetup({ ...CONSUMER, template: "business", userQuery: "jane@acme.com" }, deps);
  assert.equal(result.organization?.id, "org_1");
  assert.ok(tenant.members.get("org_1")?.has("auth0|user1"));
});

test("business setup refuses an organization with no enabled connection", async () => {
  const tenant = new FakeTenant();
  seedBusinessUser(tenant, { withOrg: true, withRole: true });
  tenant.orgConnections.set("org_1", []);

  await assert.rejects(
    runSetup({ ...CONSUMER, template: "business", userQuery: "jane@acme.com" }, harness(tenant).deps),
    (error: unknown) => {
      assert.ok(error instanceof BusinessAccessError);
      assert.match(error.message, /no enabled connections/);
      return true;
    },
  );
});

test("business setup asks for the test user before the portal is created", async () => {
  const tenant = new FakeTenant();
  seedBusinessUser(tenant, { withOrg: true, withRole: true });
  const asked: { question: string; portalsBefore: number }[] = [];
  const { deps } = harness(tenant, {
    interactive: true,
    prompt: {
      async ask(question) {
        asked.push({ question, portalsBefore: tenant.portals.size });
        return "jane@acme.com";
      },
      async confirm() {
        return true;
      },
      async checklist(): Promise<never> {
        throw new Error("unexpected question");
      },
      async choose<T>(_q: string, options: { value: T }[]): Promise<T> {
        return options[0].value;
      },
    },
  });
  const result = await runSetup({ ...CONSUMER, template: "business", name: "Acme Admin", slug: "acme-admin" }, deps);

  assert.deepEqual(asked, [
    { question: "Enter the test user's email or user ID (press Enter to skip)", portalsBefore: 0 },
  ]);
  assert.equal(tenant.portals.size, 1);
  assert.equal(result.organization?.id, "org_1");
});

test("business setup without a user check puts the organization ID in the link", async () => {
  const tenant = new FakeTenant();
  seedBusinessUser(tenant, { withOrg: false, withRole: false });
  const { deps } = harness(tenant, {
    interactive: true,
    prompt: {
      async ask(question) {
        return /test user/i.test(question) ? "" : "org_1";
      },
      async confirm() {
        return false;
      },
      async checklist(): Promise<never> {
        throw new Error("unexpected question");
      },
      async choose<T>(_q: string, options: { value: T }[]): Promise<T> {
        return options[0].value;
      },
    },
  });
  const result = await runSetup({ ...CONSUMER, template: "business", name: "Acme Admin", slug: "acme-admin" }, deps);

  assert.equal(result.portal.url, `https://${DOMAIN}/portals/acme-admin?organization=org_1`);
  assert.equal(result.organization, undefined, "no membership is checked without a user");
});

test("leaving the organization empty puts a placeholder in the link", async () => {
  const tenant = new FakeTenant();
  const { deps } = harness(tenant, {
    interactive: true,
    prompt: {
      async ask(_question, defaultValue) {
        return defaultValue ?? "";
      },
      async confirm() {
        return false;
      },
      async checklist(): Promise<never> {
        throw new Error("unexpected question");
      },
      async choose<T>(_q: string, options: { value: T }[]): Promise<T> {
        return options[0].value;
      },
    },
  });
  const result = await runSetup({ ...CONSUMER, template: "business", name: "Acme Admin", slug: "acme-admin" }, deps);

  assert.equal(result.portal.url, `https://${DOMAIN}/portals/acme-admin?organization=<ORGANIZATION_ID>`);
});

test("a dry run reports what a new consumer portal would create and sends no writes", async () => {
  const tenant = new FakeTenant();
  const plan = planWrites(tenant);
  const { deps } = harness(tenant, { api: plan.api, dryRun: true });
  const result = await runSetup(CONSUMER, deps);

  assert.equal(result.portal.status, "created");
  assert.equal(result.application.status, "created");
  assert.ok(result.resources.every((resource) => resource.status === "created"));
  assert.deepEqual(tenant.calls.filter((call) => !call.startsWith("GET ")), [], "a dry run must not write");
  assert.equal(tenant.portals.size + tenant.clients.size + tenant.forms.size + tenant.vaults.size, 0);

  const requests = plan.requests().map((request) => `${request.method} ${request.path}`);
  for (const expected of ["POST resource-servers", "POST clients", "POST client-grants", "POST forms/import", "POST portals"]) {
    assert.ok(requests.includes(expected), `the plan includes ${expected}`);
  }
  assert.equal(
    plan.requests().filter((request) => JSON.stringify(request.body) === "{}").length,
    0,
    "the permission probes that send writes are not sent in a dry run",
  );
});

test("a dry run after setup reuses what exists and plans the portal with a placeholder secret", async () => {
  const tenant = new FakeTenant();
  await runSetup(CONSUMER, harness(tenant).deps);
  const secret = [...tenant.clients.values()].find((client) => client.name?.startsWith("Acme Account"))?.client_secret;
  assert.ok(secret);
  tenant.portals.clear();
  const callsBefore = tenant.calls.length;

  const plan = planWrites(tenant);
  const { deps, output } = harness(tenant, { api: plan.api, dryRun: true });
  const result = await runSetup(CONSUMER, deps);

  assert.equal(result.application.status, "reused");
  assert.equal(result.portal.status, "created");
  assert.deepEqual(tenant.calls.slice(callsBefore).filter((call) => !call.startsWith("GET ")), []);

  const planned = plan.requests();
  assert.ok(planned.some((request) => request.method === "PATCH" && request.path.startsWith("clients/")));
  const portalRequest = planned.find((request) => request.method === "POST" && request.path === "portals");
  assert.equal((portalRequest?.body as { client: { client_secret: string } }).client.client_secret, CLIENT_SECRET_PLACEHOLDER);
  assert.ok(!JSON.stringify(planned).includes(secret), "the planned requests never carry the real secret");
  assert.ok(!output().includes(secret), "the output never shows the real secret");
});

test("a dry run of a business portal plans its organization access and writes nothing", async () => {
  const tenant = new FakeTenant();
  seedBusinessUser(tenant, { withOrg: true, withRole: false });
  const plan = planWrites(tenant);
  const { deps } = harness(tenant, { api: plan.api, dryRun: true });
  const result = await runSetup(
    { ...CONSUMER, template: "business", name: "Acme Admin", slug: "acme-admin", userQuery: "jane@acme.com" },
    deps,
  );

  assert.equal(result.organization?.id, "org_1");
  assert.equal(result.organization?.hasRole, false);
  assert.deepEqual(tenant.calls.filter((call) => !call.startsWith("GET ")), [], "a dry run must not write");
  assert.equal(tenant.portals.size, 0);
  assert.ok(plan.requests().some((request) => request.path === "resource-servers"));
});
