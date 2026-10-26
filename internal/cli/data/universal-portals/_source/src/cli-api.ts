// Management API client that calls the Auth0 CLI's `auth0 api` command. The CLI owns the
// login and the token, so this module never holds a credential. Request bodies go to the
// command on stdin, because the bodies carry client secrets and arguments are visible to
// every process on the machine.

import { spawn } from "node:child_process";

import { ApiClient, HttpMethod, toApiError } from "./api.js";
import { PluginError } from "./plugin-error.js";

export interface CommandResult {
  exitCode: number;
  stdout: string;
  stderr: string;
}

export type RunCommand = (bin: string, args: string[], input: string | undefined) => Promise<CommandResult>;

export interface CliApiOptions {
  // Path of the auth0 executable.
  bin: string;
  // Domain of the tenant to call. The CLI's active tenant is used when it is not set.
  tenant?: string;
  // Starts the process. Replaced in tests.
  run?: RunCommand;
  // Requests allowed at once after the first one has finished.
  concurrency?: number;
}

const DEFAULT_CONCURRENCY = 4;
const INSTALL_URL = "https://github.com/auth0/auth0-cli#installation";

// What the CLI prints when it cannot use the session. Each needs the user to log in again.
const SIGN_IN_PROBLEM =
  /auth token expired|login session has expired|token is invalid|corrupted authentication token|failed to find tenant|config\.json file is missing/i;

export function createCliApiClient(options: CliApiOptions): ApiClient {
  const { bin, tenant, run = runCommand } = options;
  const gate = createGate(options.concurrency ?? DEFAULT_CONCURRENCY);

  return {
    async request<T>(method: HttpMethod, path: string, body?: unknown): Promise<T> {
      await gate.enter();
      try {
        const args = ["api", method, path.replace(/^\//, ""), "--no-color", "--no-input"];
        if (tenant) args.push("--tenant", tenant);
        if (method === "DELETE") args.push("--force");

        const result = await runOrExplain(run, bin, args, body === undefined ? undefined : JSON.stringify(body));
        if (result.exitCode !== 0) throw explainFailure(result.stderr, method, path, tenant);
        return parseOutput<T>(result.stdout, method, path);
      } finally {
        gate.leave();
      }
    },
  };
}

// The first request runs alone. If the CLI has to refresh an expired token it saves the
// session file, and several processes doing that at once could damage it. Once one request
// has finished the token is fresh, so the rest can run side by side.
function createGate(concurrency: number): { enter(): Promise<void>; leave(): void } {
  let limit = 1;
  let active = 0;
  let firstFinished = false;
  const waiting: Array<() => void> = [];

  const admit = (): void => {
    while (active < limit && waiting.length > 0) {
      active += 1;
      waiting.shift()?.();
    }
  };

  return {
    enter: () =>
      new Promise<void>((resolve) => {
        waiting.push(resolve);
        admit();
      }),
    leave: () => {
      active -= 1;
      if (!firstFinished) {
        firstFinished = true;
        limit = concurrency;
      }
      admit();
    },
  };
}

async function runOrExplain(
  run: RunCommand,
  bin: string,
  args: string[],
  input: string | undefined,
): Promise<CommandResult> {
  try {
    return await run(bin, args, input);
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "ENOENT") {
      throw new PluginError(
        `The Auth0 CLI was not found (tried "${bin}"). Install it from ${INSTALL_URL}, or set AUTH0_CLI_BIN to its path.`,
        "auth0_cli_not_found",
      );
    }
    throw error;
  }
}

function parseOutput<T>(stdout: string, method: HttpMethod, path: string): T {
  const text = stdout.trim();
  if (text === "") return undefined as T;
  try {
    return JSON.parse(text) as T;
  } catch {
    throw new Error(`auth0 api ${method} ${path} printed output that is not JSON: ${text.slice(0, 120)}`);
  }
}

function explainFailure(stderr: string, method: HttpMethod, path: string, tenant: string | undefined): Error {
  // The CLI keeps only the first missing scope and drops the API's body, so the message is rebuilt
  // in the shape the permission check already reads.
  const scope = /lacks scope: ([A-Za-z0-9_:,-]+)/i.exec(stderr)?.[1];
  if (scope) {
    return toApiError(403, {
      statusCode: 403,
      error: "Forbidden",
      errorCode: "insufficient_scope",
      message: `Insufficient scope, expected any of: ${scope}`,
    });
  }

  const api = /(\d{3}): API request failed: (.*)$/m.exec(stderr);
  if (api) return toApiError(Number(api[1]), parseBody(api[2]));

  if (SIGN_IN_PROBLEM.test(stderr)) {
    return new PluginError(
      `The Auth0 CLI cannot use ${tenant ?? "its active tenant"}: ${firstMessage(stderr)} Run auth0 login, then run this command again.`,
      "auth0_login_required",
      ["auth0 login"],
    );
  }

  return new Error(`auth0 api ${method} ${path} failed: ${firstMessage(stderr)}`);
}

// The CLI ends the message with a period after the API's JSON.
function parseBody(text: string): unknown {
  for (const candidate of [text, text.replace(/\.$/, "")]) {
    try {
      return JSON.parse(candidate);
    } catch {
      // Try the next form.
    }
  }
  return text;
}

// The first line of the CLI's message, without its heading and marker.
function firstMessage(stderr: string): string {
  const line = stderr
    .split("\n")
    .map((entry) => entry.trim())
    .find((entry) => entry !== "" && !entry.startsWith("==="));
  return (line ?? "no message").replace(/^[^A-Za-z0-9]+/, "");
}

export function runCommand(bin: string, args: string[], input: string | undefined): Promise<CommandResult> {
  return new Promise((resolve, reject) => {
    const child = spawn(bin, args, { stdio: ["pipe", "pipe", "pipe"] });
    let stdout = "";
    let stderr = "";
    child.stdout.setEncoding("utf8").on("data", (chunk) => (stdout += chunk));
    child.stderr.setEncoding("utf8").on("data", (chunk) => (stderr += chunk));
    child.on("error", reject);
    child.on("close", (code) => resolve({ exitCode: code ?? 1, stdout, stderr }));
    // The process may exit before it reads stdin, which is reported through its exit code.
    child.stdin.on("error", () => undefined);
    child.stdin.end(input ?? "");
  });
}
