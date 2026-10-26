import { strict as assert } from "node:assert";
import { test } from "node:test";

import { FakeTenant } from "./fake-tenant.js";
import { buildPortalForms } from "../src/forms.js";
import { buildPortalClient, buildPortalRequest, templateFor } from "../src/setup/payload.js";
import {
  ensureClientGrant,
  ensureForms,
  ensurePortal,
  ensurePortalClient,
  ensureResourceServer,
  ensureVaultConnection,
  NameInUseError,
  refreshVaultConnection,
  SlugTakenError,
} from "../src/setup/resources.js";

const DOMAIN = "acme.us.auth0.com";
const CLIENT_INPUT = {
  name: "Acme Admin (Auth0 Portals)",
  slug: "acme-admin",
  template: "business" as const,
  tenantDomain: DOMAIN,
  portalHost: DOMAIN,
  isCustomDomain: false,
};

test("a resource server is created once and reused on rerun", async () => {
  const tenant = new FakeTenant();
  const first = await ensureResourceServer(tenant, { name: "My Account", identifier: `https://${DOMAIN}/me/` });
  const second = await ensureResourceServer(tenant, { name: "My Account", identifier: `https://${DOMAIN}/me/` });

  assert.equal(first.status, "created");
  assert.equal(second.status, "reused");
  assert.equal(second.value.id, first.value.id);
  assert.equal(tenant.resourceServers.size, 1);
});

test("a rerun returns the same client and secret, without creating a second client", async () => {
  const tenant = new FakeTenant();
  const first = await ensurePortalClient(tenant, CLIENT_INPUT);
  const second = await ensurePortalClient(tenant, CLIENT_INPUT);

  assert.equal(first.status, "created");
  assert.equal(second.status, "reused");
  assert.equal(second.value.clientId, first.value.clientId);
  assert.equal(second.value.clientSecret, first.value.clientSecret);
  assert.equal(tenant.clients.size, 1);
});

test("a rerun turns on session transfer for a client that lacks it", async () => {
  const tenant = new FakeTenant();
  const { value } = await ensurePortalClient(tenant, CLIENT_INPUT);
  delete tenant.clients.get(value.clientId)!.session_transfer;

  await ensurePortalClient(tenant, CLIENT_INPUT);

  assert.deepEqual(tenant.clients.get(value.clientId)!.session_transfer, {
    allowed_authentication_methods: ["query"],
    enforce_device_binding: "ip",
  });
});

test("a rerun keeps a session transfer setting that was changed on purpose", async () => {
  const tenant = new FakeTenant();
  const { value } = await ensurePortalClient(tenant, CLIENT_INPUT);
  tenant.clients.get(value.clientId)!.session_transfer = {
    allowed_authentication_methods: ["query"],
    enforce_device_binding: "asn",
  };

  await ensurePortalClient(tenant, CLIENT_INPUT);

  assert.equal(tenant.clients.get(value.clientId)!.session_transfer.enforce_device_binding, "asn");
});

test("an application past the first page of clients is found and reused, not created again", async () => {
  const tenant = new FakeTenant();
  const { value } = await ensurePortalClient(tenant, CLIENT_INPUT);
  // Push the application past the first page: 100 other clients come before it.
  const application = tenant.clients.get(value.clientId)!;
  tenant.clients.delete(value.clientId);
  for (let index = 0; index < 100; index += 1) {
    tenant.clients.set(`other_${index}`, { client_id: `other_${index}`, name: `Other ${index}` });
  }
  tenant.clients.set(value.clientId, application);

  const rerun = await ensurePortalClient(tenant, CLIENT_INPUT);

  assert.equal(rerun.status, "reused");
  assert.equal(tenant.clients.size, 101);
});

test("a same-named application that setup did not create is never reused", async () => {
  const tenant = new FakeTenant();
  tenant.clients.set("cli_manual", { client_id: "cli_manual", client_secret: "x", name: CLIENT_INPUT.name });

  await assert.rejects(ensurePortalClient(tenant, CLIENT_INPUT), NameInUseError);
});

