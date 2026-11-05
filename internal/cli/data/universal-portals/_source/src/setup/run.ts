import { ApiClient } from "../api.js";
import { PluginError } from "../plugin-error.js";
import { scopeFixes } from "../scope-fixes.js";
import { applicationUrl, flowEditUrl, formEditUrl, portalEditUrl } from "../dashboard.js";
import { buildPortalForms, FORM_NAMES, FormKey } from "../forms.js";
import { isValidPortalSlug } from "../portal-model.js";
import { choiceGaps, Probe, requiredMissingScopes, runPreflight, ScopeGap, SETUP_PROBES } from "./preflight.js";
import { portalUrl, resolvePortalHost } from "./host.js";
import { ensureFormsVault } from "./vault.js";
import type { Prompter } from "../prompt.js";
import type { TemplateId } from "../templates/types.js";
import type { Ui } from "../ui.js";
import { chooseTestLink, ORGANIZATION_PLACEHOLDER } from "./business.js";
import { BusinessAccessError } from "./business-types.js";
import {
  audiencesFor,
  buildPortalRequest,
  myAccountAudience,
  myOrganizationAudience,
  PORTAL_APP_SUFFIX,
  templateFor,
} from "./payload.js";
import {
  ensureClientGrant,
  ensureForms,
  FormRef,
  ensurePortal,
  ensurePortalClient,
  ensureResourceServer,
  describe,
  Ensured,
  findClientByName,
  findPortalBySlug,
  SlugTakenError,
  Status,
} from "./resources.js";


// Another portal took the slug while setup was running. Setup checks the slug before it
// creates anything, so this is a race, and the rerun needs a new name and slug.
export class SlugRaceError extends PluginError {
  constructor(readonly slug: string) {
    super(
      `The slug "${slug}" was taken while setup was running. Rerun with a different --name and --slug.`,
      "slug_race",
    );
  }
}

// When any one of several scopes is enough, the first is the one to ask for.
export class MissingScopesError extends PluginError {
  constructor(
    readonly scopes: string[],
    readonly choices: ScopeGap[],
  ) {
    const needs = [...scopes, ...choices.map((choice) => `one of ${choice.scopes.join(" or ")}`)];
    super(
      `The CLI session is missing ${needs.length} permission(s) setup needs: ${needs.join(", ")}. Nothing was created.`,
      "missing_scopes",
      needs.length > 0 ? scopeFixes([...scopes, ...choices.map((choice) => choice.scopes[0])]) : undefined,
    );
  }
}

// A 403 that names no scope. The Management API sends one when Universal Portals is not enabled,
// and the CLI can drop the reason, so setup stops here rather than after creating resources.
export class PermissionCheckError extends PluginError {
  constructor(probes: string[], tenantDomain: string) {
    super(
      `The Management API refused ${probes.map((probe) => `"${probe}"`).join(", ")} with HTTP 403 and gave no reason. ` +
        `Nothing was created. Usually Universal Portals is not enabled for ${tenantDomain}, or the session is missing a permission.`,
      "permission_check_failed",
      ["Check that Universal Portals is enabled for this tenant.", ...scopeFixes([])],
    );
  }
}

export interface SetupOptions {
  template: TemplateId;
  name: string;
  slug: string;
  userQuery?: string;
  organization?: string;
}

export interface SetupDeps {
  api: ApiClient;
  ui: Ui;
  prompt: Prompter;
  interactive: boolean;
  tenantDomain: string;
  // Sends the permission check. A dry run passes the real client here, because the probes
  // change nothing. Defaults to api.
  probeApi?: ApiClient;
}

export interface SetupResult {
  template: TemplateId;
  portal: { id: string; slug: string; url: string; status: Status };
  application: { clientId: string; status: Status };
  vault: { id: string; status: Status };
  forms: Record<FormKey, { id: string; status: Status }>;
  resources: { type: string; id: string; status: Status }[];
  organization?: { id: string; name: string; hasRole: boolean };
}

