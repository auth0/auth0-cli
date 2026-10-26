import type { PortalPage, SidebarComponent } from "../portal-model.js";

// Forms are created by setup, so their IDs are only known at runtime.
export interface FormIds {
  personalInfo: string;
  privacyConsent: string;
  communicationPreferences: string;
}

export type TemplateId = "consumer" | "business";

export interface PortalTemplate {
  id: TemplateId;
  apis: ("my-account" | "my-organization")[];
  defaultPage: string;
  sidebar: SidebarComponent[];
  pages: PortalPage[];
}
