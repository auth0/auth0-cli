import { strict as assert } from "node:assert";
import { test } from "node:test";

import { RunCommand } from "../src/cli-api.js";
import { apiFor, readContext } from "../src/context.js";
import { PluginError } from "../src/plugin-error.js";

const PROXY_ENV = {
  AUTH0_CLI_PROXY_URL: "https://127.0.0.1:1",
  AUTH0_CLI_PROXY_TOKEN: "proxy-token",
  AUTH0_CLI_TENANT_DOMAIN: "acme.us.auth0.com",
};

// `auth0 tenants list --json`, recorded from the CLI with the names replaced.
const TENANTS = JSON.stringify([
  { active: false, name: "other.eu.auth0.com" },
  { active: true, name: "acme.us.auth0.com" },
]);

function runner(result: { exitCode?: number; stdout?: string; stderr?: string } = {}) {
  const calls: { bin: string; args: string[] }[] = [];
  const run: RunCommand = async (bin, args) => {
    calls.push({ bin, args });
    return { exitCode: 0, stdout: "", stderr: "", ...result };
  };
  return { run, calls };
}

test("started by the framework, the proxy variables are used as they are", async () => {
  const { run, calls } = runner();

  const context = await readContext(PROXY_ENV, undefined, run);

  assert.deepEqual(context, {
    tenantDomain: "acme.us.auth0.com",
    transport: { kind: "proxy", url: "https://127.0.0.1:1", token: "proxy-token" },
  });
  assert.equal(calls.length, 0, "nothing needs to be asked of the CLI");
});

test("a half-set proxy environment names every variable that is missing", async () => {
  await assert.rejects(readContext({ AUTH0_CLI_PROXY_TOKEN: "t" }), (error: unknown) => {
    assert.ok(error instanceof Error);
    assert.match(error.message, /AUTH0_CLI_PROXY_URL/);
    assert.match(error.message, /AUTH0_CLI_TENANT_DOMAIN/);
    assert.doesNotMatch(error.message, /AUTH0_CLI_PROXY_TOKEN/, "the variable that is set is not listed as missing");
    assert.match(error.message, /Auth0 CLI/);
    return true;
  });
});

test("started by the CLI with its path and tenant, nothing is discovered", async () => {
  const { run, calls } = runner();

  const context = await readContext(
    { AUTH0_CLI_BIN: "/opt/bin/auth0", AUTH0_CLI_TENANT_DOMAIN: "acme.us.auth0.com" },
    undefined,
    run,
  );

  assert.deepEqual(context, { tenantDomain: "acme.us.auth0.com", transport: { kind: "cli", bin: "/opt/bin/auth0" } });
  assert.equal(calls.length, 0);
});

test("the --tenant flag wins over the tenant in the environment", async () => {
  const context = await readContext(
    { AUTH0_CLI_BIN: "auth0", AUTH0_CLI_TENANT_DOMAIN: "acme.us.auth0.com" },
    "other.eu.auth0.com",
    runner().run,
  );

  assert.equal(context.tenantDomain, "other.eu.auth0.com");
});

test("run by hand, the CLI's active tenant is used", async () => {
  const { run, calls } = runner({ stdout: TENANTS });

  const context = await readContext({}, undefined, run);

  assert.equal(context.tenantDomain, "acme.us.auth0.com");
  assert.deepEqual(context.transport, { kind: "cli", bin: "auth0" });
  assert.deepEqual(calls, [{ bin: "auth0", args: ["tenants", "list", "--json", "--no-color", "--no-input"] }]);
});

test("run by hand with --tenant, the CLI is not asked which tenant is active", async () => {
  const { run, calls } = runner({ stdout: TENANTS });

  const context = await readContext({}, "other.eu.auth0.com", run);

  assert.equal(context.tenantDomain, "other.eu.auth0.com");
  assert.equal(calls.length, 0);
});

test("with no active tenant, or no login at all, the user is asked to log in", async () => {
  const cases = [
    runner({ stdout: JSON.stringify([{ active: false, name: "acme.us.auth0.com" }]) }),
    runner({ stdout: "[]" }),
    runner({ exitCode: 1, stderr: "\n=== error\n\n ▸    config.json file is missing.\n\n" }),
  ];

  for (const { run } of cases) {
    await assert.rejects(readContext({}, undefined, run), (error: unknown) => {
      assert.ok(error instanceof PluginError);
      assert.equal(error.code, "auth0_login_required");
      assert.deepEqual(error.fixes, ["auth0 login"]);
      return true;
    });
  }
});

test("the API client follows the transport: proxy with the token, CLI with the tenant", async () => {
  const requests: string[] = [];
  const fetchImpl = async (url: string, init: RequestInit) => {
    requests.push(`${url} ${(init.headers as Record<string, string>).authorization}`);
    return new Response("{}", { status: 200 });
  };
  const proxy = apiFor(await readContext(PROXY_ENV), { fetchImpl });
  await proxy.request("GET", "clients");
  assert.deepEqual(requests, ["https://127.0.0.1:1/api/v2/clients Bearer proxy-token"]);

  const { run, calls } = runner({ stdout: "{}" });
  const cli = apiFor(await readContext({ AUTH0_CLI_BIN: "auth0", AUTH0_CLI_TENANT_DOMAIN: "acme.us.auth0.com" }), { run });
  await cli.request("GET", "clients");
  assert.deepEqual(calls[0].args.slice(-2), ["--tenant", "acme.us.auth0.com"]);
});
