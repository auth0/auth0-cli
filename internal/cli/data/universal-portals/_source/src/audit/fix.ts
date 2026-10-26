import type { ApiClient } from "../api.js";
import type { Ui } from "../ui.js";
import { auditPortal, Finding } from "./checks.js";
import type { PortalSnapshot } from "./snapshot.js";

export interface ApplyDeps {
  api: ApiClient;
  ui: Ui;
  // Reads the portal again. Each fix reads it first, so a fix never works on a
  // stale copy and never overwrites a change made by an earlier fix.
  readSnapshot: () => Promise<PortalSnapshot>;
}

export interface ApplyResult {
  applied: number;
  failed: number;
  // Fixes that were no longer needed by the time they ran.
  alreadyFixed: number;
}

// Only errors are fixed. Warnings may be intentional, so they stay in the
// report and are never changed by the CLI.
export function fixableFindings(findings: Finding[]): Finding[] {
  return findings.filter((finding) => finding.severity === "error" && finding.remedy?.apply);
}

// The ID and subject identify a finding. The message can change when the portal
// changes (a count, a slug), so it never takes part in matching.
export function findingKey(finding: Finding): string {
  return `${finding.id}:${finding.subject ?? ""}`;
}

export async function applyFixes(selected: Finding[], deps: ApplyDeps): Promise<ApplyResult> {
  const { ui } = deps;
  const result: ApplyResult = { applied: 0, failed: 0, alreadyFixed: 0 };

  for (const finding of selected) {
    const fresh = auditPortal(await deps.readSnapshot()).find((candidate) => findingKey(candidate) === findingKey(finding));
    if (!fresh?.remedy?.apply) {
      result.alreadyFixed += 1;
      ui.success(`${finding.id} ${finding.check}`, "already fixed");
      continue;
    }
    // Each fix shows the finding it resolves, with a spinner while it runs.
    const label = `${fresh.id} ${fresh.check}`;
    try {
      await ui.task(`Fixing ${fresh.check}`, () => fresh.remedy!.apply!(deps.api));
      result.applied += 1;
      ui.fixed(label, fresh.remedy.summary);
    } catch (error) {
      result.failed += 1;
      ui.error(`${label}: ${error instanceof Error ? error.message : String(error)}`);
    }
  }
  return result;
}

// A fix can expose errors the first audit could not see: a new application has no
// client grant, so linking it makes P019 visible. After each pass, chooseNext picks
// which of the newly visible errors to fix next (none ends the run). A finding is
// attempted once, and passes stop after MAX_FIX_ROUNDS so a fix that keeps breaking
// another cannot loop.
const MAX_FIX_ROUNDS = 3;

export async function applyFixRounds(
  selected: Finding[],
  deps: ApplyDeps,
  chooseNext: (fixable: Finding[]) => Promise<Finding[]>,
): Promise<ApplyResult> {
  const total: ApplyResult = { applied: 0, failed: 0, alreadyFixed: 0 };
  const attempted = new Set<string>();
  let pending = selected;

  for (let round = 0; round < MAX_FIX_ROUNDS && pending.length > 0; round += 1) {
    for (const finding of pending) attempted.add(findingKey(finding));
    const result = await applyFixes(pending, deps);
    total.applied += result.applied;
    total.failed += result.failed;
    total.alreadyFixed += result.alreadyFixed;

    const exposed = fixableFindings(auditPortal(await deps.readSnapshot())).filter((finding) => !attempted.has(findingKey(finding)));
    pending = exposed.length > 0 ? await chooseNext(exposed) : [];
  }
  return total;
}
