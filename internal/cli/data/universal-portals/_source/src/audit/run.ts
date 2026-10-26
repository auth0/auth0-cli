import { ApiClient } from "../api.js";
import { auditPortal, Finding } from "./checks.js";
import { fetchPortalSnapshot } from "./snapshot.js";
import type { Ui } from "../ui.js";

export interface AuditOptions {
  portalId: string;
  organizationId?: string;
  strict: boolean;
  // The banner is for the first audit only; the check after the fixes continues it.
  showBanner?: boolean;
}

export interface AuditResult {
  portalId: string;
  findings: Finding[];
  errors: number;
  warnings: number;
  failed: boolean;
}

export async function runAudit(options: AuditOptions, deps: { api: ApiClient; ui: Ui; tenantDomain: string }): Promise<AuditResult> {
  const { api, ui, tenantDomain } = deps;
  if (options.showBanner !== false) ui.banner("Universal Portals audit", tenantDomain);

  const snapshot = await ui.task("Reading the portal", () =>
    fetchPortalSnapshot(api, options.portalId, tenantDomain, { organizationId: options.organizationId }),
  );
  const findings = auditPortal(snapshot);
  const errors = findings.filter((finding) => finding.severity === "error").length;
  const warnings = findings.length - errors;

  ui.section(`Portal "${snapshot.portal.slug}"`);
  if (findings.length === 0) {
    ui.success("No problems found");
  } else {
    ui.table(
      ["", "ID", "Check", "Fix"],
      findings.map((finding) => [
        finding.severity === "error" ? "✖" : "▲",
        finding.id,
        finding.check,
        finding.remedy?.summary ?? finding.message,
      ]),
    );
    for (const finding of findings.filter((item) => item.remedy?.command && !item.remedy.apply)) {
      ui.detail(`${finding.id} manually: ${finding.remedy?.command}`);
    }
  }

  ui.newline();
  const summary = `${errors} error${errors === 1 ? "" : "s"}, ${warnings} warning${warnings === 1 ? "" : "s"}`;
  if (errors > 0) ui.error(summary);
  else if (warnings > 0) ui.warn(summary);
  else ui.success(summary);

  return {
    portalId: options.portalId,
    findings,
    errors,
    warnings,
    failed: errors > 0 || (options.strict && warnings > 0),
  };
}
