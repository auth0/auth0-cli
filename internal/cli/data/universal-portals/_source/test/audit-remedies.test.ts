import { strict as assert } from "node:assert";
import { test } from "node:test";

import { auditPortal } from "../src/audit/checks.js";
import { applyFixes, fixableFindings } from "../src/audit/fix.js";
import { fetchPortalSnapshot } from "../src/audit/snapshot.js";
import { runSetup } from "../src/setup/run.js";
import { Ui } from "../src/ui.js";
import { FakeTenant } from "./fake-tenant.js";
import { captureOutput } from "./capture-output.js";

const DOMAIN = "acme.us.auth0.com";
const quiet = () => new Ui(captureOutput().stream, false, true);

// A healthy consumer portal from setup, so each test breaks exactly one thing.
async function healthyPortal(): Promise<{ tenant: FakeTenant; portalId: string; clientId: string }> {
  const tenant = new FakeTenant();
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
  return { tenant, portalId: result.portal.id, clientId: result.application.clientId };
}

// Runs the fix for one finding and returns what the tenant received.
async function fixOnly(tenant: FakeTenant, portalId: string, id: string): Promise<void> {
  const readSnapshot = () => fetchPortalSnapshot(tenant, portalId, DOMAIN);
  const finding = auditPortal(await readSnapshot()).find((item) => item.id === id);
  assert.ok(finding, `${id} should be reported before the fix`);
  assert.ok(finding.remedy?.apply, `${id} should have an automatic fix`);
  await applyFixes([finding], { api: tenant, ui: quiet(), readSnapshot });
}

test("P003: converting a legacy sidebar link leaves only :v2 links in the sidebar", async () => {
  const { tenant, portalId } = await healthyPortal();
  tenant.portals.get(portalId)!.navigation.sidebar.components = [
    { type: "sidebar:component:auth0:internal_link", config: { label: "Profile", to: "profile", icon: "user" } },
  ];

  await fixOnly(tenant, portalId, "P003");

  const links = tenant.portals.get(portalId)!.navigation.sidebar.components;
  assert.deepEqual(links[0], {
    type: "sidebar:component:auth0:internal_link:v2",
    config: { label: "Profile", page_slug: "profile", icon: "user" },
  });
});

test("P002: adding the home page to the sidebar makes it reachable", async () => {
  const { tenant, portalId } = await healthyPortal();
  tenant.portals.get(portalId)!.navigation.sidebar.components = [];

  await fixOnly(tenant, portalId, "P002");

  const pages = tenant.portals.get(portalId)!.navigation.sidebar.components.map((item: { config: { page_slug: string } }) => item.config.page_slug);
  assert.ok(pages.includes("profile"));
});

test("P011: removing the Management API grant deletes it from the application", async () => {
  const { tenant, portalId, clientId } = await healthyPortal();
  tenant.grants.set("cgr_mgmt", {
    id: "cgr_mgmt",
    client_id: clientId,
    audience: `https://${DOMAIN}/api/v2/`,
    scope: ["read:users"],
  });

  await fixOnly(tenant, portalId, "P011");

  assert.equal(tenant.grants.has("cgr_mgmt"), false);
});

test("P009: fixing a missing application creates the client and links it to the portal", async () => {
  const { tenant, portalId, clientId } = await healthyPortal();
  tenant.portals.get(portalId)!.client = undefined;
  tenant.clients.delete(clientId);
  const clientsBefore = tenant.clients.size;

  await fixOnly(tenant, portalId, "P009");

  assert.equal(tenant.clients.size, clientsBefore + 1, "one client was created");
  const linked = tenant.portals.get(portalId)!.client;
  assert.ok(linked?.client_id && tenant.clients.has(linked.client_id), "the portal points at the new client");
  assert.equal(linked.client_secret !== undefined, true, "the secret is sent with the link");
});

test("P009: a link that fails after the application was created does not create a second application on rerun", async () => {
  const { tenant, portalId, clientId } = await healthyPortal();
  tenant.portals.get(portalId)!.client = undefined;
  tenant.clients.delete(clientId);
  const clientsBefore = tenant.clients.size;
  tenant.failNext(`PATCH portals/${portalId}`, 500);

  await fixOnly(tenant, portalId, "P009");
  assert.equal(tenant.portals.get(portalId)!.client, undefined, "the first link failed");

  await fixOnly(tenant, portalId, "P009");

  assert.equal(tenant.clients.size, clientsBefore + 1, "one application in total");
  const linked = tenant.portals.get(portalId)!.client;
  assert.ok(linked?.client_id && tenant.clients.has(linked.client_id), "the portal points at the application");
});

test("P023: turning DPoP off on the application", async () => {
  const { tenant, portalId, clientId } = await healthyPortal();
  tenant.clients.get(clientId)!.require_proof_of_possession = true;

  await fixOnly(tenant, portalId, "P023");

  assert.equal(tenant.clients.get(clientId)!.require_proof_of_possession, false);
});

test("warnings are not fixed, even when a fix exists for them", async () => {
  const { tenant, portalId, clientId } = await healthyPortal();
  tenant.clients.get(clientId)!.is_token_endpoint_ip_header_trusted = false;

  const findings = auditPortal(await fetchPortalSnapshot(tenant, portalId, DOMAIN));
  const warning = findings.find((item) => item.id === "P017");
  assert.equal(warning?.severity, "warning");
  assert.equal(fixableFindings(findings).some((item) => item.id === "P017"), false);
});
