import { strict as assert } from "node:assert";
import { test } from "node:test";

import { ApiClient } from "../src/api.js";
import { fetchPortalSnapshot } from "../src/audit/snapshot.js";
import { applyFixes } from "../src/audit/fix.js";
import { auditPortal } from "../src/audit/checks.js";
import { PLUGIN_SCOPES } from "../src/scopes.js";
import { runPreflight, SETUP_PROBES } from "../src/setup/preflight.js";
import { runSetup, SetupDeps } from "../src/setup/run.js";
import { Ui } from "../src/ui.js";
import { FakeTenant } from "./fake-tenant.js";
import { captureOutput } from "./capture-output.js";

const DOMAIN = "acme.us.auth0.com";
const quiet = () => new Ui(captureOutput().stream, false, true);

// Mirrors the rule the CLI proxy of the plugin framework uses (requiredScopeFor): the verb
// comes from the HTTP method and the resource is the first path segment.
const VERBS: Record<string, string> = { GET: "read", POST: "create", PATCH: "update", PUT: "update", DELETE: "delete" };

function scopeOf(call: string): string {
  const [method, path] = call.split(" ");
  const resource = path.split("?")[0].split("/")[0].replace(/-/g, "_");
  return `${VERBS[method]}:${resource}`;
}

function deps(tenant: FakeTenant): SetupDeps {
  return {
    api: tenant,
    ui: quiet(),
    prompt: {
      ask: async (_q, d) => d ?? "",
      confirm: async () => false,
      checklist: async () => {
        throw new Error("unexpected question");
      },
      choose: async (_q, options) => options[0].value,
    },
    interactive: false,
    tenantDomain: DOMAIN,
  };
}

test("every scope the plugin's flows need is in PLUGIN_SCOPES", async () => {
  const tenant = new FakeTenant();
  tenant.connections.set("con_db", { id: "con_db", name: "Username-Password-Authentication", strategy: "auth0" });
  const result = await runSetup({ template: "consumer", name: "Acme Account", slug: "acme-account" }, deps(tenant));

  await runPreflight(tenant, SETUP_PROBES, DOMAIN);

  tenant.portals.get(result.portal.id)!.navigation.sidebar.components = [
    { type: "sidebar:component:auth0:internal_link", config: { label: "Profile", to: "profile", icon: "user" } },
  ];
  const readSnapshot = () => fetchPortalSnapshot(tenant, result.portal.id, DOMAIN);
  const fixes = auditPortal(await readSnapshot()).filter((finding) => finding.remedy?.apply);
  await applyFixes(fixes, { api: tenant, ui: quiet(), readSnapshot });

  const needed = [...new Set(tenant.calls.map(scopeOf))].sort();
  const missing = needed.filter((scope) => !PLUGIN_SCOPES.includes(scope));
  assert.deepEqual(missing, [], "add these scopes to PLUGIN_SCOPES");
});

export type { ApiClient };
