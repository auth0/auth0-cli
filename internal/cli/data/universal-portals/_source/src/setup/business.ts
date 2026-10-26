import { ApiClient, ApiError, listAll } from "../api.js";
import type { Choice } from "../prompt.js";
import { createOrganization, requireEnabledConnection } from "./business-organization.js";
import { ensureOrganizationRole } from "./business-role.js";
import {
  BusinessAccess,
  BusinessAccessError,
  BusinessDeps,
  BusinessInput,
  OrganizationRecord,
  userLabel,
  UserRecord,
} from "./business-types.js";

// A Business portal is opened for an organization, so the user who will test
// it has to belong to that organization, with a connection to sign in with and
// a role. Setup asks for the user before it creates the portal, and checks each
// step before it changes anything.

export async function resolveBusinessAccess(input: BusinessInput, deps: BusinessDeps): Promise<BusinessAccess> {
  const user = await chooseUser(input.userQuery, deps);
  deps.ui.success(`"${userLabel(user)}" found`);

  const organization = await chooseOrganization(user, input.organization, deps);
  deps.ui.success(`Organization "${organization.name}" selected`);

  await requireEnabledConnection(organization, deps.api);
  const hasOrganizationRole = await ensureOrganizationRole(user, organization, deps);

  return {
    userId: user.user_id,
    organizationId: organization.id,
    organizationName: organization.name,
    hasOrganizationRole,
  };
}

function describeUser(user: UserRecord): string {
  return `${user.email ?? "no email"} · ${user.user_id}${user.name ? ` · ${user.name}` : ""}`;
}

// Looks a user up by email, or by ID when the query has no "@". An unknown ID
// is an empty result, not an error, so the caller can ask again.
async function lookupUsers(query: string, api: ApiClient): Promise<UserRecord[]> {
  if (query.includes("@")) {
    // The search matches similar addresses too, so only the exact email is kept. The
    // search reads users, so it needs read:users, the same scope as the lookup by ID.
    const phrase = `email:"${query.replace(/["\\]/g, "\\$&")}"`;
    const found = await listAll<UserRecord>(api, `users?q=${encodeURIComponent(phrase)}&search_engine=v3`, "users");
    return found.filter((user) => user.email?.toLowerCase() === query.toLowerCase());
  }
  try {
    return [await api.request<UserRecord>("GET", `users/${encodeURIComponent(query)}`)];
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) return [];
    throw error;
  }
}

async function chooseUser(initialQuery: string | undefined, deps: BusinessDeps): Promise<UserRecord> {
  let query = initialQuery;
  for (;;) {
    if (!query) {
      if (!deps.interactive) {
        throw new BusinessAccessError("A Business portal needs the user who will test it.", [
          "Rerun with --user <email or user id>.",
        ]);
      }
      query = (await deps.prompt.ask("Enter the test user's email or user ID", undefined, "jane@acme.com")).trim();
      if (!query) continue;
    }

    const matches = await lookupUsers(query, deps.api);
    if (matches.length === 1) return matches[0];

    if (!deps.interactive) {
      if (matches.length === 0) {
        throw new BusinessAccessError(`No user matches "${query}".`, ["Check the email or user ID, then rerun with --user."]);
      }
      throw new BusinessAccessError(`More than one user has the email "${query}".`, [
        "Rerun with the user ID instead: " + matches.map((user) => user.user_id).join(", "),
      ]);
    }

    if (matches.length === 0) {
      deps.ui.warn(`No user matches "${query}".`);
      query = undefined;
      continue;
    }

    const choice = await deps.prompt.choose<UserRecord | "retry">(`${matches.length} users match "${query}"`, [
      ...matches.map((user): Choice<UserRecord | "retry"> => ({ label: describeUser(user), value: user })),
      { label: "Try another email or user ID", value: "retry" },
    ]);
    if (choice === "retry") {
      query = undefined;
      continue;
    }
    return choice;
  }
}

