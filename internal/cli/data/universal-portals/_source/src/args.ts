import { parseArgs } from "node:util";

import { PluginError } from "./plugin-error.js";
import type { TemplateId } from "./templates/types.js";

export class UsageError extends PluginError {
  constructor(message: string) {
    super(message, "usage");
  }
}

export interface ParsedSetupArgs {
  command: "setup" | "audit" | "help";
  portalId?: string;
  strict: boolean;
  fix?: boolean;
  // Set by the CLI's --tenant flag, which it forwards to the plugin.
  tenant?: string;
  noColor: boolean;
  template?: TemplateId;
  name?: string;
  slug?: string;
  user?: string;
  organization?: string;
  json: boolean;
  noInput: boolean;
  // Setup only: show what a run would create and change, without writing to the tenant.
  dryRun?: boolean;
}

// Parses the arguments the plugin receives from the CLI. Interactive questions
// are asked by the caller for anything missing, so this only reports what the
// caller supplied. Global flags the CLI owns are accepted here; --tenant is
// returned so the caller can check it.
export function parseSetupArgs(argv: string[]): ParsedSetupArgs {
  const { globals, rest } = splitGlobalFlags(argv);
  const { values, positionals } = parseArgs({
    args: rest,
    allowPositionals: true,
    strict: true,
    options: {
      template: { type: "string" },
      name: { type: "string", short: "n" },
      slug: { type: "string", short: "s" },
      user: { type: "string", short: "u" },
      organization: { type: "string", short: "o" },
      json: { type: "boolean" },
      "no-input": { type: "boolean" },
      "dry-run": { type: "boolean" },
      help: { type: "boolean", short: "h" },
      strict: { type: "boolean" },
      fix: { type: "boolean" },
    },
  });

  const [first] = positionals;
  if (values.help || first === undefined) {
    return {
      command: "help",
      strict: false,
      tenant: globals.tenant,
      noColor: globals.noColor,
      json: Boolean(values.json),
      noInput: Boolean(values["no-input"]),
    };
  }
  if (first === "audit") {
    if (positionals.length !== 2) {
      throw new UsageError("Usage: auth0 universal-portals audit <portal-id> [--organization <id>] [--strict]");
    }
    return {
      command: "audit",
      portalId: positionals[1],
      tenant: globals.tenant,
      noColor: globals.noColor,
      organization: values.organization,
      strict: Boolean(values.strict),
      fix: Boolean(values.fix),
      json: Boolean(values.json),
      noInput: Boolean(values["no-input"]),
    };
  }
  if (first !== "setup") {
    throw new UsageError(`Unknown command "${first}". Run: auth0 universal-portals --help`);
  }
  if (positionals.length > 1) {
    throw new UsageError(`Unexpected arguments: ${positionals.slice(1).join(" ")}`);
  }

  const template = values.template;
  if (template !== undefined && template !== "consumer" && template !== "business") {
    throw new UsageError(`--template must be "consumer" or "business", not "${template}".`);
  }

  return {
    command: "setup",
    strict: false,
    tenant: globals.tenant,
    noColor: globals.noColor,
    template,
    name: values.name,
    slug: values.slug,
    user: values.user,
    organization: values.organization,
    json: Boolean(values.json),
    noInput: Boolean(values["no-input"]),
    dryRun: Boolean(values["dry-run"]),
  };
}

// Without a terminal, every answer has to come from a flag.
export function missingFlags(args: ParsedSetupArgs): string[] {
  const missing: string[] = [];
  if (!args.template) missing.push("--template");
  if (!args.name) missing.push("--name");
  if (args.template === "business" && !args.user) missing.push("--user");
  return missing;
}

export function usage(): string {
  return [
    "Usage: auth0 universal-portals setup [flags]",
    "",
    "Creates an Auth0 Portal with its application, forms and pages.",
    "",
    "Flags:",
    "  --template <consumer|business>  Portal template. Asked when omitted.",
    "  -n, --name <name>               Portal name. Asked when omitted.",
    "  -s, --slug <slug>               URL slug. Defaults to a slug of the name.",
    "  -u, --user <email or id>        Business only: the user who will test the portal.",
    "  -o, --organization <id|name>    Business only: the organization, when the user has several.",
    "      --no-input                  Never prompt; fail on anything missing.",
    "      --dry-run                   Show what setup would create, without changing the tenant.",
    "      --json                      Print the result as JSON on stdout.",
    "  -h, --help                      Show this help.",
    "",
    "Usage: auth0 universal-portals audit <portal-id> [flags]",
    "",
    "Checks a portal and its application against the setup defaults.",
    "",
    "Flags:",
    "  -o, --organization <id>         Business only: the organization the portal opens in.",
    "      --fix                       Apply the errors the CLI can fix, without asking.",
    "      --strict                    Also exit 1 when there are warnings.",
    "      --json                      Print the result as JSON on stdout.",
    "  -h, --help                      Show this help.",
  ].join("\n");
}

function splitGlobalFlags(argv: string[]): { globals: { tenant?: string; noColor: boolean }; rest: string[] } {
  const globals: { tenant?: string; noColor: boolean } = { noColor: false };
  const rest: string[] = [];
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    if (arg === "--no-color") {
      globals.noColor = true;
      continue;
    }
    if (arg === "--debug") continue;
    if (arg === "--tenant") {
      globals.tenant = argv[index + 1];
      index += 1;
      continue;
    }
    if (arg.startsWith("--tenant=")) {
      globals.tenant = arg.slice("--tenant=".length);
      continue;
    }
    rest.push(arg);
  }
  return { globals, rest };
}