export async function runSetup(options: SetupOptions, deps: SetupDeps): Promise<SetupResult> {
  const resources: SetupResult["resources"] = [];
  const { ui, api, tenantDomain } = deps;

  const probes = SETUP_PROBES.filter((probe) => !probe.businessOnly || options.template === "business");
  await checkPermissions(probes, deps);

  let slug = options.slug;
  if (!isValidPortalSlug(slug)) throw new Error(`The slug "${slug}" must match ^[a-z][a-z0-9-]{0,99}$.`);

  // A non-interactive run has nobody to ask, so it needs the user up front.
  if (options.template === "business" && !options.userQuery && !deps.interactive) {
    throw new BusinessAccessError("A Business portal needs the user who will test it.", [
      "Rerun with --user <email or user id>.",
    ]);
  }

  const appName = `${options.name} ${PORTAL_APP_SUFFIX}`;
  slug = await availableSlug(api, slug, appName, deps);
  // The test user (or the organization for the link) is settled before anything is created.
  const testLink = options.template === "business" ? await chooseTestLink(options, deps) : undefined;

  const host = await resolvePortalHost(api, tenantDomain);

  ui.section("Resource servers");
  for (const server of resourceServersFor(options.template, tenantDomain)) {
    const outcome = await ui.task(`Checking ${server.name}`, () => ensureResourceServer(api, server));
    resources.push({ type: "resource_server", id: outcome.value.id, status: outcome.status });
    ui.success(`"${server.name}"`, describe(outcome.status));
  }
  ui.result("Resource servers ready");

  ui.section("Application");
  const client = await ui.task("Creating application", () =>
    ensurePortalClient(api, {
      name: appName,
      slug,
      template: options.template,
      tenantDomain,
      portalHost: host.domain,
      isCustomDomain: host.isCustom,
    }),
  );
  resources.push({ type: "application", id: client.value.clientId, status: client.status });
  ui.success(`"${appName}"`, describe(client.status));
  ui.reference("Application", client.value.clientId, applicationUrl(tenantDomain, client.value.clientId));

  for (const audience of audiencesFor(options.template, tenantDomain)) {
    const grant = await ui.task(`Granting ${apiLabel(audience.audience, tenantDomain)}`, () =>
      ensureClientGrant(api, {
        clientId: client.value.clientId,
        audience: audience.audience,
        scope: audience.scope,
        subjectType: "user",
      }),
    );
    resources.push({ type: "client_grant", id: grant.value.id, status: grant.status });
    ui.success(`"${apiLabel(audience.audience, tenantDomain)}" access`, describe(grant.status));
  }
  ui.result("Application ready");

  ui.section("Forms");
  const vault = await ensureFormsVault(api, deps.ui, tenantDomain, resources);
  const forms = await ui.task("Creating forms", () => ensureForms(api, buildPortalForms(vault.value.id)));
  for (const [key, outcome] of Object.entries(forms) as [FormKey, Ensured<FormRef>][]) {
    resources.push({ type: "form", id: outcome.value.id, status: outcome.status });
    ui.success(`"${FORM_NAMES[key]}"`, describe(outcome.status));
    ui.reference("Form", outcome.value.id, formEditUrl(tenantDomain, outcome.value.id));
    if (outcome.value.flowId) ui.reference("Flow", outcome.value.flowId, flowEditUrl(tenantDomain, outcome.value.flowId));
  }
  ui.result("Forms ready");

  const template = templateFor(options.template, {
    personalInfo: forms.personalInfo.value.id,
    privacyConsent: forms.privacyConsent.value.id,
    communicationPreferences: forms.communicationPreferences.value.id,
  });

  ui.section("Portal");
  let portal: Ensured<{ id: string; slug: string }>;
  try {
    portal = await ui.task("Creating portal", () =>
      ensurePortal(
        api,
        buildPortalRequest(template, {
          slug,
          name: options.name,
          clientId: client.value.clientId,
          clientSecret: client.value.clientSecret,
        }),
      ),
    );
  } catch (error) {
    if (!(error instanceof SlugTakenError)) throw error;
    // No prompt can run inside the task, so the run stops. The resources created so far
    // stay, and a new name gives a new application, so the rerun needs both a new name and slug.
    throw new SlugRaceError(slug);
  }
  resources.push({ type: "portal", id: portal.value.id, status: portal.status });
  ui.success(`"${options.name}"`, describe(portal.status));
  ui.result("Portal ready");
  ui.reference("Portal", portal.value.id, portalEditUrl(tenantDomain, portal.value.id));

  const url = portalUrl(host.domain, portal.value.slug, testLink?.organizationId);
  if (testLink?.isPlaceholder) {
    ui.info(`Replace ${ORGANIZATION_PLACEHOLDER} in the link with the organization ID to test with.`);
  }

  return {
    template: options.template,
    portal: { id: portal.value.id, slug: portal.value.slug, url, status: portal.status },
    application: { clientId: client.value.clientId, status: client.status },
    vault: { id: vault.value.id, status: vault.status },
    forms: Object.fromEntries(
      Object.entries(forms).map(([key, outcome]) => [key, { id: outcome.value.id, status: outcome.status }]),
    ) as SetupResult["forms"],
    resources,
    organization: testLink?.access
      ? {
          id: testLink.access.organizationId,
          name: testLink.access.organizationName,
          hasRole: testLink.access.hasOrganizationRole,
        }
      : undefined,
  };
}

