import type { PortalPage, SidebarComponent } from "../portal-model.js";
import { SIDEBAR_ICONS } from "./icons.js";
import { SECURITY_PAGE, SECURITY_SIDEBAR_ICON } from "./security.js";
import type { FormIds, PortalTemplate } from "./types.js";

export function businessTemplate(forms: Pick<FormIds, "personalInfo" | "communicationPreferences">): PortalTemplate {
  const pages: PortalPage[] = [
    {
      title: "Organization",
      slug: "organization",
      components: [
        {
          type: "page:component:auth0:structure:section",
          config: {
            title: "Organization details",
            description: "Update your organization's name and other details visible to its members.",
            variant: "card",
            children: [{ type: "page:component:auth0:my_organization:details_edit" }],
          },
        },
      ],
    },
    {
      title: "Domains",
      slug: "domains",
      components: [
        {
          type: "page:component:auth0:structure:section",
          config: {
            title: "Verified domains",
            description: "Add and verify the domains that your organization members sign in with.",
            variant: "card",
            children: [{ type: "page:component:auth0:my_organization:domain_table" }],
          },
        },
      ],
    },
    SECURITY_PAGE,
    {
      title: "Preferences",
      slug: "preferences",
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
    link("Organization", "organization", SIDEBAR_ICONS.organization),
    link("Domains", "domains", SIDEBAR_ICONS.domains),
    link("Security", "security", SECURITY_SIDEBAR_ICON),
    link("Preferences", "preferences", SIDEBAR_ICONS.preferences),
  ];

  return {
    id: "business",
    apis: ["my-organization", "my-account"],
    defaultPage: "organization",
    sidebar,
    pages,
  };
}

function link(label: string, pageSlug: string, icon: string): SidebarComponent {
  return { type: "sidebar:component:auth0:internal_link:v2", config: { label, page_slug: pageSlug, icon } };
}
