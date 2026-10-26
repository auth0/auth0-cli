import { strict as assert } from "node:assert";
import { test } from "node:test";
import { PassThrough, Writable } from "node:stream";

import { clackPrompter } from "../src/clack-prompter.js";
import { CancelledError } from "../src/prompt.js";

// Keystrokes go in through a stream, and the prompt's output is captured. Keys are
// sent only after the question has been drawn, so the answers do not race the render.
function terminal() {
  const input = new PassThrough();
  let drawn = "";
  const output = new Writable({
    write(chunk, _encoding, done) {
      drawn += chunk;
      done();
    },
  });
  Object.assign(output, { isTTY: false, columns: 80 });
  return {
    input: input as unknown as NodeJS.ReadStream,
    output: output as unknown as NodeJS.WriteStream,
    async answer<T>(question: string, keys: string[], run: () => Promise<T>): Promise<T> {
      const result = run();
      await waitUntilDrawn(() => drawn.includes(question));
      for (const key of keys) input.write(key);
      return result;
    },
  };
}

async function waitUntilDrawn(drawn: () => boolean): Promise<void> {
  while (!drawn()) await new Promise((resolve) => setImmediate(resolve));
}

const ENTER = "\r";
const DOWN = "\x1b[B";
const CTRL_C = "\x03";

test("a typed answer is returned trimmed", async () => {
  const term = terminal();
  const prompter = clackPrompter(term.input, term.output);

  const answer = await term.answer("Slug", ["  acme-account  ", ENTER], () => prompter.ask("Slug"));

  assert.equal(answer, "acme-account");
});

test("an empty answer returns the default", async () => {
  const term = terminal();
  const prompter = clackPrompter(term.input, term.output);

  const answer = await term.answer("Slug", [ENTER], () => prompter.ask("Slug", "acme-account-2"));

  assert.equal(answer, "acme-account-2");
});

test("an empty answer with no default is an empty string, which skips an optional question", async () => {
  const term = terminal();
  const prompter = clackPrompter(term.input, term.output);

  const answer = await term.answer("Test user", [ENTER], () => prompter.ask("Test user"));

  assert.equal(answer, "");
});

test("confirm returns the default on Enter, and the other answer when pressed", async () => {
  const term = terminal();
  const prompter = clackPrompter(term.input, term.output);

  const defaulted = await term.answer("Assign a role?", [ENTER], () => prompter.confirm("Assign a role?", true));
  assert.equal(defaulted, true);

  const term2 = terminal();
  const prompter2 = clackPrompter(term2.input, term2.output);
  const declined = await term2.answer("Assign a role?", ["n", ENTER], () => prompter2.confirm("Assign a role?", true));
  assert.equal(declined, false);
});

test("choose returns the value of the option the user picks", async () => {
  const term = terminal();
  const prompter = clackPrompter(term.input, term.output);
  const options = [
    { label: "Consumer", hint: "end users", value: "consumer" },
    { label: "Business", hint: "organization admins", value: "business" },
  ];

  const picked = await term.answer("Template", [DOWN, ENTER], () => prompter.choose("Template", options));

  assert.equal(picked, "business");
});

test("Ctrl+C at a question rejects with CancelledError, for every question type", async () => {
  const term = terminal();
  const prompter = clackPrompter(term.input, term.output);

  await assert.rejects(term.answer("Slug", [CTRL_C], () => prompter.ask("Slug")), CancelledError);
  await assert.rejects(term.answer("Open?", [CTRL_C], () => prompter.confirm("Open?", true)), CancelledError);
});
