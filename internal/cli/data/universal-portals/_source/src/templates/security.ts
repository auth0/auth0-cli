import type { PortalPage } from "../portal-model.js";
import { SIDEBAR_ICONS } from "./icons.js";

// Shared by both templates, so the account security page is defined once.
export const SECURITY_PAGE: PortalPage = {
  title: "Security",
  slug: "security",
  components: [
    {
      type: "page:component:auth0:structure:section",
      config: {
        title: "Multi-factor authentication",
        description:
          "Add an extra layer of protection to your account by requiring a second verification step each time you sign in.",
        variant: "card",
        children: [{ type: "page:component:auth0:my_account:mfa_management" }],
      },
    },
    {
      type: "page:component:auth0:structure:section",
      config: {
        title: "Passkeys",
        description:
          "Use your fingerprint, face, or screen lock instead of a password to sign in quickly and more securely.",
        variant: "card",
        children: [{ type: "page:component:auth0:my_account:passkey_management" }],
      },
    },
  ],
};

export const SECURITY_SIDEBAR_ICON = SIDEBAR_ICONS.security;
