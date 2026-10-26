import { strict as assert } from "node:assert";
import { chmodSync, mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";

import { ApiError } from "../src/api.js";
import { CommandResult, createCliApiClient, RunCommand } from "../src/cli-api.js";
import { PluginError } from "../src/plugin-error.js";

// What `auth0 api` prints on stderr when a request fails. The 400, 404 and unknown-tenant
// samples were recorded from the CLI (v1.32.0) with the tenant name replaced. The 403 and 409
// samples are built from the CLI's own message formats, because the recording tenant's token
// had no missing scope and a conflict needs a write.
const STDERR = {
  notFound:
    "\n=== acme.us.auth0.com error\n\n ▸    404: API request failed: " +
    '{"statusCode":404,"error":"Not Found","message":"The client does not exist","errorCode":"inexistent_client"}.\n\n',
  badQuery:
    "\n=== acme.us.auth0.com error\n\n ▸    400: API request failed: " +
    `{"statusCode":400,"error":"Bad Request","message":"Query validation error: 'Value 9999 is greater than maximum 100' on property per_page (Number of results per page. Default value is 50, maximum value is 100).","errorCode":"invalid_query_string"}.\n\n`,
  conflict:
    "\n=== acme.us.auth0.com error\n\n ▸    409: API request failed: " +
    '{"statusCode":409,"error":"Conflict","message":"Slug already in use","errorCode":"portal_conflict"}.\n\n',
  missingScope:
    "\n=== acme.us.auth0.com error\n\n ▸    Request failed because access token lacks scope: read:portals.\n " +
    "If authenticated via client credentials, add this scope to the designated client. " +
    "If authenticated as a user, request this scope during login by running `auth0 login --scopes read:portals`.\n\n",
  unknownTenant:
    "\n=== nope.us.auth0.com error\n\n ▸    Failed to find tenant: nope.us.auth0.com. " +
    "Run 'auth0 tenants list' to see your configured tenants or run 'auth0 login' to configure a new tenant.\n\n",
  expiredSession:
    "\n=== acme.us.auth0.com error\n\n ▸    Auth token expired and --no-input is set; run 'auth0 login' to re-authenticate.\n\n",
  somethingElse: "\n=== acme.us.auth0.com error\n\n ▸    Failed to send request: dial tcp: lookup acme.us.auth0.com: no such host.\n\n",
};

interface Call {
  bin: string;
  args: string[];
  input: string | undefined;
}

function runner(result: Partial<CommandResult>): { run: RunCommand; calls: Call[] } {
  const calls: Call[] = [];
  const run: RunCommand = async (bin, args, input) => {
    calls.push({ bin, args, input });
    return { exitCode: 0, stdout: "", stderr: "", ...result };
  };
  return { run, calls };
}

async function failure(stderr: string): Promise<unknown> {
  const { run } = runner({ exitCode: 1, stderr });
  const api = createCliApiClient({ bin: "auth0", tenant: "acme.us.auth0.com", run });
  try {
    await api.request("GET", "clients/x");
  } catch (error) {
    return error;
  }
  throw new Error("the request was expected to fail");
}

test("a successful request returns the parsed JSON from stdout", async () => {
  const { run, calls } = runner({ stdout: '{\n  "client_id": "abc"\n}\n' });
  const api = createCliApiClient({ bin: "auth0", tenant: "acme.us.auth0.com", run });

  const result = await api.request("GET", "clients/abc?fields=client_id");

  assert.deepEqual(result, { client_id: "abc" });
  assert.deepEqual(calls[0].args, [
    "api", "GET", "clients/abc?fields=client_id", "--no-color", "--no-input", "--tenant", "acme.us.auth0.com",
  ]);
});

test("an empty response is undefined, like a 204 from the API", async () => {
  const { run } = runner({ stdout: "" });
  const api = createCliApiClient({ bin: "auth0", run });

  assert.equal(await api.request("PATCH", "clients/abc", { name: "x" }), undefined);
});

test("the request body travels on stdin and never in the arguments", async () => {
  const secret = "s3cr3t-value-that-must-not-leak";
  const { run, calls } = runner({ stdout: "{}" });
  const api = createCliApiClient({ bin: "auth0", run });

  await api.request("POST", "flows/vault/connections", { setup: { client_secret: secret } });

  assert.equal(calls[0].input, JSON.stringify({ setup: { client_secret: secret } }));
  assert.ok(!calls[0].args.join(" ").includes(secret), "the secret must not appear in the process arguments");
});

test("a request without a body sends nothing on stdin, a leading slash is dropped, and DELETE is forced", async () => {
  const { run, calls } = runner({});
  const api = createCliApiClient({ bin: "auth0", run });

  await api.request("DELETE", "/client-grants/cgr_1");

  assert.equal(calls[0].input, undefined);
  assert.deepEqual(calls[0].args, ["api", "DELETE", "client-grants/cgr_1", "--no-color", "--no-input", "--force"]);
});

test("a 404 becomes an ApiError with the status, code and message from the API", async () => {
  const error = await failure(STDERR.notFound);

  assert.ok(error instanceof ApiError);
  assert.equal(error.status, 404);
  assert.equal(error.errorCode, "inexistent_client");
  assert.equal(error.message, "The client does not exist");
});

test("a 400 and a 409 keep their status and error code", async () => {
  const bad = await failure(STDERR.badQuery);
  const conflict = await failure(STDERR.conflict);

  assert.ok(bad instanceof ApiError && conflict instanceof ApiError);
  assert.deepEqual([bad.status, bad.errorCode], [400, "invalid_query_string"]);
  assert.deepEqual([conflict.status, conflict.errorCode], [409, "portal_conflict"]);
});

test("an API error whose message quotes a missing scope stays an API error", async () => {
  const error = await failure(
    "\n=== acme.us.auth0.com error\n\n ▸    400: API request failed: " +
      '{"statusCode":400,"error":"Bad Request","message":"Invalid description: access token lacks scope: read:portals","errorCode":"invalid_body"}.\n\n',
  );

  assert.ok(error instanceof ApiError);
  assert.deepEqual([error.status, error.errorCode], [400, "invalid_body"]);
});

test("an API error body printed over several lines keeps its status, code and message", async () => {
  const body = JSON.stringify(
    { statusCode: 404, error: "Not Found", message: "The portal does not exist", errorCode: "inexistent_portal" },
    null,
    2,
  );
  const error = await failure(`\n=== acme.us.auth0.com error\n\n ▸    404: API request failed: ${body}.\n\n`);

  assert.ok(error instanceof ApiError);
  assert.deepEqual([error.status, error.errorCode, error.message], [404, "inexistent_portal", "The portal does not exist"]);
});

test("a number that only ends in three digits is not read as a status", async () => {
  const error = await failure("\n=== acme.us.auth0.com error\n\n ▸    1404: API request failed: {}.\n\n");

  assert.ok(!(error instanceof ApiError), "1404 is not an HTTP status");
});

test("a missing scope becomes the 403 the preflight already understands", async () => {
  const error = await failure(STDERR.missingScope);

  assert.ok(error instanceof ApiError);
  assert.equal(error.status, 403);
  assert.equal(error.errorCode, "insufficient_scope");
  assert.equal(error.message, "Insufficient scope, expected any of: read:portals");
});

test("an expired session or an unknown tenant asks the user to log in", async () => {
  const cases: [string, RegExp][] = [
    [STDERR.expiredSession, /Auth token expired/],
    [STDERR.unknownTenant, /Failed to find tenant: nope\.us\.auth0\.com/],
  ];
  for (const [stderr, reason] of cases) {
    const error = await failure(stderr);
    assert.ok(error instanceof PluginError);
    assert.equal(error.code, "auth0_login_required");
    assert.deepEqual(error.fixes, ["auth0 login"]);
    assert.match(error.message, reason, "the CLI's own reason is kept");
  }
});

test("any other failure is reported with the CLI's own message", async () => {
  const error = await failure(STDERR.somethingElse);

  assert.ok(error instanceof Error && !(error instanceof ApiError));
  assert.match(error.message, /auth0 api GET clients\/x failed: Failed to send request: dial tcp/);
});

test("a missing auth0 binary tells the user how to install it or point to it", async () => {
  const api = createCliApiClient({ bin: join(tmpdir(), "no-such-auth0-binary") });

  await assert.rejects(api.request("GET", "clients"), (error: unknown) => {
    assert.ok(error instanceof PluginError);
    assert.equal(error.code, "auth0_cli_not_found");
    assert.match(error.message, /AUTH0_CLI_BIN/);
    return true;
  });
});

test("the first request runs alone, then up to four run together", async () => {
  let active = 0;
  let peak = 0;
  const peaks: number[] = [];
  const run: RunCommand = async () => {
    active += 1;
    peak = Math.max(peak, active);
    await new Promise((resolve) => setTimeout(resolve, 15));
    peaks.push(peak);
    active -= 1;
    return { exitCode: 0, stdout: "{}", stderr: "" };
  };
  const api = createCliApiClient({ bin: "auth0", run, concurrency: 4 });

  await Promise.all(Array.from({ length: 9 }, (_, index) => api.request("GET", `clients/${index}`)));

  assert.equal(peaks[0], 1, "the first request must finish before any other starts");
  assert.equal(peak, 4);
});

test("the real runner passes the body on stdin to the process and reads its stdout", async () => {
  const dir = mkdtempSync(join(tmpdir(), "up-fake-auth0-"));
  const script = join(dir, "auth0");
  // A stand-in for the CLI: echoes its arguments and the stdin it received.
  writeFileSync(
    script,
    '#!/usr/bin/env node\nlet input = "";\nprocess.stdin.on("data", (c) => (input += c));\n' +
      'process.stdin.on("end", () => console.log(JSON.stringify({ args: process.argv.slice(2), input })));\n',
  );
  chmodSync(script, 0o755);
  const api = createCliApiClient({ bin: script, tenant: "acme.us.auth0.com" });

  const echoed = await api.request<{ args: string[]; input: string }>("POST", "clients", { name: "x" });

  assert.deepEqual(JSON.parse(echoed.input), { name: "x" });
  assert.deepEqual(echoed.args.slice(0, 3), ["api", "POST", "clients"]);
});

// `auth0 api` reads stdin until it ends for every method except GET, even when there is no body. A
// request without a body must still close it, or the command waits forever.
test("a request without a body closes stdin, so a command that reads it to the end does not hang", async () => {
  const dir = mkdtempSync(join(tmpdir(), "up-fake-auth0-"));
  const script = join(dir, "auth0");
  writeFileSync(
    script,
    '#!/usr/bin/env node\nprocess.stdin.resume();\nprocess.stdin.on("end", () => console.log("{}"));\n' +
      // If stdin is never closed, stop on our own, so a failing test does not keep the run alive.
      "setTimeout(() => process.exit(2), 8000).unref();\n",
  );
  chmodSync(script, 0o755);
  const api = createCliApiClient({ bin: script });

  const outcome = await Promise.race([
    api.request("DELETE", "client-grants/cgr_1").then(() => "finished"),
    new Promise<string>((resolve) => setTimeout(() => resolve("hung"), 5000)),
  ]);

  assert.equal(outcome, "finished");
});
