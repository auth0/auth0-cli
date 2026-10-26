import type { CreatePortalRequest } from "../portal-model.js";
import type { PortalTemplate, TemplateId } from "../templates/types.js";
import { businessTemplate } from "../templates/business.js";
import { consumerTemplate } from "../templates/consumer.js";
import type { FormIds } from "../templates/types.js";

// Scopes the portal application needs on each resource server, as listed in
// the current Auth0 Portals setup guide. Trim them per component when a
// template uses fewer components.
export const MY_ACCOUNT_SCOPES = [
  "read:me:factors",
  "read:me:authentication_methods",
  "create:me:authentication_methods",
  "delete:me:authentication_methods",
];

// Scopes the Business template's organization components use, from the portal
// component reference. Organization details: read and update. Verified domains:
// domains and identity provider domains, read, create, update and delete.
export const MY_ORGANIZATION_SCOPES = [
  "read:my_org:details",
  "update:my_org:details",
  "read:my_org:configuration",
  "read:my_org:domains",
  "create:my_org:domains",
  "update:my_org:domains",
  "delete:my_org:domains",
  "read:my_org:identity_providers",
  "create:my_org:identity_providers_domains",
  "delete:my_org:identity_providers_domains",
];

// Scopes for the machine-to-machine app that backs the forms' vault connection.
export const FORMS_VAULT_SCOPES = ["update:users", "update:users_app_metadata"];

// Suffix of the portal application's name, after the portal's name.
export const PORTAL_APP_SUFFIX = "(Universal Portals)";

export const PORTAL_GRANT_TYPES = [
  "authorization_code",
  "refresh_token",
  "http://auth0.com/oauth/grant-type/mfa-oob",
  "http://auth0.com/oauth/grant-type/mfa-otp",
  "http://auth0.com/oauth/grant-type/mfa-recovery-code",
];

export const REFRESH_TOKEN_LIFETIMES = {
  idleSeconds: 3600,
  absoluteSeconds: 86400,
};

export function myAccountAudience(tenantDomain: string): string {
  return `https://${tenantDomain}/me/`;
}

export function myOrganizationAudience(tenantDomain: string): string {
  return `https://${tenantDomain}/my-org/`;
}

export function managementAudience(tenantDomain: string): string {
  return `https://${tenantDomain}/api/v2/`;
}

// The audiences a template calls, with the scopes each needs. Setup creates a
// client grant and a refresh-token policy for each.
export function audiencesFor(template: TemplateId, tenantDomain: string): { audience: string; scope: string[] }[] {
  const account = { audience: myAccountAudience(tenantDomain), scope: MY_ACCOUNT_SCOPES };
  if (template === "consumer") return [account];
  return [{ audience: myOrganizationAudience(tenantDomain), scope: MY_ORGANIZATION_SCOPES }, account];
}

export interface PortalClientInput {
  name: string;
  slug: string;
  template: TemplateId;
  tenantDomain: string;
  // Host the portal is served from (the custom domain, or the tenant domain).
  portalHost: string;
  isCustomDomain: boolean;
}

export function buildPortalClient(input: PortalClientInput): Record<string, unknown> {
  const client: Record<string, unknown> = {
    name: input.name,
    app_type: "regular_web",
    is_first_party: true,
    oidc_conformant: true,
    token_endpoint_auth_method: "client_secret_post",
    is_token_endpoint_ip_header_trusted: true,
    callbacks: [`https://${input.portalHost}/portals/auth/callback`],
    // Users land on the portal's own host after logout.
    allowed_logout_urls: [`https://${input.portalHost}`],
    grant_types: PORTAL_GRANT_TYPES,
    organization_usage: "allow",
    organization_require_behavior: "no_prompt",
    refresh_token: refreshTokenSettings(input.template, input.tenantDomain),
    client_metadata: { portals_setup_slug: input.slug },
    // Lets a native app open the portal with a session transfer token.
    session_transfer: { allowed_authentication_methods: ["query"], enforce_device_binding: "ip" },
  };
  if (input.isCustomDomain) {
    client.oidc_logout = backChannelLogout(input.portalHost);
  }
  return client;
}

export function refreshTokenSettings(template: TemplateId, tenantDomain: string): Record<string, unknown> {
  return {
    rotation_type: "non-rotating",
    expiration_type: "expiring",
    token_lifetime: REFRESH_TOKEN_LIFETIMES.absoluteSeconds,
    idle_token_lifetime: REFRESH_TOKEN_LIFETIMES.idleSeconds,
    policies: audiencesFor(template, tenantDomain).map((entry) => ({
      audience: entry.audience,
      scope: entry.scope,
    })),
  };
}

function backChannelLogout(portalHost: string): Record<string, unknown> {
  return {
    backchannel_logout_urls: [`https://${portalHost}/portals/auth/backchannel-logout`],
    backchannel_logout_initiators: {
      mode: "custom",
      selected_initiators: [
        "idp-logout",
        "rp-logout",
        "session-expired",
        "session-revoked",
        "account-deleted",
        "account-deactivated",
      ],
    },
  };
}

export function buildFormsVaultClient(name: string): Record<string, unknown> {
  return {
    name,
    app_type: "non_interactive",
    client_metadata: { auth0_portals_role: "forms-vault" },
  };
}

export function buildPortalRequest(
  template: PortalTemplate,
  input: { slug: string; name: string; clientId: string; clientSecret: string },
): CreatePortalRequest {
  return {
    slug: input.slug,
    name: input.name,
    client: {
      token_endpoint_auth_method: "client_secret_post",
      client_id: input.clientId,
      client_secret: input.clientSecret,
    },
    navigation: { sidebar: { components: template.sidebar } },
    pages: { default: template.defaultPage, standard: template.pages },
  };
}

export function templateFor(id: TemplateId, forms: FormIds): PortalTemplate {
  return id === "consumer" ? consumerTemplate(forms) : businessTemplate(forms);
}
