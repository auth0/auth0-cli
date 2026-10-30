import { missingFlags, ParsedSetupArgs, UsageError } from "./args.js";
import { apiFor, checkTenantFlag, readContext } from "./context.js";
import { openBrowser } from "./browser.js";
import { CancelledError, type Prompter } from "./prompt.js";
import type { TemplateId } from "./templates/types.js";
import type { IoStreams } from "./cli.js";
import { type PlannedRequest, planWrites } from "./dry-run.js";
import { reportError } from "./errors.js";
import { runSetup, SetupOptions, SetupResult, SlugRaceError } from "./setup/run.js";
import { toPortalSlug } from "./slug.js";
import { trackWrites } from "./write-tracker.js";
import type { Ui } from "./ui.js";

export async function runSetupCommand(
  args: ParsedSetupArgs,
  env: NodeJS.ProcessEnv,
  io: IoStreams,
  ui: Ui,
  interactive: boolean,
  prompter: Prompter | undefined,
): Promise<number> {
  try {
    const context = await readContext(env, args.tenant, io.run);
    checkTenantFlag(args.tenant, context, " Nothing was created.");
    if (!interactive) {
      const missing = missingFlags(args);
      if (missing.length > 0) {
        throw new UsageError(`Missing required flags for a non-interactive run: ${missing.join(", ")}`);
      }
    }
    const tracked = trackWrites(apiFor(context, { fetchImpl: io.fetchImpl, run: io.run }));
    // A dry run reads the tenant as a real run does, but keeps the requests that would change it.
    const planned = args.dryRun ? planWrites(tracked.api) : undefined;

    ui.banner("Universal Portals setup", context.tenantDomain);
    if (planned) ui.info(DRY_RUN_NOTE);
    try {
      const options = prompter
        ? await askForMissing(args, prompter, context.tenantDomain)
        : optionsFromArgs(args);
      const result = await runSetup(options, {
        api: planned?.api ?? tracked.api,
        ui,
        prompt: prompter ?? unansweredPrompter(),
        interactive,
        tenantDomain: context.tenantDomain,
        dryRun: planned !== undefined,
      });
      if (planned) return reportPlan(result, planned.requests(), args.json, io, ui);
      if (args.json) {
        io.stdout.write(`${JSON.stringify(result, null, 2)}\n`);
        return 0;
      }
      ui.ready(result.portal.url);
      if (prompter && (await declinesToOpen(prompter))) return 0;
      if (prompter) (io.openBrowser ?? openBrowser)(result.portal.url);
      return 0;
    } catch (error) {
      if (error instanceof CancelledError && !args.json) return reportCancelled(ui, tracked.wrote());
      // Resources created before the failure stay, and the rerun reuses them. A slug race
      // is not resumed by a rerun with the same name, so it keeps its own message.
      if (!args.json && tracked.wrote() && !(error instanceof SlugRaceError)) {
        const code = reportError(error, false, io, ui);
        ui.detail(RESUME_HINT);
        return code;
      }
      throw error;
    }
  } catch (error) {
    return reportError(error, args.json, io, ui);
  }
}

// The statuses in a dry run are what a real run would report. Writes are not checked here, so
// the note says so: a real run is the only check of the permissions for creating things.
const DRY_RUN_NOTE =
  "Dry run: nothing is created or changed. Statuses show what a real run would do. Write permissions are checked by a real run only.";

// The result of a real run, plus the requests it would send in the order it would send them. The
// portal is not opened, because nothing was created.
function reportPlan(result: SetupResult, requests: PlannedRequest[], json: boolean, io: IoStreams, ui: Ui): number {
  if (json) {
    io.stdout.write(`${JSON.stringify({ dryRun: true, ...result, requests }, null, 2)}\n`);
    return 0;
  }
  ui.result(`Dry run complete. ${requests.length} request(s) would change the tenant.`);
  ui.detail("Run the same command without --dry-run to set up the portal.");
  return 0;
}

// Enter opens the portal. "n" or Ctrl+C means no: the portal is already set up,
// so cancelling this question is not a cancelled setup.
async function declinesToOpen(prompter: Prompter): Promise<boolean> {
  try {
    return !(await prompter.confirm("Open the portal in your browser?", true));
  } catch (error) {
    if (error instanceof CancelledError) return true;
    throw error;
  }
}

// Ctrl+C exits with 130. Business setup writes an organization before its later
// questions, so the message says what stays in the tenant when a write happened.
const RESUME_HINT = "Run the same command again to resume. Existing resources are reused.";

function reportCancelled(ui: Ui, wrote: boolean): number {
  if (wrote) {
    ui.warn("Cancelled. Nothing was rolled back: what was already created is still in your tenant.");
    ui.warn(RESUME_HINT);
  } else {
    ui.warn("Cancelled. Nothing was created.");
  }
  ui.cancelled("Setup cancelled.");
  return 130;
}

function optionsFromArgs(args: ParsedSetupArgs): SetupOptions {
  const name = args.name ?? "";
  return {
    template: args.template ?? "consumer",
    name,
    slug: args.slug ?? toPortalSlug(name),
    userQuery: args.user,
    organization: args.organization,
  };
}

export async function askForMissing(args: ParsedSetupArgs, prompt: Prompter, tenantDomain: string): Promise<SetupOptions> {
  const template =
    args.template ??
    (await prompt.choose<TemplateId>("Which kind of portal do you want to set up?", [
      {
        label: "Consumer portal",
        hint: "for end-users to manage their account in a self-service way",
        value: "consumer",
      },
      {
        label: "Business portal",
        hint: "for organization administrators to manage their organization in a self-service way",
        value: "business",
      },
    ]));
  const name = args.name ?? (await prompt.ask("Portal name", "My Portal"));
  const slug = args.slug ?? (await prompt.ask("URL slug", toPortalSlug(name)));
  // The test user is asked before anything is created, unless it was passed as a flag.
  return { template, name, slug, userQuery: args.user, organization: args.organization };
}

function unansweredPrompter(): Prompter {
  const refuse = (): never => {
    throw new UsageError("A question was needed but this run cannot prompt. Pass the flag instead.");
  };
  return {
    ask: async () => refuse(),
    confirm: async () => refuse(),
    choose: async () => refuse(),
    checklist: async () => refuse(),
  };
}
