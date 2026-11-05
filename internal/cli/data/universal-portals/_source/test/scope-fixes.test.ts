import { strict as assert } from "node:assert";
import { test } from "node:test";

import { scopeFixes } from "../src/scope-fixes.js";
import { PLUGIN_SCOPES } from "../src/scopes.js";

test("a user login asks for every scope the tool uses, so it does not drop the ones granted before", () => {
  const [login] = scopeFixes(["read:portals"]);
  const scopes = login.replace("Signed in as a user: auth0 login --scopes ", "").split(",");

  assert.deepEqual(scopes, PLUGIN_SCOPES);
});

test("a scope outside the tool's list is still asked for", () => {
  const [login] = scopeFixes(["read:something_new"]);

  assert.ok(login.endsWith(",read:something_new"));
});

test("a client-credentials login only needs the missing scopes added to its grant", () => {
  assert.equal(
    scopeFixes(["read:portals", "create:portals"])[1],
    "Signed in with client credentials: add read:portals, create:portals to the application's Management API grant",
  );
});
