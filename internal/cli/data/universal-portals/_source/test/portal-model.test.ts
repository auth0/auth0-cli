import { strict as assert } from "node:assert";
import { test } from "node:test";

import { isValidPortalSlug } from "../src/portal-model.js";

test("accepts lowercase slugs that start with a letter", () => {
  assert.equal(isValidPortalSlug("acme-admin"), true);
  assert.equal(isValidPortalSlug("a"), true);
});

test("rejects slugs that break the portal pattern", () => {
  assert.equal(isValidPortalSlug("My Portal"), false);
  assert.equal(isValidPortalSlug("1portal"), false);
  assert.equal(isValidPortalSlug("-portal"), false);
  assert.equal(isValidPortalSlug("portal_one"), false);
  assert.equal(isValidPortalSlug(""), false);
});

test("rejects slugs longer than 100 characters", () => {
  assert.equal(isValidPortalSlug(`a${"b".repeat(99)}`), true);
  assert.equal(isValidPortalSlug(`a${"b".repeat(100)}`), false);
});
