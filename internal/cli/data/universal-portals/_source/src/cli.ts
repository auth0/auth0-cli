#!/usr/bin/env node
import { realpathSync } from "node:fs";
import { pathToFileURL } from "node:url";

import { parseSetupArgs, ParsedSetupArgs, UsageError, usage } from "./args.js";
import type { FetchLike } from "./api.js";
import { clackPrompter } from "./clack-prompter.js";
import type { Prompter } from "./prompt.js";
import { runAuditCommand } from "./audit-command.js";
import { describeError } from "./errors.js";
import { runSetupCommand } from "./setup-command.js";
import { colorEnabled, Ui } from "./ui.js";

export interface IoStreams {
  stdout: { write(text: string): void };
  stderr: NodeJS.WriteStream;
  stdinStream: NodeJS.ReadStream;
  isTTY: boolean;
  fetchImpl?: FetchLike;
  // Answers the questions. Defaults to the terminal when the run is interactive.
  prompter?: Prompter;
  // Opens the portal after setup. Defaults to the system browser; previews replace it.
  openBrowser?: (url: string) => void;
}

export async function main(argv: string[], env: NodeJS.ProcessEnv, io: IoStreams): Promise<number> {
  let args: ParsedSetupArgs;
  try {
    args = parseSetupArgs(argv);
  } catch (error) {
    io.stderr.write(`${describeError(error).message}\n`);
    return error instanceof UsageError ? 2 : 1;
  }

  if (args.command === "help") {
    io.stdout.write(`${usage()}\n`);
    return 0;
  }

  const agentMode = args.json || env.AUTH0_AGENT_MODE === "1" || env.AUTH0_AGENT_MODE === "true";
  const interactive = !args.noInput && !agentMode && io.isTTY;
  const ui = new Ui(
    io.stderr,
    colorEnabled(env, io.isTTY, args.noColor) && !agentMode,
    agentMode,
    io.isTTY && !agentMode,
  );

  // Undefined when the run cannot ask: the commands then fall back to flags and plain output.
  const prompter = interactive ? (io.prompter ?? clackPrompter(io.stdinStream, io.stderr)) : undefined;
  if (args.command === "audit") return runAuditCommand(args, env, io, ui, prompter);
  return runSetupCommand(args, env, io, ui, interactive, prompter);
}

// npx starts the plugin through a symlink in node_modules/.bin, so compare the
// resolved path with this module's URL, not the path as typed.
const entryPoint = process.argv[1] ? realpathSync(process.argv[1]) : undefined;
if (entryPoint && import.meta.url === pathToFileURL(entryPoint).href) {
  main(process.argv.slice(2), process.env, {
    stdout: process.stdout,
    stderr: process.stderr,
    stdinStream: process.stdin,
    isTTY: Boolean(process.stdin.isTTY && process.stderr.isTTY),
  }).then((code) => {
    process.exitCode = code;
  });
}
