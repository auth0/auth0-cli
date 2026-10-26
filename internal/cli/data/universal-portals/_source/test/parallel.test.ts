import { strict as assert } from "node:assert";
import { test } from "node:test";

import { ApiClient, HttpMethod } from "../src/api.js";
import { fetchPortalSnapshot } from "../src/audit/snapshot.js";
import { runPreflight, SETUP_PROBES } from "../src/setup/preflight.js";
import { runSetup } from "../src/setup/run.js";
import { Ui } from "../src/ui.js";
import { captureOutput } from "./capture-output.js";
import { FakeTenant } from "./fake-tenant.js";

const DOMAIN = "acme.us.auth0.com";

// Answers like a real network: each request takes a while, and the delay can differ per request, so
// answers arrive out of order. It records the most requests that were in flight at once.
function slowNetwork(tenant: FakeTenant, delayFor: (method: HttpMethod, path: string) => number) {
  let active = 0;
  let peak = 0;
  const api: ApiClient = {
    async request<T>(method: HttpMethod, path: string, body?: unknown): Promise<T> {
      active += 1;
      peak = Math.max(peak, active);
      try {
        await new Promise((resolve) => setTimeout(resolve, delayFor(method, path)));
        return await tenant.request<T>(method, path, body);
      } finally {
        active -= 1;
      }
    },
  };
  return { api, peak: () => peak };
}

test("the permission probes run together, and the gaps keep the order of the probes", async () => {
  const tenant = new FakeTenant();
  tenant.deny("POST clients", "Insufficient scope, expected any of: create:clients");
  tenant.deny("POST portals", "Insufficient scope, expected any of: create:portals");
  // The first gap answers last, the second answers first.
  const network = slowNetwork(tenant, (_method, path) => (path === "clients" ? 40 : 2));

  const result = await runPreflight(network.api, SETUP_PROBES, DOMAIN);

  assert.deepEqual(result.gaps.map((gap) => gap.scopes), [["create:clients"], ["create:portals"]]);
  assert.ok(network.peak() > 1, "the probes were sent one after another");
});

test("the audit reads the independent parts of a portal together and gets the same snapshot", async () => {
  const tenant = new FakeTenant();
  tenant.connections.set("con_db", { id: "con_db", name: "Username-Password-Authentication", strategy: "auth0" });
  const quiet = new Ui(captureOutput().stream, false, true);
  const setup = await runSetup(
    { template: "consumer", name: "Acme Account", slug: "acme-account" },
    {
      api: tenant,
      ui: quiet,
      prompt: {
        ask: async (_q, d) => d ?? "",
        confirm: async (_q, d) => d,
        choose: async (_q, o) => o[0].value,
        checklist: async () => [],
      },
      interactive: false,
      tenantDomain: DOMAIN,
    },
  );
  const sequential = await fetchPortalSnapshot(tenant, setup.portal.id, DOMAIN);

  const network = slowNetwork(tenant, () => 10);
  const together = await fetchPortalSnapshot(network.api, setup.portal.id, DOMAIN);

  assert.deepEqual(together, sequential);
  assert.ok(network.peak() > 1, "the reads were sent one after another");
});
