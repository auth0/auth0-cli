// Dev-only preview of the setup experience. It runs the real CLI entry point, so
// the questions, the progress and the result are exactly what the CLI shows. The
// setup runs against the in-memory fake tenant with latency on each call, and seeds
// some resources so the output mixes created and already-exists results. It makes
// no API calls and opens no browser.
//
// Run it in a terminal: npm run preview
import { FakeTenant } from "../dist/test/fake-tenant.js";
import { FORM_NAMES } from "../dist/src/forms.js";
import { main } from "../dist/src/cli.js";

const TENANT = "acme.us.auth0.com";
const LATENCY_MS = Number(process.env.PREVIEW_LATENCY_MS ?? 400);
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

if (!process.stdin.isTTY) {
  console.error("The preview asks questions, so run it in a terminal.");
  process.exit(1);
}

const tenant = new FakeTenant();
tenant.resourceServers.set("rs_seed", {
  id: "rs_seed",
  name: "Auth0 My Account API",
  identifier: `https://${TENANT}/me/`,
});
tenant.forms.set("ap_0000000000000000000001", { id: "ap_0000000000000000000001", name: FORM_NAMES.personalInfo, document: {} });

// Business data: two users share an email, so the picker shows up, and only one
// of them belongs to an organization that has a connection and a role.
tenant.connections.set("con_db", { id: "con_db", name: "Username-Password-Authentication", strategy: "auth0" });
tenant.users.set("auth0|000000000000000000000001", { user_id: "auth0|000000000000000000000001", email: "jane@acme.com", name: "Jane Doe" });
tenant.users.set("google-oauth2|100000000000000000001", { user_id: "google-oauth2|100000000000000000001", email: "jane@acme.com", name: "Jane Doe" });
tenant.organizations.set("org_PreviewAcmeCorp1", { id: "org_PreviewAcmeCorp1", name: "acme-corp", display_name: "Acme Corp" });
tenant.members.set("org_PreviewAcmeCorp1", new Set(["google-oauth2|100000000000000000001"]));
tenant.orgConnections.set("org_PreviewAcmeCorp1", ["con_db"]);
tenant.roles.set("rol_PreviewAdmin0001", { id: "rol_PreviewAdmin0001", name: "Organization - Admin" });
tenant.orgRoles.set("org_PreviewAcmeCorp1:google-oauth2|100000000000000000001", ["rol_PreviewAdmin0001"]);

// A member with no role, and an organization with no enabled connection.
tenant.users.set("auth0|000000000000000000000002", { user_id: "auth0|000000000000000000000002", email: "ben@acme.com", name: "Ben Ortiz" });
tenant.members.get("org_PreviewAcmeCorp1").add("auth0|000000000000000000000002");
tenant.organizations.set("org_PreviewAcmeLabs1", { id: "org_PreviewAcmeLabs1", name: "acme-labs", display_name: "Acme Labs" });
tenant.users.set("auth0|000000000000000000000003", { user_id: "auth0|000000000000000000000003", email: "sam@acme.com", name: "Sam Lee" });
tenant.members.set("org_PreviewAcmeLabs1", new Set(["auth0|000000000000000000000003"]));

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
  openBrowser: (url) => console.error(`(preview: the browser is not opened: ${url})`),
};

const code = await main(["setup"], env, io);
console.error(`(preview: exit ${code})`);
