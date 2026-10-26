import { strict as assert } from "node:assert";
import { test } from "node:test";
import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, symlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { PassThrough } from "node:stream";
import { fileURLToPath } from "node:url";

import { ApiError, FetchLike } from "../src/api.js";
import { main, IoStreams } from "../src/cli.js";
import { missingFlags, parseSetupArgs, UsageError } from "../src/args.js";
import { toPortalSlug } from "../src/slug.js";
import type { Prompter } from "../src/prompt.js";
import { captureOutput } from "./capture-output.js";
import { FakeTenant } from "./fake-tenant.js";

const DOMAIN = "acme.us.auth0.com";
const ENV = {
  AUTH0_CLI_PROXY_URL: "https://127.0.0.1:4433",
  AUTH0_CLI_PROXY_TOKEN: "proxy-token",
  AUTH0_CLI_TENANT_DOMAIN: DOMAIN,
};

// Serves the fake tenant over the same HTTP shape the proxy uses, so the CLI's
// real HTTP client is exercised.
function fetchFor(tenant: FakeTenant): FetchLike {
  return async (url, init) => {
    const parsed = new URL(url);
    const path = parsed.pathname.replace(/^\/api\/v2\//, "") + parsed.search;
    const body = init.body ? JSON.parse(String(init.body)) : undefined;
    try {
      const result = await tenant.request(init.method as "GET", path, body);
      return new Response(result === undefined ? "" : JSON.stringify(result), { status: 200 });
    } catch (error) {
      if (error instanceof ApiError) {
        return new Response(JSON.stringify(error.body), { status: error.status });
      }
      throw error;
    }
  };
}

function captureIo(fetchImpl: FetchLike, isTTY = false): { io: IoStreams; stdout: () => string; stderr: () => string } {
  let out = "";
  const captureErr = captureOutput();
  const io: IoStreams = {
    stdout: { write: (text) => (out += text) },
    stderr: captureErr.stream,
    stdinStream: process.stdin,
    isTTY,
    fetchImpl,
    // A test must never start the real auth0 CLI: it would call a real tenant. Tests that
    // need the CLI pass their own runner.
    run: async () => {
      throw new Error("a test tried to start the real auth0 CLI");
    },
  };
  return { io, stdout: () => out, stderr: () => captureErr.text() };
}

// Polls until the condition holds, or resolves false after the timeout.
async function waitFor(condition: () => boolean, timeoutMs: number): Promise<boolean> {
  const deadline = Date.now() + timeoutMs;
  while (!condition()) {
    if (Date.now() > deadline) return false;
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
  return true;
}

test("parses the setup flags", () => {
  const args = parseSetupArgs([
    "setup", "--template", "business", "--name", "Acme Admin",
    "--user", "jane@acme.com", "--json",
  ]);
  assert.equal(args.command, "setup");
  assert.equal(args.template, "business");
  assert.equal(args.name, "Acme Admin");
  assert.equal(args.user, "jane@acme.com");
  assert.equal(args.json, true);
});

test("rejects an unknown template and an unknown command", () => {
  assert.throws(() => parseSetupArgs(["setup", "--template", "both"]), UsageError);
  assert.throws(() => parseSetupArgs(["publish"]), /Unknown command "publish"/);
});

test("audit takes exactly one portal ID, plus --organization and --strict", () => {
  const args = parseSetupArgs(["audit", "up_123", "--organization", "org_1", "--strict"]);
  assert.equal(args.command, "audit");
  assert.equal(args.portalId, "up_123");
  assert.equal(args.organization, "org_1");
  assert.equal(args.strict, true);
  assert.throws(() => parseSetupArgs(["audit"]), /Usage: auth0 universal-portals audit/);
});

test("no command prints help", () => {
  assert.equal(parseSetupArgs([]).command, "help");
  assert.equal(parseSetupArgs(["--help"]).command, "help");
});

test("missingFlags names what a non-interactive run still needs", () => {
  const args = parseSetupArgs(["setup", "--template", "business"]);
  assert.deepEqual(missingFlags(args), ["--name", "--user"]);
  assert.deepEqual(missingFlags(parseSetupArgs(["setup", "--template", "consumer", "--name", "x"])), []);
});

test("the default slug is a kebab-case version of the name", () => {
  assert.equal(toPortalSlug("Acme Admin"), "acme-admin");
  assert.equal(toPortalSlug("  Café & Co. 2!  "), "caf-co-2");
  assert.equal(toPortalSlug("123 portal"), "portal");
  assert.equal(toPortalSlug("!!!"), "my-portal");
});

test("--help prints usage and does not touch the tenant", async () => {
  const tenant = new FakeTenant();
  const { io, stdout } = captureIo(fetchFor(tenant));
  const code = await main(["--help"], ENV, io);
  assert.equal(code, 0);
  assert.match(stdout(), /Usage: auth0 universal-portals setup/);
  assert.equal(tenant.calls.length, 0);
});

test("interactive questions are drawn on stderr, never stdout", async () => {
  const keys = new PassThrough();
  const { io, stdout, stderr } = captureIo(fetchFor(new FakeTenant()), true);
  io.stdinStream = keys as unknown as NodeJS.ReadStream;

  const run = main(["setup"], ENV, io);
  const drawn = await waitFor(() => stderr().includes("Which kind of portal"), 2000);
  assert.ok(drawn, "the first question was not drawn on stderr");
  keys.write("\x03");

  assert.equal(await run, 130);
  assert.equal(stdout(), "");
});

// Answers each question once it is drawn on stderr, in the order given.
async function answerInOrder(keys: PassThrough, stderr: () => string, answers: [question: string, keys: string][]) {
  for (const [question, input] of answers) {
    if (!(await waitFor(() => stderr().includes(question), 2000))) {
      throw new Error(`question not drawn: ${question}`);
    }
    keys.write(input);
  }
}

function seedBusinessTenant(tenant: FakeTenant): void {
  tenant.users.set("auth0|jane", { user_id: "auth0|jane", email: "jane@acme.com", name: "Jane" });
  tenant.connections.set("con_db", { id: "con_db", name: "Username-Password-Authentication", strategy: "auth0" });
}

test("cancelling after an organization was created says what was kept", async () => {
  const tenant = new FakeTenant();
  seedBusinessTenant(tenant);
  const keys = new PassThrough();
  const { io, stderr } = captureIo(fetchFor(tenant), true);
  io.stdinStream = keys as unknown as NodeJS.ReadStream;

  const run = main(
    ["setup", "--template", "business", "--name", "Acme Admin", "--slug", "acme-admin", "--user", "auth0|jane"],
    ENV,
    io,
  );
  await answerInOrder(keys, stderr, [
    ["Choose the organization to test the portal in", "\r"],
    ["Organization display name", "Acme Corp\r"],
    ["Organization name", "\r"],
    ["enable a database connection", "\r"],
    ["No role in this tenant", "\x03"],
  ]);

  assert.equal(await run, 130);
  assert.equal(tenant.organizations.size, 1);
  assert.match(stderr(), /Nothing was rolled back/);
  assert.doesNotMatch(stderr(), /Nothing was created/);
});

test("cancelling before any write says nothing was created", async () => {
  const tenant = new FakeTenant();
  seedBusinessTenant(tenant);
  const keys = new PassThrough();
  const { io, stderr } = captureIo(fetchFor(tenant), true);
  io.stdinStream = keys as unknown as NodeJS.ReadStream;

  const run = main(
    ["setup", "--template", "business", "--name", "Acme Admin", "--slug", "acme-admin", "--user", "auth0|jane"],
    ENV,
    io,
  );
  await answerInOrder(keys, stderr, [
    ["Choose the organization to test the portal in", "\r"],
    ["Organization display name", "\x03"],
  ]);

  assert.equal(await run, 130);
  assert.equal(tenant.organizations.size, 0);
  assert.match(stderr(), /Nothing was created/);
});

const LEGACY_LINK = { type: "sidebar:component:auth0:internal_link", config: { label: "Profile", to: "profile", icon: "user" } };

// A consumer portal from setup, with its application.
async function tenantWithPortal(): Promise<{ tenant: FakeTenant; portalId: string; clientId: string }> {
  const tenant = new FakeTenant();
  tenant.connections.set("con_db", { id: "con_db", name: "Username-Password-Authentication", strategy: "auth0" });
  const { io } = captureIo(fetchFor(tenant));
  const code = await main(["setup", "--no-input", "--template", "consumer", "--name", "Acme Account", "--slug", "acme-account"], ENV, io);
  assert.equal(code, 0);
  const portalId = [...tenant.portals.keys()][0];
  return { tenant, portalId, clientId: tenant.portals.get(portalId)!.client.client_id };
}

// Answers the fix menu by position (0 all, 1 choose, 2 skip) and keeps the checked
// errors whose IDs are listed.
function fixMenu(position: number, keep: string[] = []): Prompter {
  return {
    ask: async () => {
      throw new Error("unexpected question");
    },
    confirm: async () => {
      throw new Error("unexpected question");
    },
    choose: async <T>(_question: string, options: { value: T }[]): Promise<T> => options[position].value,
    checklist: async <T>(_question: string, items: { label: string; value: T }[]): Promise<T[]> =>
      items.filter((item) => keep.includes(item.label.split(" ")[0])).map((item) => item.value),
  };
}

async function auditInTerminal(tenant: FakeTenant, portalId: string, prompter: Prompter): Promise<{ code: number; stderr: string }> {
  const { io, stderr } = captureIo(fetchFor(tenant), true);
  io.prompter = prompter;
  const code = await main(["audit", portalId], ENV, io);
  return { code, stderr: stderr() };
}

test("the fix menu's 'fix all' applies every error and the portal passes the audit", async () => {
  const { tenant, portalId } = await tenantWithPortal();
  tenant.portals.get(portalId)!.navigation.sidebar.components = [LEGACY_LINK];

  const { code } = await auditInTerminal(tenant, portalId, fixMenu(0));

  assert.equal(code, 0);
  const links = tenant.portals.get(portalId)!.navigation.sidebar.components;
  assert.ok(links.every((link: { type: string }) => link.type.endsWith(":v2")));
});

test("choosing errors applies only the checked ones, and a later error is asked about again", async () => {
  const { tenant, portalId, clientId } = await tenantWithPortal();
  tenant.portals.get(portalId)!.navigation.sidebar.components = [LEGACY_LINK];
  tenant.portals.get(portalId)!.client = undefined;
  tenant.clients.delete(clientId);
  const clientsBefore = tenant.clients.size;

  const { code } = await auditInTerminal(tenant, portalId, fixMenu(1, ["P003"]));

  assert.equal(code, 1, "the unchecked application error is still reported");
  assert.ok(tenant.portals.get(portalId)!.navigation.sidebar.components.every((link: { type: string }) => link.type.endsWith(":v2")));
  assert.equal(tenant.portals.get(portalId)!.client, undefined, "the unchecked error was not fixed");
  assert.equal(tenant.clients.size, clientsBefore, "no application was created");
});

test("skipping the fixes leaves the portal as it is", async () => {
  const { tenant, portalId } = await tenantWithPortal();
  tenant.portals.get(portalId)!.navigation.sidebar.components = [LEGACY_LINK];

  const { code } = await auditInTerminal(tenant, portalId, fixMenu(2));

  assert.equal(code, 1, "the error is still reported");
  assert.equal(tenant.portals.get(portalId)!.navigation.sidebar.components[0].type, LEGACY_LINK.type);
});

test("a failure after resources were created says that a rerun reuses them", async () => {
  const tenant = new FakeTenant();
  tenant.connections.set("con_db", { id: "con_db", name: "Username-Password-Authentication", strategy: "auth0" });
  tenant.failNext("POST portals", 500);
  const { io, stderr } = captureIo(fetchFor(tenant));

  const code = await main(["setup", "--no-input", "--template", "consumer", "--name", "Acme Account", "--slug", "acme-account"], ENV, io);

  assert.equal(code, 1);
  assert.ok(tenant.clients.size > 0, "resources were created before the failure");
  assert.match(stderr(), /Run the same command again to resume\. Existing resources are reused\./);
});

test("a slug taken during setup keeps its own message, without the rerun hint", async () => {
  const tenant = new FakeTenant();
  tenant.connections.set("con_db", { id: "con_db", name: "Username-Password-Authentication", strategy: "auth0" });
  tenant.failNext("POST portals", 409);
  const { io, stderr } = captureIo(fetchFor(tenant));

  const code = await main(["setup", "--no-input", "--template", "consumer", "--name", "Acme Account", "--slug", "acme-account"], ENV, io);

  assert.equal(code, 1);
  assert.match(stderr(), /taken while setup was running/);
  assert.doesNotMatch(stderr(), /Existing resources are reused/);
});

// The recorded --json output of a consumer setup. Set UPDATE_GOLDEN=1 to record it again
// after an intended change to the output, and review the diff.
const CONSUMER_SETUP_JSON = fileURLToPath(new URL("../../test/fixtures/consumer-setup.json", import.meta.url));

test("a consumer setup prints the same JSON, byte for byte, as the recorded output", async () => {
  const tenant = new FakeTenant();
  tenant.connections.set("con_db", { id: "con_db", name: "Username-Password-Authentication", strategy: "auth0" });
  const { io, stdout } = captureIo(fetchFor(tenant));

  const code = await main(
    ["setup", "--no-input", "--json", "--template", "consumer", "--name", "Acme Account", "--slug", "acme-account"],
    ENV,
    io,
  );

  assert.equal(code, 0);
  if (process.env.UPDATE_GOLDEN === "1") writeFileSync(CONSUMER_SETUP_JSON, stdout());
  assert.equal(stdout(), readFileSync(CONSUMER_SETUP_JSON, "utf8"));
});

test("a non-interactive run without the required flags stops before any API call", async () => {
  const tenant = new FakeTenant();
  const { io, stderr } = captureIo(fetchFor(tenant));
  const code = await main(["setup", "--no-input", "--template", "consumer"], ENV, io);

  assert.equal(code, 2);
  assert.match(stderr(), /--name/);
  assert.equal(tenant.calls.length, 0);
});

test("a run without the Auth0 CLI installed says how to install it, and calls nothing", async () => {
  const tenant = new FakeTenant();
  const { io, stderr } = captureIo(fetchFor(tenant));
  io.run = async () => {
    throw Object.assign(new Error("spawn auth0 ENOENT"), { code: "ENOENT" });
  };
  const code = await main(["setup", "--no-input", "--template", "consumer", "--name", "x"], {}, io);

  assert.equal(code, 1);
  assert.match(stderr(), /The Auth0 CLI was not found/);
  assert.match(stderr(), /AUTH0_CLI_BIN/);
  assert.equal(tenant.calls.length, 0);
});

test("a half-set proxy environment stops the run before any API call", async () => {
  const tenant = new FakeTenant();
  const { io, stderr } = captureIo(fetchFor(tenant));
  const code = await main(["setup", "--no-input", "--template", "consumer", "--name", "x"], { AUTH0_CLI_PROXY_TOKEN: "t" }, io);

  assert.equal(code, 1);
  assert.match(stderr(), /Missing: AUTH0_CLI_PROXY_URL, AUTH0_CLI_TENANT_DOMAIN/);
  assert.equal(tenant.calls.length, 0);
});

test("a non-interactive consumer run prints the result as JSON and never the full secret", async () => {
  const tenant = new FakeTenant();
  const { io, stdout, stderr } = captureIo(fetchFor(tenant));
  const code = await main(
    ["setup", "--no-input", "--json", "--template", "consumer", "--name", "Acme Account"],
    ENV,
    io,
  );

  assert.equal(code, 0);
  const result = JSON.parse(stdout());
  assert.equal(result.portal.url, `https://${DOMAIN}/portals/acme-account`);
  const secret = [...tenant.clients.values()].find((client) => client.name?.startsWith("Acme Account"))?.client_secret;
  assert.ok(secret);
  assert.ok(!stdout().includes(secret) && !stderr().includes(secret), "the full secret must not be printed");
});

test("in agent mode a missing scope is reported as a JSON error on stderr", async () => {
  const tenant = new FakeTenant();
  tenant.deny("POST portals", "Insufficient scope, expected any of: create:portals");
  const { io, stdout, stderr } = captureIo(fetchFor(tenant));
  const code = await main(
    ["setup", "--json", "--template", "consumer", "--name", "Acme Account"],
    ENV,
    io,
  );

  assert.equal(code, 1);
  assert.equal(stdout(), "");
  const envelope = JSON.parse(stderr());
  assert.equal(envelope.error, "missing_scopes");
  assert.deepEqual(envelope.fixes, ["auth0 login --scopes create:portals"]);
});

test("a Business run that is missing its user stops with a usage error", async () => {
  const { io, stderr } = captureIo(fetchFor(new FakeTenant()));
  const code = await main(
    ["setup", "--no-input", "--template", "business", "--name", "Acme Admin"],
    ENV,
    io,
  );
  assert.equal(code, 2);
  assert.match(stderr(), /--user/);
});

test("the entry point runs when started through a symlink, as npx starts a plugin", () => {
  const entry = fileURLToPath(new URL("../src/cli.js", import.meta.url));
  const link = join(mkdtempSync(join(tmpdir(), "up-bin-")), "universal-portals");
  symlinkSync(entry, link);

  const output = execFileSync(process.execPath, [link, "--help"], { encoding: "utf8" });
  assert.match(output, /Usage: auth0 universal-portals setup/);
});

test("the CLI's global flags are accepted and --tenant is returned for the check", () => {
  const args = parseSetupArgs(["setup", "--tenant", "acme.us.auth0.com", "--debug", "--no-color", "--template", "consumer", "--name", "x"]);
  assert.equal(args.tenant, "acme.us.auth0.com");
  assert.equal(args.command, "setup");
});

test("a --tenant that differs from the CLI's tenant stops the run before any API call", async () => {
  const tenant = new FakeTenant();
  const { io, stderr } = captureIo(fetchFor(tenant));
  const code = await main(
    ["setup", "--tenant", "other.us.auth0.com", "--no-input", "--template", "consumer", "--name", "x"],
    ENV,
    io,
  );
  assert.equal(code, 2);
  assert.match(stderr(), /does not match the tenant/);
  assert.equal(tenant.calls.length, 0);
});
