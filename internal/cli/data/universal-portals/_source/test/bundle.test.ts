import { strict as assert } from "node:assert";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

import { sourceHash } from "../src/source-hash.js";

// The Auth0 CLI embeds this file, so a bundle that is older than its sources would ship old code.
test("the committed bundle was built from the current sources", () => {
  const root = fileURLToPath(new URL("../../", import.meta.url));
  const bundle = readFileSync(new URL("../../../universal-portals.mjs", import.meta.url), "utf8");

  const recorded = /^\/\/ source-sha256: ([0-9a-f]{64})$/m.exec(bundle.split("\n").slice(0, 5).join("\n"))?.[1];

  assert.ok(recorded, "the bundle does not start with its source hash");
  assert.equal(
    recorded,
    sourceHash(root),
    'the sources changed after the bundle was built: run "npm run bundle" in the _source folder and commit the result',
  );
});
