import { strict as assert } from "node:assert";
import { test } from "node:test";

import { ApiClient, ApiError, HttpMethod, toApiError } from "../src/api.js";
import { FeatureNotEnabledError, Probe, choiceGaps, parseScopeGap, requiredMissingScopes, runPreflight } from "../src/setup/preflight.js";

// Answers each probe with the response the scope gate would return, keyed by
// "METHOD path". Anything not listed succeeds.
function tenantWith(answers: Record<string, { status: number; body: unknown }>): ApiClient {
  return {
    async request<T>(method: HttpMethod, path: string): Promise<T> {
      const answer = answers[`${method} ${path}`];
      if (!answer) return undefined as T;
      if (answer.status >= 400) throw toApiError(answer.status, answer.body);
      return answer.body as T;
    },
  };
}

const insufficient = (scopes: string, mode: "any" | "all" = "any") => ({
  status: 403,
  body: {
    statusCode: 403,
    error: "Forbidden",
    message: `Insufficient scope, expected ${mode} of: ${scopes}`,
    errorCode: "insufficient_scope",
  },
});

const PROBES: Probe[] = [
  { name: "create client", method: "POST", path: "clients", body: {} },
  { name: "import forms", method: "POST", path: "forms/import", body: {} },
  { name: "create portal", method: "POST", path: "portals", body: {} },
];

test("a scope the session has is not reported", async () => {
  const result = await runPreflight(tenantWith({}), PROBES, "acme.us.auth0.com");
  assert.deepEqual(result.gaps, []);
  assert.deepEqual(result.unexpected, []);
});

test("a validation error (400) means the scope is present", async () => {
  const api = tenantWith({ "POST clients": { status: 400, body: { message: "Payload validation error" } } });
  const result = await runPreflight(api, PROBES, "acme.us.auth0.com");
  assert.deepEqual(result.gaps, []);
});

test("every missing scope is reported at once, with the requirement", async () => {
  const api = tenantWith({
    "POST clients": insufficient("create:clients"),
    "POST forms/import": insufficient("create:forms,create:flows", "all"),
    "POST portals": insufficient("create:portals"),
  });
  const result = await runPreflight(api, PROBES, "acme.us.auth0.com");

  assert.deepEqual(result.gaps, [
    { probe: "create client", requirement: "any", scopes: ["create:clients"] },
    { probe: "import forms", requirement: "all", scopes: ["create:forms", "create:flows"] },
    { probe: "create portal", requirement: "any", scopes: ["create:portals"] },
  ]);
  assert.deepEqual(requiredMissingScopes(result), ["create:clients", "create:flows", "create:forms", "create:portals"]);
});

test("an 'any of' gap with several options is a choice, not a required scope", async () => {
  const api = tenantWith({ "POST portals": insufficient("create:portals,create:portals_beta") });
  const result = await runPreflight(api, PROBES, "acme.us.auth0.com");
  assert.deepEqual(requiredMissingScopes(result), []);
  assert.equal(choiceGaps(result).length, 1);
});

test("feature_not_enabled stops the preflight with an actionable error", async () => {
  const api = tenantWith({
    "POST portals": {
      status: 403,
      body: { statusCode: 403, message: "Feature not enabled", errorCode: "feature_not_enabled" },
    },
  });
  await assert.rejects(runPreflight(api, PROBES, "acme.us.auth0.com"), (error: unknown) => {
    assert.ok(error instanceof FeatureNotEnabledError);
    assert.match(error.message, /Universal Portals is not enabled for acme\.us\.auth0\.com/);
    assert.match(error.message, /Ask Auth0 to enable it .*run the command again/);
    return true;
  });
});

test("a 403 without a scope message is kept as unexpected, not as a scope gap", async () => {
  const api = tenantWith({ "POST clients": { status: 403, body: { message: "Forbidden" } } });
  const result = await runPreflight(api, PROBES, "acme.us.auth0.com");
  assert.deepEqual(result.gaps, []);
  assert.equal(result.unexpected.length, 1);
  assert.equal(result.unexpected[0].status, 403);
});

test("parseScopeGap ignores errors that are not 403", () => {
  const error = new ApiError(500, undefined, "expected any of: create:clients", {});
  assert.equal(parseScopeGap("create client", error), undefined);
});