test("a client grant is created, reused when unchanged, and updated when scopes differ", async () => {
  const tenant = new FakeTenant();
  const audience = `https://${DOMAIN}/me/`;

  const created = await ensureClientGrant(tenant, { clientId: "cli_1", audience, scope: ["read:me:factors"] });
  const reused = await ensureClientGrant(tenant, { clientId: "cli_1", audience, scope: ["read:me:factors"] });
  const updated = await ensureClientGrant(tenant, {
    clientId: "cli_1",
    audience,
    scope: ["read:me:factors", "read:me:authentication_methods"],
  });

  assert.equal(created.status, "created");
  assert.equal(reused.status, "reused");
  assert.equal(updated.status, "updated");
  assert.equal(tenant.grants.size, 1);
});

test("a vault connection is reused by name, and a scope change refreshes it with setup", async () => {
  const tenant = new FakeTenant();
  const setup = { domain: DOMAIN, clientId: "cli_vault", clientSecret: "s" };
  const created = await ensureVaultConnection(tenant, "Auth0 Portals forms vault", setup);
  const reused = await ensureVaultConnection(tenant, "Auth0 Portals forms vault", setup);

  assert.equal(created.status, "created");
  assert.equal(reused.value.id, created.value.id);
  assert.equal(tenant.vaults.size, 1);

  await refreshVaultConnection(tenant, created.value.id, setup);
  assert.equal(tenant.vaults.get(created.value.id)?.setup.client_id, "cli_vault");
});

test("forms are imported once and reused by name", async () => {
  const tenant = new FakeTenant();
  const forms = buildPortalForms("ac_vault");

  const first = await ensureForms(tenant, forms);
  const second = await ensureForms(tenant, forms);

  assert.equal(first.personalInfo.status, "created");
  assert.equal(second.personalInfo.status, "reused");
  assert.equal(second.personalInfo.value.id, first.personalInfo.value.id);
  assert.equal(tenant.forms.size, 3);
});

test("a portal is created, reused for the same client, and refused for another client", async () => {
  const tenant = new FakeTenant();
  const forms = await ensureForms(tenant, buildPortalForms("ac_vault"));
  const template = templateFor("business", {
    personalInfo: forms.personalInfo.value.id,
    privacyConsent: forms.privacyConsent.value.id,
    communicationPreferences: forms.communicationPreferences.value.id,
  });
  const request = buildPortalRequest(template, {
    slug: "acme-admin",
    name: "Acme Admin",
    clientId: "cli_1",
    clientSecret: "s",
  });

  const created = await ensurePortal(tenant, request);
  const reused = await ensurePortal(tenant, request);
  assert.equal(created.status, "created");
  assert.equal(reused.status, "reused");

  const otherClient = { ...request, client: { ...request.client, client_id: "cli_other" } };
  await assert.rejects(ensurePortal(tenant, otherClient), SlugTakenError);
});

test("a simulated failure mid-run leaves completed resources in place for the rerun to reuse", async () => {
  const tenant = new FakeTenant();
  await ensurePortalClient(tenant, CLIENT_INPUT);
  tenant.failNext("POST forms/import", 500);

  await assert.rejects(ensureForms(tenant, buildPortalForms("ac_vault")), /simulated failure/);

  const rerunClient = await ensurePortalClient(tenant, CLIENT_INPUT);
  assert.equal(rerunClient.status, "reused");
  assert.equal(tenant.clients.size, 1);
});

test("each form reports the flow it runs, whether it was created or reused", async () => {
  const tenant = new FakeTenant();
  const first = await ensureForms(tenant, buildPortalForms("ac_vault"));
  const second = await ensureForms(tenant, buildPortalForms("ac_vault"));

  assert.ok(first.personalInfo.value.flowId, "a created form reports its flow");
  assert.equal(second.personalInfo.value.flowId, first.personalInfo.value.flowId);
  assert.equal(second.personalInfo.status, "reused");
});

test("the portal application accepts session transfer tokens from native apps", () => {
  const client = buildPortalClient(CLIENT_INPUT);
  assert.deepEqual(client.session_transfer, {
    allowed_authentication_methods: ["query"],
    enforce_device_binding: "ip",
  });
});
