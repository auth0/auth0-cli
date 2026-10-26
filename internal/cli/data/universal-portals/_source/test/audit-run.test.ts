import { strict as assert } from "node:assert";
import { test } from "node:test";

import { auditPortal } from "../src/audit/checks.js";
import { applyFixRounds, fixableFindings } from "../src/audit/fix.js";
import { fetchPortalSnapshot } from "../src/audit/snapshot.js";
import { runAudit } from "../src/audit/run.js";
import { MY_ORGANIZATION_SCOPES } from "../src/setup/payload.js";
import { runSetup } from "../src/setup/run.js";
import { Ui } from "../src/ui.js";
import { FakeTenant } from "./fake-tenant.js";
import { captureOutput } from "./capture-output.js";

const DOMAIN = "acme.us.auth0.com";

function quietUi(): Ui {
  return new Ui(captureOutput().stream, false, true);
}

function setupPrompt() {
  return {
    async ask(_q: string, defaultValue?: string) {
      return defaultValue ?? "";
    },
    async confirm(_q: string, defaultValue: boolean) {
      return defaultValue;
    },
    async checklist(): Promise<never> {
      throw new Error("unexpected question");
    },
    async choose<T>(_q: string, options: { value: T }[]): Promise<T> {
      return options[0].value;
    },
  };
}

// A portal created by setup should pass the audit with no findings, so the
// audit and the setup agree on what a healthy portal is.
test("a portal created by setup passes the audit", async () => {
  const tenant = new FakeTenant();
  tenant.connections.set("con_db", { id: "con_db", name: "Username-Password-Authentication", strategy: "auth0" });
  const result = await runSetup(
    { template: "consumer", name: "Acme Account", slug: "acme-account" },
    { api: tenant, ui: quietUi(), prompt: setupPrompt(), interactive: false, tenantDomain: DOMAIN },
  );

  const audit = await runAudit(
    { portalId: result.portal.id, strict: false },
    { api: tenant, ui: quietUi(), tenantDomain: DOMAIN },
  );
  assert.deepEqual(audit.findings, []);
  assert.equal(audit.failed, false);
});

test("a deleted form is reported as an error and fails the audit", async () => {
  const tenant = new FakeTenant();
  tenant.connections.set("con_db", { id: "con_db", name: "Username-Password-Authentication", strategy: "auth0" });
  const result = await runSetup(
    { template: "consumer", name: "Acme Account", slug: "acme-account" },
    { api: tenant, ui: quietUi(), prompt: setupPrompt(), interactive: false, tenantDomain: DOMAIN },
  );
  const formId = [...tenant.forms.keys()][0];
  tenant.forms.delete(formId);

  const audit = await runAudit(
    { portalId: result.portal.id, strict: false },
    { api: tenant, ui: quietUi(), tenantDomain: DOMAIN },
  );
  assert.ok(audit.findings.some((finding) => finding.id === "P007"));
  assert.equal(audit.failed, true);
});

test("a portal whose application was deleted is reported as P009, not a crash", async () => {
  const tenant = new FakeTenant();
  tenant.connections.set("con_db", { id: "con_db", name: "Username-Password-Authentication", strategy: "auth0" });
  const result = await runSetup(
    { template: "consumer", name: "Acme Account", slug: "acme-account" },
    { api: tenant, ui: quietUi(), prompt: setupPrompt(), interactive: false, tenantDomain: DOMAIN },
  );
  tenant.clients.delete(result.application.clientId);

  const audit = await runAudit(
    { portalId: result.portal.id, strict: false },
    { api: tenant, ui: quietUi(), tenantDomain: DOMAIN },
  );
  assert.ok(audit.findings.some((finding) => finding.id === "P009"));
  assert.equal(audit.failed, true);
});

test("the P009 fix creates an application and links it, so the next audit is clean", async () => {
  const tenant = new FakeTenant();
  tenant.connections.set("con_db", { id: "con_db", name: "Username-Password-Authentication", strategy: "auth0" });
  const result = await runSetup(
    { template: "consumer", name: "Acme Account", slug: "acme-account" },
    { api: tenant, ui: quietUi(), prompt: setupPrompt(), interactive: false, tenantDomain: DOMAIN },
  );
  tenant.clients.delete(result.application.clientId);

  const readSnapshot = () => fetchPortalSnapshot(tenant, result.portal.id, DOMAIN);
  const findings = auditPortal(await readSnapshot());
  await applyFixRounds(fixableFindings(findings), { api: tenant, ui: quietUi(), readSnapshot }, async (exposed) => exposed);

  const linked = tenant.portals.get(result.portal.id)!.client.client_id;
  assert.ok(tenant.clients.has(linked), "the portal points at a client that exists");
  assert.notEqual(linked, result.application.clientId);
  const audit = await runAudit(
    { portalId: result.portal.id, strict: false },
    { api: tenant, ui: quietUi(), tenantDomain: DOMAIN },
  );
  assert.equal(audit.errors, 0);
});

