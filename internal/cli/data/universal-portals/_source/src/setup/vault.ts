import type { ApiClient } from "../api.js";
import { vaultAppsUrl } from "../dashboard.js";
import type { Ui } from "../ui.js";
import { FORMS_VAULT_SCOPES, managementAudience } from "./payload.js";
import type { SetupResult } from "./run.js";
import {
  describe,
  ensureClientGrant,
  ensureFormsVaultClient,
  Ensured,
  ensureVaultConnection,
  refreshVaultConnection,
} from "./resources.js";

export const FORMS_VAULT_NAME = "Universal Portals forms vault";

// Shared forms vault: one machine-to-machine app and one vault connection per
// tenant. A scope change needs a vault refresh, which sends setup again so the
// vault mints a session with the new grants.
export async function ensureFormsVault(
  api: ApiClient,
  ui: Ui,
  tenantDomain: string,
  resources: SetupResult["resources"],
): Promise<Ensured<{ id: string }>> {
  const app = await ui.task("Preparing the forms vault", () => ensureFormsVaultClient(api, FORMS_VAULT_NAME));
  const grant = await ensureClientGrant(api, {
    clientId: app.value.clientId,
    audience: managementAudience(tenantDomain),
    scope: FORMS_VAULT_SCOPES,
    subjectType: "client",
  });

  const setup = { domain: tenantDomain, clientId: app.value.clientId, clientSecret: app.value.clientSecret };
  const vault = await ensureVaultConnection(api, FORMS_VAULT_NAME, setup);
  if (grant.status === "updated" && vault.status === "reused") {
    await refreshVaultConnection(api, vault.value.id, setup);
    ui.success("Forms vault refreshed with the updated permissions");
  }
  resources.push({ type: "forms_vault_client", id: app.value.clientId, status: app.status });
  resources.push({ type: "forms_vault_grant", id: grant.value.id, status: grant.status });
  resources.push({ type: "forms_vault", id: vault.value.id, status: vault.status });
  ui.success(`"${FORMS_VAULT_NAME}"`, describe(vault.status));
  // Vault connections have no page of their own, so the ID links to the AUTH0 app type.
  ui.reference("Vault connection", vault.value.id, vaultAppsUrl(tenantDomain));
  return vault;
}
