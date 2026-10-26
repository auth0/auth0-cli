import { strict as assert } from "node:assert";
import { test } from "node:test";

import { scopeFixes } from "../src/scope-fixes.js";

test("a missing scope has one fix for a user login and one for a client-credentials login", () => {
  assert.deepEqual(scopeFixes(["read:portals", "create:portals"]), [
    "Signed in as a user: auth0 login --scopes read:portals,create:portals",
    "Signed in with client credentials: add read:portals, create:portals to the application's Management API grant",
  ]);
});
