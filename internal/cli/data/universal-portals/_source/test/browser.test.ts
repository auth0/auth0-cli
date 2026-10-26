import { strict as assert } from "node:assert";
import { test } from "node:test";

import { browserCommand } from "../src/browser.js";

const URL_ = "https://auth.acme.com/portals/acme";

test("macOS opens the URL with open", () => {
  assert.deepEqual(browserCommand(URL_, "darwin"), { command: "open", args: [URL_] });
});

test("Windows opens the URL with start, passing an empty title first", () => {
  assert.deepEqual(browserCommand(URL_, "win32"), { command: "cmd", args: ["/c", "start", "", URL_] });
});

test("Linux opens the URL with xdg-open", () => {
  assert.deepEqual(browserCommand(URL_, "linux"), { command: "xdg-open", args: [URL_] });
});
