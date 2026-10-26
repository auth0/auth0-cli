import type { PortalPage, SidebarComponent } from "../portal-model.js";
import { SIDEBAR_ICONS } from "./icons.js";
import { SECURITY_PAGE, SECURITY_SIDEBAR_ICON } from "./security.js";
import type { FormIds, PortalTemplate } from "./types.js";

export function consumerTemplate(forms: FormIds): PortalTemplate {
  const pages: PortalPage[] = [
    {
      title: "Profile",
      slug: "profile",
      components: [
        {
          type: "page:component:auth0:structure:section",
          config: {
            title: "Personal information",
            description:
              "Basic info about you, like your name and contact details, that you use across services.",
            variant: "card",
            children: [
              {
                type: "page:component:auth0:form",
                config: {
                  form_id: forms.personalInfo,
                  completion_message: "Your personal information has been updated.",
                },
              },
            ],
          },
        },
      ],
    },
    SECURITY_PAGE,
    {
      title: "Legal & privacy",
      slug: "legal-privacy",
      components: [
        {
          type: "page:component:auth0:structure:section",
          config: {
            title: "Privacy & data consent",
            description: "Control how your personal data is collected and used across our services.",
            variant: "card",
            children: [
              {
                type: "page:component:auth0:form",
                config: {
                  form_id: forms.privacyConsent,
                  completion_message: "Your privacy preferences have been saved.",
                },
              },
            ],
          },
        },
        {
          type: "page:component:auth0:structure:section",
          config: {
            title: "Communication preferences",
            description: "Choose which emails and notifications you'd like to receive from us.",
            variant: "card",
            children: [
              {
                type: "page:component:auth0:form",
                config: {
                  form_id: forms.communicationPreferences,
                  completion_message: "Your communication preferences have been updated.",
                },
              },
            ],
          },
        },
      ],
    },
  ];

  const sidebar: SidebarComponent[] = [
    link("Profile", "profile", SIDEBAR_ICONS.profile),
    link("Security", "security", SECURITY_SIDEBAR_ICON),
    link("Legal & privacy", "legal-privacy", SIDEBAR_ICONS.legalPrivacy),
  ];

  return {
    id: "consumer",
    apis: ["my-account"],
    defaultPage: "profile",
    sidebar,
    pages,
  };
}

function link(label: string, pageSlug: string, icon: string): SidebarComponent {
  return { type: "sidebar:component:auth0:internal_link:v2", config: { label, page_slug: pageSlug, icon } };
}
