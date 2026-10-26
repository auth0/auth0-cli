import { strict as assert } from "node:assert";
import { readFileSync } from "node:fs";
import { test } from "node:test";

// The lockfile is committed to a public repository, so every package URL must be the public
// registry. Installs still work behind a mirror, because npm swaps the host it is configured for.
test("every package in the lockfile resolves from the public registry", () => {
  const lockfile = JSON.parse(readFileSync(new URL("../../package-lock.json", import.meta.url), "utf8"));
  const resolved = Object.entries<{ resolved?: string }>(lockfile.packages)
    .filter(([name, entry]) => name !== "" && entry.resolved !== undefined)
    .map(([name, entry]) => ({ name, url: entry.resolved as string }));

  assert.ok(resolved.length > 0, "the lockfile lists no packages");
  const elsewhere = resolved.filter(({ url }) => !url.startsWith("https://registry.npmjs.org/"));
  assert.deepEqual(
    elsewhere.map(({ name }) => name),
    [],
    "these packages resolve from a host other than the public registry",
  );
});
