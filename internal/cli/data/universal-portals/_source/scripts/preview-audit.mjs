// Dev-only preview of the audit experience. It sets up a healthy portal in the
// in-memory fake tenant, breaks it in the chosen way, then runs the real audit
// command. Nothing touches a real tenant.
//
// npm run preview:audit                       menu, with random breaks
// npm run preview:audit -- --fix              apply the errors without asking
// npm run preview:audit -- --scenario NAME    a named set of breaks
// Scenarios: home, rotation, policy, grant, nav, connections, all
import { FakeTenant } from "../dist/test/fake-tenant.js";
import { runSetup } from "../dist/src/setup/run.js";
import { main } from "../dist/src/cli.js";
import { Ui } from "../dist/src/ui.js";

const TENANT = "acme.us.auth0.com";
const LATENCY_MS = Number(process.env.PREVIEW_LATENCY_MS ?? 300);
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

if (!process.stdin.isTTY) {
  console.error("The audit preview asks questions, so run it in a terminal.");
  process.exit(1);
}

const args = process.argv.slice(2);
const scenarioName = args.includes("--scenario") ? args[args.indexOf("--scenario") + 1] : undefined;

// Each break is one setup mistake, and the finding it should produce.
const BREAKS = {
  home: {
    finding: "P001 home page",
    apply: ({ portal }) => (portal.pages.default = undefined),
  },
  rotation: {
    finding: "P013 refresh token rotation",
    apply: ({ app }) => (app.refresh_token = { ...app.refresh_token, rotation_type: "rotating" }),
  },
  policy: {
    finding: "P020 refresh token policy",
    apply: ({ app }) => (app.refresh_token = { ...app.refresh_token, policies: [] }),
  },
  grant: {
    finding: "P019 client grant",
    apply: ({ tenant, app }) => {
      for (const [id, grant] of tenant.grants) {
        if (grant.client_id === app.client_id && grant.audience.endsWith("/me/")) tenant.grants.delete(id);
      }
    },
  },
  nav: {
    finding: "P002 home page in navigation",
    apply: ({ portal }) => {
      portal.navigation.sidebar.components = portal.navigation.sidebar.components.filter(
        (item) => item.config.page_slug !== "profile",
      );
    },
  },
  connections: {
    finding: "P021 connections",
    apply: ({ tenant }) => tenant.connections.clear(),
  },
  callback: {
    finding: "P012 callback url",
    apply: ({ app }) => (app.callbacks = []),
  },
  client_credentials: {
    finding: "P010 client credentials",
    apply: ({ app }) => (app.grant_types = [...app.grant_types, "client_credentials"]),
  },
  ip_header: {
    finding: "P017 token endpoint IP header",
    apply: ({ app }) => (app.is_token_endpoint_ip_header_trusted = false),
  },
  empty_page: {
    finding: "P006 empty page",
    apply: ({ portal }) => {
      portal.pages.standard.push({ title: "Billing", slug: "billing", components: [] });
      portal.navigation.sidebar.components.push({
        type: "sidebar:component:auth0:internal_link:v2",
        config: { label: "Billing", page_slug: "billing", icon: "file" },
      });
    },
  },
};

const SCENARIOS = {
  home: ["home"],
  rotation: ["rotation"],
  policy: ["policy"],
  grant: ["grant"],
  nav: ["nav"],
  connections: ["connections"],
  all: Object.keys(BREAKS),
};

function chooseBreaks() {
  if (scenarioName) {
    const names = SCENARIOS[scenarioName];
    if (!names) {
      console.error(`Unknown scenario "${scenarioName}". Use one of: ${Object.keys(SCENARIOS).join(", ")}.`);
      process.exit(1);
    }
    return names;
  }
  // Random by default: at least four different breaks, up to all of them, so
  // every run looks different.
  const all = Object.keys(BREAKS);
  const names = all.sort(() => Math.random() - 0.5);
  const count = 4 + Math.floor(Math.random() * (all.length - 4 + 1));
  return names.slice(0, count);
}

const tenant = new FakeTenant();
tenant.connections.set("con_db", { id: "con_db", name: "Username-Password-Authentication", strategy: "auth0" });
const quietPrompt = {
  ask: async (_q, d) => d ?? "",
  confirm: async (_q, d) => d,
  choose: async (_q, o) => o[0].value,
  checklist: async () => {
    throw new Error("unexpected question");
  },
};
const setup = await runSetup(
  { template: "consumer", name: "Acme Account", slug: "acme-account" },
  { api: tenant, ui: new Ui(() => undefined, false, true), prompt: quietPrompt, interactive: false, tenantDomain: TENANT },
);

const breaks = chooseBreaks();
const context = {
  tenant,
  app: tenant.clients.get(setup.application.clientId),
  portal: tenant.portals.get(setup.portal.id),
};
for (const name of breaks) BREAKS[name].apply(context);
console.error(`(preview: broke ${breaks.length} thing(s): ${breaks.map((name) => BREAKS[name].finding).join("; ")})`);
tenant.calls.length = 0;

const fetchImpl = async (url, init) => {
  await sleep(LATENCY_MS);
  const parsed = new URL(url);
  const path = parsed.pathname.replace(/^\/api\/v2\//, "") + parsed.search;
  const body = init.body ? JSON.parse(String(init.body)) : undefined;
  try {
    const result = await tenant.request(init.method, path, body);
    return new Response(result === undefined ? "" : JSON.stringify(result), { status: 200 });
  } catch (error) {
    return new Response(JSON.stringify(error.body), { status: error.status });
  }
};

const env = {
  AUTH0_CLI_PROXY_URL: "https://127.0.0.1:4433",
  AUTH0_CLI_PROXY_TOKEN: "preview",
  AUTH0_CLI_TENANT_DOMAIN: TENANT,
};
const io = {
  stdout: process.stdout,
  stderr: process.stderr,
  stdinStream: process.stdin,
  isTTY: true,
  fetchImpl,
};

const cliArgs = ["audit", setup.portal.id, ...(args.includes("--fix") ? ["--fix"] : [])];
const code = await main(cliArgs, env, io);
console.error(`(preview: exit ${code})`);
