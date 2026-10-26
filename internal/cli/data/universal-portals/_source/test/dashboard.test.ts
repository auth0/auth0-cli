import { strict as assert } from "node:assert";
import { test } from "node:test";

import { applicationUrl, tenantLocation } from "../src/dashboard.js";

test("a tenant with a region code maps to its manage host, region and name", () => {
  assert.deepEqual(tenantLocation("acme.us.auth0.com"), {
    base: "https://manage.auth0.com",
    region: "us",
    name: "acme",
  });
});

test("a tenant without a region code is treated as us", () => {
  assert.deepEqual(tenantLocation("dev-tti06f6y.auth0.com"), {
    base: "https://manage.auth0.com",
    region: "us",
    name: "dev-tti06f6y",
  });
});

test("the application link opens the application's settings in the dashboard", () => {
  assert.equal(
    applicationUrl("acme.eu.auth0.com", "abc123"),
    "https://manage.auth0.com/dashboard/eu/acme/applications/abc123/settings",
  );
});
