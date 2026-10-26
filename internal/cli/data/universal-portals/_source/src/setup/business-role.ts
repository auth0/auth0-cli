import { listAll } from "../api.js";
import type { Choice } from "../prompt.js";
import { BusinessDeps, OrganizationRecord, userLabel, UserRecord } from "./business-types.js";
import { MY_ORGANIZATION_SCOPES, myOrganizationAudience } from "./payload.js";

// Returns whether the user's organization roles, together, grant every scope the
// organization components need. Missing scopes are a warning, not a stop: the portal
// still signs the user in, but those components will not work for them.
export async function ensureOrganizationRole(
  user: UserRecord,
  organization: OrganizationRecord,
  deps: BusinessDeps,
): Promise<boolean> {
  const roles = await listAll<{ id: string; name: string }>(
    deps.api,
    `organizations/${encodeURIComponent(organization.id)}/members/${encodeURIComponent(user.user_id)}/roles`,
    "roles",
  );
  const granted = new Set<string>();
  for (const role of roles) {
    for (const scope of await permissionsOf(role.id, deps)) granted.add(scope);
  }
  const missing = MY_ORGANIZATION_SCOPES.filter((scope) => !granted.has(scope));
  if (missing.length === 0) return true;

  const missingReads = missing.filter((scope) => scope.startsWith("read:"));
  const missingWrites = missing.filter((scope) => !scope.startsWith("read:"));
  if (missingReads.length > 0) {
    deps.ui.warn(
      `${userLabel(user)} lacks read permissions for the organization components in "${organization.name}": ${missingReads.join(", ")}. Those components will not open for them.`,
    );
  }
  if (missingWrites.length > 0) {
    deps.ui.warn(
      `${userLabel(user)} lacks write permissions for the organization components in "${organization.name}": ${missingWrites.join(", ")}. The components open, but changes cannot be saved.`,
    );
  }
  if (!deps.interactive) return false;

  const candidates = await rolesGranting(missing, deps);
  let role: TenantRole;
  if (candidates.length === 0) {
    const create = await deps.prompt.confirm(
      `No role in this tenant has these permissions. Create "${PORTAL_ROLE_NAME}" and assign it to ${userLabel(user)}?`,
      true,
    );
    if (!create) return false;
    role = await createPortalRole(deps);
  } else {
    const assign = await deps.prompt.confirm("Assign an organization role now?", true);
    if (!assign) return false;

    const picked = await deps.prompt.choose<TenantRole | "create">("Role to assign", [
      ...candidates.map((candidate): Choice<TenantRole | "create"> => ({ label: candidate.name, value: candidate })),
      { label: `Create "${PORTAL_ROLE_NAME}"`, hint: "only the permissions the portal needs", value: "create" },
    ]);
    role = picked === "create" ? await createPortalRole(deps) : picked;
  }
  await deps.api.request(
    "POST",
    `organizations/${encodeURIComponent(organization.id)}/members/${encodeURIComponent(user.user_id)}/roles`,
    { roles: [role.id] },
  );
  deps.ui.success(`Assigned role ${role.name} to ${userLabel(user)}`);
  return true;
}

// The role the CLI creates when no tenant role has the permissions. It carries only the
// organization permissions of the portal components, nothing more. An existing role with
// the same name is reused, and only the missing permissions are added to it.
export const PORTAL_ROLE_NAME = "Universal Portals - Admin";

interface TenantRole {
  id: string;
  name: string;
}

async function createPortalRole(deps: BusinessDeps): Promise<TenantRole> {
  const tenantRoles = await listAll<TenantRole>(deps.api, "roles", "roles");
  const role =
    tenantRoles.find((candidate) => candidate.name === PORTAL_ROLE_NAME) ??
    (await deps.api.request<TenantRole>("POST", "roles", {
      name: PORTAL_ROLE_NAME,
      description: "The organization permissions the Universal Portals components need.",
    }));

  const granted = await permissionsOf(role.id, deps);
  const missing = MY_ORGANIZATION_SCOPES.filter((scope) => !granted.includes(scope));
  if (missing.length > 0) {
    await deps.api.request("POST", `roles/${encodeURIComponent(role.id)}/permissions`, {
      permissions: missing.map((permission_name) => ({
        permission_name,
        resource_server_identifier: myOrganizationAudience(deps.tenantDomain),
      })),
    });
  }
  return role;
}

async function permissionsOf(roleId: string, deps: BusinessDeps): Promise<string[]> {
  const permissions = await listAll<{ permission_name: string }>(
    deps.api,
    `roles/${encodeURIComponent(roleId)}/permissions`,
    "permissions",
  );
  return permissions.map((permission) => permission.permission_name);
}

// The tenant roles that grant every one of the scopes, so an assignment fixes the warning.
async function rolesGranting(scopes: string[], deps: BusinessDeps): Promise<{ id: string; name: string }[]> {
  const tenantRoles = await listAll<{ id: string; name: string }>(deps.api, "roles", "roles");
  const matches: { id: string; name: string }[] = [];
  for (const role of tenantRoles) {
    const granted = await permissionsOf(role.id, deps);
    if (scopes.every((scope) => granted.includes(scope))) matches.push(role);
  }
  return matches;
}