test("errors that a fix exposes are left alone when the user declines them", async () => {
  const tenant = new FakeTenant();
  tenant.connections.set("con_db", { id: "con_db", name: "Username-Password-Authentication", strategy: "auth0" });
  const result = await runSetup(
    { template: "consumer", name: "Acme Account", slug: "acme-account" },
    { api: tenant, ui: quietUi(), prompt: setupPrompt(), interactive: false, tenantDomain: DOMAIN },
  );
  tenant.clients.delete(result.application.clientId);

  const readSnapshot = () => fetchPortalSnapshot(tenant, result.portal.id, DOMAIN);
  await applyFixRounds(fixableFindings(auditPortal(await readSnapshot())), { api: tenant, ui: quietUi(), readSnapshot }, async () => []);

  const audit = await runAudit({ portalId: result.portal.id, strict: false }, { api: tenant, ui: quietUi(), tenantDomain: DOMAIN });
  assert.ok(audit.findings.some((finding) => finding.id === "P019"), "the exposed grant error is still reported");
});

test("a form ID the API cannot match is reported as a missing form (P007), not a crash", async () => {
  const tenant = new FakeTenant();
  tenant.connections.set("con_db", { id: "con_db", name: "Username-Password-Authentication", strategy: "auth0" });
  const result = await runSetup(
    { template: "consumer", name: "Acme Account", slug: "acme-account" },
    { api: tenant, ui: quietUi(), prompt: setupPrompt(), interactive: false, tenantDomain: DOMAIN },
  );
  const portal = tenant.portals.get(result.portal.id)!;
  const form = portal.pages.standard[0].components[0].config.children[0];
  form.config.form_id = "ap_not-a-real-form-id";

  const audit = await runAudit(
    { portalId: result.portal.id, strict: false },
    { api: tenant, ui: quietUi(), tenantDomain: DOMAIN },
  );
  assert.ok(audit.findings.some((finding) => finding.id === "P007"));
});

test("a warning fails the audit only with --strict", async () => {
  const tenant = new FakeTenant();
  tenant.connections.set("con_db", { id: "con_db", name: "Username-Password-Authentication", strategy: "auth0" });
  const result = await runSetup(
    { template: "consumer", name: "Acme Account", slug: "acme-account" },
    { api: tenant, ui: quietUi(), prompt: setupPrompt(), interactive: false, tenantDomain: DOMAIN },
  );
  // Custom code with no custom domain is a warning.
  const portal = tenant.portals.get(result.portal.id)!;
  portal.custom_code = { head: "<script></script>" };

  const lenient = await runAudit({ portalId: result.portal.id, strict: false }, { api: tenant, ui: quietUi(), tenantDomain: DOMAIN });
  const strict = await runAudit({ portalId: result.portal.id, strict: true }, { api: tenant, ui: quietUi(), tenantDomain: DOMAIN });
  assert.equal(lenient.failed, false);
  assert.equal(strict.failed, true);
});

test("a business portal created by setup passes the audit", async () => {
  const tenant = new FakeTenant();
  tenant.connections.set("con_db", { id: "con_db", name: "Username-Password-Authentication", strategy: "auth0" });
  tenant.users.set("auth0|jane", { user_id: "auth0|jane", email: "jane@acme.com", name: "Jane" });
  tenant.organizations.set("org_1", { id: "org_1", name: "acme" });
  tenant.members.set("org_1", new Set(["auth0|jane"]));
  tenant.orgConnections.set("org_1", ["con_db"]);
  tenant.roles.set("rol_admin", { id: "rol_admin", name: "Organization - Admin", permissions: MY_ORGANIZATION_SCOPES });
  tenant.orgRoles.set("org_1:auth0|jane", ["rol_admin"]);

  const result = await runSetup(
    { template: "business", name: "Acme Admin", slug: "acme-admin", userQuery: "auth0|jane" },
    { api: tenant, ui: quietUi(), prompt: setupPrompt(), interactive: false, tenantDomain: DOMAIN },
  );

  const audit = await runAudit(
    { portalId: result.portal.id, strict: false },
    { api: tenant, ui: quietUi(), tenantDomain: DOMAIN },
  );
  assert.deepEqual(audit.findings, []);
});
