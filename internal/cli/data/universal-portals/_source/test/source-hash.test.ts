import { strict as assert } from "node:assert";
import { createHash } from "node:crypto";
import { mkdirSync, mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { test } from "node:test";

import { sourceHash } from "../src/source-hash.js";

function tree(files: Record<string, string>): string {
  const root = mkdtempSync(join(tmpdir(), "up-source-hash-"));
  for (const [name, text] of Object.entries(files)) {
    mkdirSync(dirname(join(root, name)), { recursive: true });
    writeFileSync(join(root, name), text);
  }
  return root;
}

const FILES = { "package-lock.json": "{}", "scripts/bundle.mjs": "x", "src/a.ts": "a", "src/deep/b.ts": "b" };

// The Go test that guards the committed bundle computes the same value, so this pins the definition:
// sorted paths, each followed by a NUL, its text with LF line endings, and a NUL.
test("the hash covers the lockfile, the bundle script and every file under src, in sorted order", () => {
  const expected = createHash("sha256")
    .update("package-lock.json\0{}\0scripts/bundle.mjs\0x\0src/a.ts\0a\0src/deep/b.ts\0b\0")
    .digest("hex");

  assert.equal(sourceHash(tree(FILES)), expected);
});

test("a change under src changes the hash, a change outside it does not", () => {
  const before = sourceHash(tree(FILES));

  assert.notEqual(sourceHash(tree({ ...FILES, "src/a.ts": "changed" })), before);
  assert.equal(sourceHash(tree({ ...FILES, "test/x.test.ts": "new", "dist/y.js": "z" })), before);
});

test("Windows line endings give the same hash as Unix ones", () => {
  const unix = tree({ ...FILES, "src/a.ts": "line one\nline two\n" });
  const windows = tree({ ...FILES, "src/a.ts": "line one\r\nline two\r\n" });

  assert.equal(sourceHash(windows), sourceHash(unix));
});
