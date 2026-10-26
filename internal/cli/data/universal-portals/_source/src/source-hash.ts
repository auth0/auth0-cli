import { createHash } from "node:crypto";
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";

// Everything the single-file bundle is built from: the sources, the script that builds it, and the
// lockfile that fixes the dependencies. Paths use "/" and are sorted, so the list is the same on
// every machine.
export function sourceFiles(root: string): string[] {
  const files = ["package-lock.json", "scripts/bundle.mjs"];
  const walk = (directory: string): void => {
    for (const entry of readdirSync(join(root, directory), { withFileTypes: true })) {
      const path = `${directory}/${entry.name}`;
      if (entry.isDirectory()) walk(path);
      else files.push(path);
    }
  };
  walk("src");
  return files.sort();
}

// A hash of those files, written into the bundle's first lines. A test recomputes it, so a bundle
// that is older than its sources is caught. Line endings are normalized so a Windows checkout
// gives the same value.
export function sourceHash(root: string): string {
  const hash = createHash("sha256");
  for (const file of sourceFiles(root)) {
    const text = readFileSync(join(root, file), "utf8").replace(/\r\n/g, "\n");
    hash.update(`${file}\0${text}\0`);
  }
  return hash.digest("hex");
}
