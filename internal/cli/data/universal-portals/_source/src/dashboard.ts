// Management Dashboard links for the resources setup creates, so the output can
// link each ID to the page where it can be reviewed.

export interface TenantLocation {
  base: string;
  region: string;
  name: string;
}

// For example "acme.us.auth0.com" -> https://manage.auth0.com, region "us",
// name "acme". A tenant without a region code is treated as "us".
export function tenantLocation(tenantDomain: string): TenantLocation {
  const parts = tenantDomain.split(".");
  const hasRegion = parts.length >= 4 && parts[1].length === 2;
  const serviceDomain = hasRegion ? parts.slice(2) : parts.slice(1);
  return {
    base: `https://manage.${serviceDomain.join(".")}`,
    region: hasRegion ? parts[1] : "us",
    name: parts[0],
  };
}

function dashboardRoot(tenantDomain: string): string {
  const { base, region, name } = tenantLocation(tenantDomain);
  return `${base}/dashboard/${region}/${name}`;
}

export function applicationUrl(tenantDomain: string, clientId: string): string {
  return `${dashboardRoot(tenantDomain)}/applications/${clientId}/settings`;
}

export function portalEditUrl(tenantDomain: string, portalId: string): string {
  return `${dashboardRoot(tenantDomain)}/universal-portals/${portalId}/edit`;
}

// Forms and flows live in the Auth0 Forms app, under the tenant's path.
function formsRoot(tenantDomain: string): string {
  const { region, name } = tenantLocation(tenantDomain);
  return `https://forms.auth0.com/tenants/${region}/${name}`;
}

export function formEditUrl(tenantDomain: string, formId: string): string {
  return `${formsRoot(tenantDomain)}/forms/${formId}/edit`;
}

export function flowEditUrl(tenantDomain: string, flowId: string): string {
  return `${formsRoot(tenantDomain)}/flows/${flowId}/edit`;
}

// Vault connections are listed per app type, so the link opens the AUTH0 list.
export function vaultAppsUrl(tenantDomain: string): string {
  return `${formsRoot(tenantDomain)}/vault/apps/AUTH0/edit`;
}