async function chooseOrganization(
  user: UserRecord,
  requested: string | undefined,
  deps: BusinessDeps,
): Promise<OrganizationRecord> {
  const memberships = await listAll<OrganizationRecord>(deps.api, `users/${encodeURIComponent(user.user_id)}/organizations`, "organizations");

  if (requested) {
    const match = memberships.find((org) => org.id === requested || org.name === requested);
    if (match) return match;
    throw new BusinessAccessError(`${userLabel(user)} is not a member of "${requested}".`, [
      `Add them: auth0 api post "organizations/${requested}/members" --data '{"members":["${user.user_id}"]}'`,
    ]);
  }

  if (!deps.interactive) {
    if (memberships.length === 1) return memberships[0];
    if (memberships.length > 1) {
      throw new BusinessAccessError(`${userLabel(user)} belongs to ${memberships.length} organizations.`, [
        "Rerun with --organization <id>: " + memberships.map((org) => org.id).join(", "),
      ]);
    }
    throw new BusinessAccessError(`${userLabel(user)} is not a member of any organization.`, [
      `Add them: auth0 api post "organizations/<org-id>/members" --data '{"members":["${user.user_id}"]}'`,
    ]);
  }

  if (memberships.length === 1) return memberships[0];

  // Members pick one of their organizations. Users with none can join an
  // existing organization of the tenant. Either way a new one can be created.
  const candidates = memberships.length > 0 ? memberships : await listTenantOrganizations(deps.api);
  if (memberships.length === 0) deps.ui.warn(`${userLabel(user)} is not a member of any organization.`);

  deps.ui.info("The portal opens in one organization, and the test user has to be a member of it.");
  const choice = await deps.prompt.choose<OrganizationRecord | "create">("Choose the organization to test the portal in", [
    ...candidates.map((org): Choice<OrganizationRecord | "create"> => ({
      label: `${org.name} (${org.id})`,
      value: org,
    })),
    { label: "None of these, create a new organization", value: "create" },
  ]);

  if (choice === "create") return createOrganization(user, deps);

  if (memberships.length > 0) return choice;
  return addToOrganization(user, choice, deps);
}

async function listTenantOrganizations(api: ApiClient): Promise<OrganizationRecord[]> {
  return await listAll<OrganizationRecord>(api, "organizations", "organizations");
}

async function addToOrganization(
  user: UserRecord,
  organization: OrganizationRecord,
  deps: BusinessDeps,
): Promise<OrganizationRecord> {
  const confirmed = await deps.prompt.confirm(`Add ${userLabel(user)} as a member of ${organization.name}?`, true);
  if (!confirmed) {
    throw new BusinessAccessError(`${userLabel(user)} is not a member of an organization, and setup did not add them.`, [
      `Add them: auth0 api post "organizations/${organization.id}/members" --data '{"members":["${user.user_id}"]}'`,
    ]);
  }
  await deps.api.request("POST", `organizations/${encodeURIComponent(organization.id)}/members`, {
    members: [user.user_id],
  });
  deps.ui.success(`Added ${userLabel(user)} to ${organization.name}`);
  return organization;
}

const BUSINESS_REQUIREMENTS =
  "Business portals only work in the context of an organization: without it, the organization components do not load. Pick the user who will test the portal, so the link opens in their organization.";
const SKIPPED_TEST_USER_REQUIREMENTS =
  "Without a test user, the link needs an organization. The portal only signs members in when the URL carries ?organization=<id>.";
// The placeholder the link keeps until the user replaces it with an organization ID.
export const ORGANIZATION_PLACEHOLDER = "<ORGANIZATION_ID>";

// The link the setup prints: the organization it opens in, and the test user's access
// when one was chosen.
export interface TestLink {
  organizationId: string;
  // True while the organization is still the placeholder the user has to replace.
  isPlaceholder: boolean;
  access?: BusinessAccess;
}

// Settles the test user before anything is created, so a user who cannot sign in
// stops the run with nothing to clean up. Skipping it asks for the organization ID
// for the link instead, with a placeholder when none is given.
export async function chooseTestLink(input: BusinessInput, deps: BusinessDeps): Promise<TestLink> {
  const { ui, prompt, interactive } = deps;
  ui.section("Test user");

  let userQuery = input.userQuery;
  if (userQuery === undefined && interactive) {
    ui.info(BUSINESS_REQUIREMENTS);
    userQuery = (await prompt.ask("Enter the test user's email or user ID (press Enter to skip)", undefined, "jane@acme.com")).trim() || undefined;
  }

  if (userQuery) {
    const access = await resolveBusinessAccess({ userQuery, organization: input.organization }, deps);
    return { organizationId: access.organizationId, isPlaceholder: false, access };
  }

  if (interactive) ui.info(SKIPPED_TEST_USER_REQUIREMENTS);
  const organizationId = await prompt.ask(
    "Enter the organization ID to test in (the link adds ?organization=<id>)",
    ORGANIZATION_PLACEHOLDER,
  );
  return { organizationId, isPlaceholder: organizationId === ORGANIZATION_PLACEHOLDER };
}
