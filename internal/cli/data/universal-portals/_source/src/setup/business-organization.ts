import { ApiClient, ApiError, listAll } from "../api.js";
import { BusinessAccessError, BusinessDeps, OrganizationRecord, userLabel, UserRecord } from "./business-types.js";

// Organization names are identifiers: lowercase letters, numbers, hyphens and
// underscores, and unique in the tenant. The display name is free text.
const ORG_NAME_PATTERN = /^[a-z0-9_-]{1,50}$/;

function suggestOrgName(displayName: string): string {
  return displayName.toLowerCase().replace(/[^a-z0-9_-]+/g, "-").replace(/^-+|-+$/g, "").slice(0, 50);
}

// Creates an organization, enables a database connection on it, and adds the
// user. Each write is confirmed first. A taken name asks again, as the portal
// slug does, with a suggestion that is free to use.
export async function createOrganization(user: UserRecord, deps: BusinessDeps): Promise<OrganizationRecord> {
  const displayName = (await deps.prompt.ask("Organization display name", undefined, "Acme Corp")).trim();
  if (!displayName) throw new BusinessAccessError("The organization needs a display name.", []);

  let suggestion = suggestOrgName(displayName);
  for (;;) {
    const orgName = await askOrgName(deps, suggestion);

    const confirmed = await deps.prompt.confirm(
      `Create organization "${displayName}" (${orgName}), enable a database connection on it, and add ${userLabel(user)}?`,
      true,
    );
    if (!confirmed) {
      throw new BusinessAccessError("No organization was created.", [
        `Create one: auth0 orgs create --name ${orgName} --display "${displayName}"`,
      ]);
    }

    let created: OrganizationRecord;
    try {
      created = await deps.api.request<OrganizationRecord>("POST", "organizations", {
        name: orgName,
        display_name: displayName,
      });
    } catch (error) {
      // Only a taken name asks again. Later failures happen after the organization
      // exists, so asking for a new name would create a second one.
      if (!(error instanceof ApiError) || error.status !== 409) throw error;
      deps.ui.warn(`The organization name "${orgName}" is already used in this tenant.`);
      suggestion = `${orgName}-2`.slice(0, 50);
      continue;
    }
    deps.ui.success(`Created organization "${displayName}"`);

    const connection = await chooseDatabaseConnection(deps);
    await deps.api.request("POST", `organizations/${encodeURIComponent(created.id)}/enabled_connections`, {
      connection_id: connection,
      assign_membership_on_login: false,
    });
    await deps.api.request("POST", `organizations/${encodeURIComponent(created.id)}/members`, {
      members: [user.user_id],
    });
    deps.ui.success(`Added ${userLabel(user)} to ${displayName}`);
    return created;
  }
}

async function askOrgName(deps: BusinessDeps, suggestion: string): Promise<string> {
  for (;;) {
    const answer = (await deps.prompt.ask("Organization name", suggestion)).trim();
    if (ORG_NAME_PATTERN.test(answer)) return answer;
    deps.ui.error("Use lowercase letters, numbers, hyphens and underscores, up to 50 characters.");
  }
}

async function chooseDatabaseConnection(deps: BusinessDeps): Promise<string> {
  const connections = (await listAll<{ id: string; name: string; strategy: string }>(deps.api, "connections", "connections")).filter((connection) => connection.strategy === "auth0");
  if (connections.length === 0) {
    throw new BusinessAccessError("This tenant has no database connection to sign in with.", [
      "Create one in the dashboard, then rerun setup.",
    ]);
  }
  if (connections.length === 1) return connections[0].id;
  return deps.prompt.choose(
    "Database connection for the organization",
    connections.map((connection) => ({ label: connection.name, value: connection.id })),
  );
}

export async function requireEnabledConnection(organization: OrganizationRecord, api: ApiClient): Promise<void> {
  const connections = await listAll<{ connection_id: string }>(api, `organizations/${encodeURIComponent(organization.id)}/enabled_connections`, "enabled_connections");
  if (connections.length === 0) {
    throw new BusinessAccessError(`Organization "${organization.name}" has no enabled connections, so members cannot sign in.`, [
      `Enable one: auth0 api post "organizations/${organization.id}/enabled_connections" --data '{"connection_id":"<connection-id>","assign_membership_on_login":false}'`,
    ]);
  }
}
