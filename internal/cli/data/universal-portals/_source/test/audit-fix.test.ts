import { strict as assert } from "node:assert";
import { test } from "node:test";

import { runAudit } from "../src/audit/run.js";
import { applyFixes, findingKey, fixableFindings } from "../src/audit/fix.js";
import { auditPortal } from "../src/audit/checks.js";
import { fetchPortalSnapshot } from "../src/audit/snapshot.js";
import { runSetup } from "../src/setup/run.js";
import { Ui } from "../src/ui.js";
import { FakeTenant } from "./fake-tenant.js";
import { captureOutput } from "./capture-output.js";

const DOMAIN = "acme.us.auth0.com";
const quiet = () => new Ui(captureOutput().stream, false, true);

async function createPortal(tenant: FakeTenant): Promise<{ portalId: string; clientId: string }> {
  tenant.connections.set("con_db", { id: "con_db", name: "Username-Password-Authentication", strategy: "auth0" });
  const result = await runSetup(
    { template: "consumer", name: "Acme Account", slug: "acme-account" },
    {
      api: tenant,
      ui: quiet(),
      prompt: { ask: async (_q, d) => d ?? "", confirm: async (_q, d) => d, choose: async (_q, o) => o[0].value, checklist: async () => { throw new Error("unexpected question"); } },
      interactive: false,
      tenantDomain: DOMAIN,
    },
  );
  return { portalId: result.portal.id, clientId: result.application.clientId };
}

function answers(confirm: boolean) {
  return {
    async ask() {
      return "";
    },
    async confirm() {
      return confirm;
    },
    async checklist(): Promise<never> {
      throw new Error("unexpected question");
    },
    async choose<T>(_q: string, options: { value: T }[]): Promise<T> {
      return options[0].value;
    },
  };
}

test("applying the errors repairs a home page, a missing refresh policy, and rotation", async () => {
  const tenant = new FakeTenant();
  const { portalId, clientId } = await createPortal(tenant);

  // Break the portal the way it commonly breaks: no home page, a component
  // without its refresh policy, and rotation switched on.
  tenant.portals.get(portalId)!.pages.default = undefined;
  const client = tenant.clients.get(clientId)!;
  client.refresh_token = { ...client.refresh_token, rotation_type: "rotating", policies: [] };

  const readSnapshot = () => fetchPortalSnapshot(tenant, portalId, DOMAIN);
  const findings = auditPortal(await readSnapshot());
  const errors = fixableFindings(findings).filter((finding) => finding.severity === "error");
  const result = await applyFixes(errors, { api: tenant, ui: quiet(), readSnapshot });
  assert.ok(result.applied >= 3, `applied ${result.applied}`);

  const after = await runAudit({ portalId, strict: false }, { api: tenant, ui: quiet(), tenantDomain: DOMAIN });
  assert.equal(after.errors, 0, JSON.stringify(after.findings));
});

test("a fix that is already in place is skipped, not applied twice", async () => {
  const tenant = new FakeTenant();
  const { portalId } = await createPortal(tenant);
  tenant.portals.get(portalId)!.pages.default = undefined;
  const readSnapshot = () => fetchPortalSnapshot(tenant, portalId, DOMAIN);
  const stale = auditPortal(await readSnapshot());
  const home = stale.find((finding) => finding.id === "P001")!;
  tenant.portals.get(portalId)!.pages.default = "profile"; // fixed by hand meanwhile

  const result = await applyFixes([home], { api: tenant, ui: quiet(), readSnapshot });
  assert.equal(result.alreadyFixed, 1);
  assert.equal(result.applied, 0);
});

test("a fix still applies when the finding's message changed after the audit", async () => {
  const tenant = new FakeTenant();
  const { portalId } = await createPortal(tenant);
  const legacy = { type: "sidebar:component:auth0:internal_link", config: { label: "Profile", to: "profile", icon: "user" } };
  tenant.portals.get(portalId)!.navigation.sidebar.components = [legacy];
  const readSnapshot = () => fetchPortalSnapshot(tenant, portalId, DOMAIN);
  const finding = auditPortal(await readSnapshot()).find((item) => item.id === "P003");
  assert.ok(finding?.remedy?.apply, "P003 is reported with a fix");

  // A second legacy link changes the count in the message, not the problem.
  tenant.portals.get(portalId)!.navigation.sidebar.components.push({ ...legacy, config: { ...legacy.config, to: "account" } });
  const result = await applyFixes([finding], { api: tenant, ui: quiet(), readSnapshot });

  assert.equal(result.applied, 1);
  assert.equal(result.alreadyFixed, 0);
});

test("findings that share an ID are told apart by their subject", async () => {
  const tenant = new FakeTenant();
  const { portalId } = await createPortal(tenant);
  tenant.portals.get(portalId)!.pages.standard.push(
    { slug: "terms", title: "Terms", components: [] },
    { slug: "faq", title: "FAQ", components: [] },
  );

  const emptyPages = auditPortal(await fetchPortalSnapshot(tenant, portalId, DOMAIN)).filter((finding) => finding.id === "P006");

  assert.equal(emptyPages.length, 2);
  assert.equal(new Set(emptyPages.map(findingKey)).size, 2);
});

test("warnings are reported but never offered for fixing", async () => {
  const tenant = new FakeTenant();
  const { portalId, clientId } = await createPortal(tenant);
  tenant.clients.get(clientId)!.is_token_endpoint_ip_header_trusted = false;

  const findings = auditPortal(await fetchPortalSnapshot(tenant, portalId, DOMAIN));
  assert.equal(fixableFindings(findings).length, 0, "warnings are never fixable");
  assert.ok(findings.some((finding) => finding.id === "P017" && finding.severity === "warning"));
});
