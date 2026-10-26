import { ApiClient, ApiError, HttpMethod } from "../api.js";
import { PluginError } from "../plugin-error.js";

// One request per capability setup needs. A 2xx or 400 means the scope is
// present. A 403 names the scope it wants, so every gap is reported at once.
// The create probes send an empty body. Checked on the live API on 2026-10-23:
// each one returned 400 for the missing required property and created nothing.
export interface Probe {
  name: string;
  method: HttpMethod;
  path: string;
  body?: unknown;
  // Only used by the business template.
  businessOnly?: boolean;
}

export const SETUP_PROBES: Probe[] = [
  { name: "list resource servers", method: "GET", path: "resource-servers?per_page=1" },
  { name: "create resource server", method: "POST", path: "resource-servers", body: {} },
  { name: "list clients", method: "GET", path: "clients?per_page=1" },
  { name: "create client", method: "POST", path: "clients", body: {} },
  { name: "list client grants", method: "GET", path: "client-grants?per_page=1" },
  { name: "create client grant", method: "POST", path: "client-grants", body: {} },
  { name: "list vault connections", method: "GET", path: "flows/vault/connections" },
  { name: "create vault connection", method: "POST", path: "flows/vault/connections", body: {} },
  { name: "list forms", method: "GET", path: "forms?per_page=1" },
  { name: "import forms", method: "POST", path: "forms/import", body: {} },
  { name: "list portals", method: "GET", path: "portals" },
  { name: "create portal", method: "POST", path: "portals", body: {} },
  {
    name: "look up users by email",
    method: "GET",
    path: "users?q=email%3A%22preflight%40example.invalid%22&search_engine=v3",
    businessOnly: true,
  },
  { name: "list organizations", method: "GET", path: "organizations?per_page=1", businessOnly: true },
];

export interface ScopeGap {
  probe: string;
  requirement: "any" | "all";
  scopes: string[];
}

export interface PreflightResult {
  gaps: ScopeGap[];
  // Probes that failed for a reason other than a missing scope.
  unexpected: { probe: string; status: number; message: string }[];
}

export class FeatureNotEnabledError extends PluginError {
  constructor(readonly tenantDomain: string) {
    super(
      `Universal Portals is not enabled for ${tenantDomain}. Ask Auth0 to enable it for this tenant, then run the command again.`,
      "feature_not_enabled",
    );
  }
}

export async function runPreflight(
  api: ApiClient,
  probes: Probe[],
  tenantDomain: string,
): Promise<PreflightResult> {
  const gaps: ScopeGap[] = [];
  const unexpected: PreflightResult["unexpected"] = [];

  for (const probe of probes) {
    try {
      await api.request(probe.method, probe.path, probe.body);
    } catch (error) {
      if (!(error instanceof ApiError)) throw error;
      if (error.status === 400) continue;
      if (error.errorCode === "feature_not_enabled") throw new FeatureNotEnabledError(tenantDomain);
      const gap = parseScopeGap(probe.name, error);
      if (gap) {
        gaps.push(gap);
      } else {
        unexpected.push({ probe: probe.name, status: error.status, message: error.message });
      }
    }
  }

  return { gaps, unexpected };
}

// Management API message: "Insufficient scope, expected any of: a,b" or
// "Insufficient scope, expected all of: a,b".
export function parseScopeGap(probe: string, error: ApiError): ScopeGap | undefined {
  if (error.status !== 403) return undefined;
  const match = /expected (any|all) of:\s*(.+)$/.exec(error.message);
  if (!match) return undefined;
  const requirement = match[1] === "all" ? "all" : "any";
  const scopes = match[2].split(",").map((scope) => scope.trim()).filter(Boolean);
  return { probe, requirement, scopes };
}

// Scopes the session must have. An "any of" gap with several options is not
// listed: any one of them would do, so the caller reports it as a choice.
export function requiredMissingScopes(result: PreflightResult): string[] {
  const required = result.gaps.filter((gap) => gap.requirement === "all" || gap.scopes.length === 1);
  return [...new Set(required.flatMap((gap) => gap.scopes))].sort();
}

export function choiceGaps(result: PreflightResult): ScopeGap[] {
  return result.gaps.filter((gap) => gap.requirement === "any" && gap.scopes.length > 1);
}
