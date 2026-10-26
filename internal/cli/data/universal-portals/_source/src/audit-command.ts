import type { ParsedSetupArgs } from "./args.js";
import { CancelledError, type Prompter } from "./prompt.js";
import { apiFor, checkTenantFlag, readContext } from "./context.js";
import { applyFixRounds, fixableFindings } from "./audit/fix.js";
import type { Finding } from "./audit/checks.js";
import { AuditOptions, AuditResult, runAudit } from "./audit/run.js";
import { fetchPortalSnapshot } from "./audit/snapshot.js";
import type { ApiClient } from "./api.js";
import type { IoStreams } from "./cli.js";
import { reportError } from "./errors.js";
import type { Ui } from "./ui.js";

export async function runAuditCommand(
  args: ParsedSetupArgs,
  env: NodeJS.ProcessEnv,
  io: IoStreams,
  ui: Ui,
  prompter: Prompter | undefined,
): Promise<number> {
  try {
    const context = await readContext(env, args.tenant, io.run);
    checkTenantFlag(args.tenant, context);
    const api = apiFor(context, { fetchImpl: io.fetchImpl, run: io.run });
    const portalId = args.portalId ?? "";
    const organizationId = args.organization;
    const readSnapshot = () => fetchPortalSnapshot(api, portalId, context.tenantDomain, { organizationId });
    const options = { portalId, organizationId, strict: args.strict };

    const report = await runAudit(options, { api, ui, tenantDomain: context.tenantDomain });
    const fixable = fixableFindings(report.findings);
    const applyDeps = { api, ui, readSnapshot };
    let result = report;

    if (args.fix) {
      // Scripts: apply the errors the CLI can fix, without asking.
      const errors = fixable.filter((finding) => finding.severity === "error");
      if (errors.length > 0) {
        ui.section("Fix");
        await applyFixRounds(errors, applyDeps, async (exposed) => exposed);
        result = await finalAudit(options, ui, api, context.tenantDomain);
      }
    } else if (prompter && fixable.length > 0) {
      const selected = await chooseFixes(prompter, fixable, ui);
      if (selected.length > 0) {
        ui.section("Fix");
        await applyFixRounds(selected, applyDeps, async (exposed) => {
          ui.detail("These errors appeared after the fixes above.");
          return chooseFixes(prompter, exposed, ui);
        });
        result = await finalAudit(options, ui, api, context.tenantDomain);
      }
    } else if (fixable.length > 0) {
      ui.detail("Run with --fix to apply the errors the CLI can fix.");
    }

    if (args.json) io.stdout.write(`${JSON.stringify(result, null, 2)}\n`);
    return result.failed ? 1 : 0;
  } catch (error) {
    if (error instanceof CancelledError && !args.json) {
      ui.warn("Cancelled. Fixes already applied stay in your tenant.");
      ui.cancelled("Audit cancelled.");
      return 130;
    }
    return reportError(error, args.json, io, ui);
  }
}

// Asks what to do with the findings: fix every error, choose the fixes, or
// skip. Errors start checked in the checklist, and recommendations do not.
async function chooseFixes(prompter: Prompter, errors: Finding[], ui: Ui): Promise<Finding[]> {
  ui.detail("Warnings are not changed. They may be intentional.");
  const choice = await prompter.choose<"all" | "choose" | "skip">("What do you want to fix?", [
    { label: "Fix all errors", hint: "recommended", value: "all" },
    { label: "Choose errors to fix", hint: "every error starts checked", value: "choose" },
    { label: "Skip", hint: "leave the portal as it is", value: "skip" },
  ]);
  if (choice === "skip") return [];
  if (choice === "all") return errors;
  return prompter.checklist(
    "Errors to fix",
    errors.map((finding) => ({
      label: `${finding.id} ${finding.remedy?.summary ?? finding.message}`,
      value: finding,
      checked: true,
    })),
  );
}

async function finalAudit(
  options: AuditOptions,
  ui: Ui,
  api: ApiClient,
  tenantDomain: string,
): Promise<AuditResult> {
  ui.section("Checking the portal again");
  return runAudit({ ...options, showBanner: false }, { api, ui, tenantDomain });
}