async function checkPermissions(probes: Probe[], deps: SetupDeps): Promise<void> {
  deps.ui.section("Permissions");
  const api = deps.probeApi ?? deps.api;
  const result = await deps.ui.task("Checking permissions", () => runPreflight(api, probes, deps.tenantDomain));
  const refused = result.unexpected.filter((issue) => issue.status === 403);
  if (refused.length > 0) throw new PermissionCheckError(refused.map((issue) => issue.probe), deps.tenantDomain);

  const required = requiredMissingScopes(result);
  const choices = choiceGaps(result);
  // The error lists the scopes and the fixes once, through reportError.
  if (required.length > 0 || choices.length > 0) throw new MissingScopesError(required, choices);

  for (const issue of result.unexpected) {
    const status = issue.message.includes(String(issue.status)) ? "" : ` (HTTP ${issue.status})`;
    deps.ui.warn(`Could not verify "${issue.probe}": ${issue.message}${status}`);
  }
  deps.ui.result(
    result.unexpected.length > 0
      ? "No permission is missing, but the checks above could not be verified"
      : "The CLI session has every permission setup needs",
  );
}

function resourceServersFor(template: TemplateId, tenantDomain: string): { name: string; identifier: string }[] {
  const account = { name: "Auth0 My Account API", identifier: myAccountAudience(tenantDomain) };
  if (template === "consumer") return [account];
  return [{ name: "Auth0 My Organization API", identifier: myOrganizationAudience(tenantDomain) }, account];
}

function apiLabel(audience: string, tenantDomain: string): string {
  if (audience === myAccountAudience(tenantDomain)) return "My Account API";
  if (audience === myOrganizationAudience(tenantDomain)) return "My Organization API";
  return "Management API";
}

// Runs before anything is created: a slug used by another portal is fixed here, not after the
// application and forms exist. A portal already using this setup's application is a rerun, so it passes.
async function availableSlug(api: ApiClient, slug: string, appName: string, deps: SetupDeps): Promise<string> {
  let current = slug;
  for (;;) {
    const portal = await findPortalBySlug(api, current);
    if (!portal) return current;
    const app = await findClientByName(api, appName);
    if (app !== undefined && portal.client?.client_id === app.client_id) return current;
    if (!deps.interactive) {
      throw new Error(`The slug "${current}" is already used by another portal. Rerun with a different --slug.`);
    }
    deps.ui.warn(`The slug "${current}" is already taken.`);
    current = await askValidSlug(deps.prompt, `${current}-2`, deps.ui);
  }
}

async function askValidSlug(prompt: Prompter, suggestion: string, ui: Ui): Promise<string> {
  for (;;) {
    const answer = await prompt.ask("Slug", suggestion);
    if (isValidPortalSlug(answer)) return answer;
    ui.error("The slug must start with a lowercase letter and use only lowercase letters, digits and hyphens.");
  }
}
