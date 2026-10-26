import { strict as assert } from "node:assert";
import { test } from "node:test";

import { colorEnabled, Ui } from "../src/ui.js";
import { captureOutput } from "./capture-output.js";

function capture(color: boolean, silent = false, animate = false): { ui: Ui; out: () => string } {
  const captured = captureOutput();
  return { ui: new Ui(captured.stream, color, silent, animate), out: () => captured.text() };
}

test("success, warn, error and info write their messages", () => {
  const { ui, out } = capture(false);
  ui.success("Resource servers ready");
  ui.warn("Slug is taken");
  ui.error("Missing scope");
  ui.info("Run again to resume");
  const text = out();
  assert.match(text, /Resource servers ready/);
  assert.match(text, /Slug is taken/);
  assert.match(text, /Missing scope/);
  assert.match(text, /Run again to resume/);
});

test("the outcome note of a success line is dimmed when colors are on", () => {
  const plain = capture(false);
  plain.ui.success("Auth0 My Account API", "already exists");
  assert.match(plain.out(), /Auth0 My Account API already exists/);
  assert.doesNotMatch(plain.out(), /\x1b\[/, "no escape sequences without colors");

  const colored = capture(true);
  colored.ui.success("Auth0 My Account API", "already exists");
  assert.match(colored.out(), /\x1b\[2malready exists\x1b\[0m/);
});

test("the title box shows the command and the tenant before any question", () => {
  const { ui, out } = capture(false);
  ui.banner("Universal Portals setup", "acme.us.auth0.com");
  assert.match(out(), /Universal Portals setup/);
  assert.match(out(), /acme\.us\.auth0\.com/);
});

test("silent mode writes nothing", () => {
  const { ui, out } = capture(false, true);
  ui.banner("Universal Portals setup", "acme.us.auth0.com");
  ui.section("Resource servers");
  ui.success("Auth0 My Account API");
  ui.warn("Slug is taken");
  ui.ready("https://acme.us.auth0.com/portals/acme");
  assert.equal(out(), "");
});

test("NO_COLOR and --no-color disable colors even on a TTY", () => {
  assert.equal(colorEnabled({ NO_COLOR: "1" }, true), false);
  assert.equal(colorEnabled({}, true, true), false);
  assert.equal(colorEnabled({}, true), true);
  assert.equal(colorEnabled({}, false), false);
});

test("a task returns its result and, without a terminal, prints no spinner", async () => {
  const { ui, out } = capture(false, false, false);
  const result = await ui.task("Creating application", async () => "created");
  assert.equal(result, "created");
  assert.equal(out(), "");
});

test("a task with a terminal shows its label while the work runs", async () => {
  const { ui, out } = capture(false, false, true);
  const result = await ui.task("Creating application", async () => {
    await new Promise((resolve) => setTimeout(resolve, 200));
    return 42;
  });
  assert.equal(result, 42);
  assert.match(out(), /Creating application/);
});

test("a failing task rethrows its error", async () => {
  const { ui } = capture(false, false, true);
  await assert.rejects(
    ui.task("Creating portal", async () => {
      throw new Error("boom");
    }),
    /boom/,
  );
});

test("a reference links the ID to its dashboard page when colors are on", () => {
  const colored = capture(true);
  colored.ui.reference("Application", "abc123", "https://manage.auth0.com/dashboard/us/acme/applications/abc123/settings");
  assert.match(colored.out(), /\x1b\]8;;https:\/\/manage\.auth0\.com\/dashboard\/us\/acme\/applications\/abc123\/settings\x1b\\/);
});

test("without colors a reference shows the plain ID and no escape sequences", () => {
  const plain = capture(false);
  plain.ui.reference("Application", "abc123", "https://manage.auth0.com/x");
  assert.match(plain.out(), /Application: abc123/);
  assert.doesNotMatch(plain.out(), /\x1b\]8;;/);
});

test("a section, its items and its result print in order, each after a blank line", () => {
  const { ui, out } = capture(false);
  ui.section("Application");
  ui.success("\"Acme Account (Universal Portals)\" created");
  ui.result("Application created");
  const text = out();
  const header = text.indexOf("Application\n");
  const item = text.indexOf("Acme Account");
  const result = text.indexOf("Application created");
  assert.ok(header >= 0 && header < item && item < result, "header, item, then result");
  assert.match(text, /\n\u2502\n/, "a blank line separates the lines");
});

test("the closing block shows the portal URL in a note", () => {
  const { ui, out } = capture(false);
  ui.ready("https://acme.us.auth0.com/portals/acme");
  assert.match(out(), /Open portal: https:\/\/acme\.us\.auth0\.com\/portals\/acme/);
});

test("a reference in a colored note links the portal URL", () => {
  const { ui, out } = capture(true);
  ui.ready("https://acme.us.auth0.com/portals/acme");
  assert.match(out(), /\x1b\]8;;https:\/\/acme\.us\.auth0\.com\/portals\/acme\x1b\\/);
});
