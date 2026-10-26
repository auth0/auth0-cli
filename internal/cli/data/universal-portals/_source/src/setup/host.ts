import type { ApiClient } from "../api.js";
import { readCustomDomain } from "../audit/snapshot.js";
import { ORGANIZATION_PLACEHOLDER } from "./business.js";

export async function resolvePortalHost(
  api: ApiClient,
  tenantDomain: string,
): Promise<{ domain: string; isCustom: boolean }> {
  const custom = await readCustomDomain(api);
  if (custom?.status === "ready" && custom.domain) return { domain: custom.domain, isCustom: true };
  return { domain: tenantDomain, isCustom: false };
}

export function portalUrl(domain: string, slug: string, organizationId?: string): string {
  const base = `https://${domain}/portals/${slug}`;
  if (!organizationId) return base;
  // A placeholder stays readable so it is easy to find and replace.
  const value = organizationId === ORGANIZATION_PLACEHOLDER ? organizationId : encodeURIComponent(organizationId);
  return `${base}?organization=${value}`;
}
