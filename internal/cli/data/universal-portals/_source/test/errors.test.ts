import { strict as assert } from "node:assert";
import { test } from "node:test";

import { ApiError } from "../src/api.js";
import { UsageError } from "../src/args.js";
import { describeError } from "../src/errors.js";
import { BusinessAccessError } from "../src/setup/business-types.js";
import { FeatureNotEnabledError } from "../src/setup/preflight.js";
import { MissingScopesError, SlugRaceError } from "../src/setup/run.js";

test("each error reports its own code, and the fixes it names", () => {
  assert.deepEqual(describeError(new UsageError("bad flag")), { code: "usage", message: "bad flag", details: {} });
  assert.deepEqual(describeError(new SlugRaceError("acme")).code, "slug_race");
  assert.deepEqual(describeError(new FeatureNotEnabledError("acme.us.auth0.com")).code, "feature_not_enabled");
  assert.deepEqual(describeError(new BusinessAccessError("no user", ["Rerun with --user."])).details, {
    fixes: ["Rerun with --user."],
  });
  assert.deepEqual(describeError(new MissingScopesError(["read:clients"], [])).details, {
    fixes: ["auth0 login --scopes read:clients"],
  });
});

test("an error with no fixes leaves fixes out of the JSON", () => {
  const { details } = describeError(new MissingScopesError([], []));
  assert.equal("fixes" in details, false);
});

test("an API error keeps the code the API sent", () => {
  const error = new ApiError(403, "insufficient_scope", "Insufficient scope", {});
  assert.deepEqual(describeError(error), { code: "insufficient_scope", message: "Insufficient scope", details: { status: 403 } });
});
