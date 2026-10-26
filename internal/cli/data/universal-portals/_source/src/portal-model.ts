// Shapes of the Auth0 Portals Management API payloads used by setup
// and, later, audit. Written by hand from the public API behavior.

export const PORTAL_SLUG_PATTERN = /^[a-z][a-z0-9-]{0,99}$/;

export const LIMITS = {
  portalName: { min: 1, max: 150 },
  pageTitle: { min: 1, max: 150 },
  sectionTitle: { max: 50 },
  sectionDescription: { max: 200 },
  sectionChildren: { max: 20 },
  headingTitle: { max: 50 },
  headingDescription: { max: 200 },
  richTextContent: { max: 10000 },
  imageSrc: { max: 500 },
  imageHeight: { min: 1, max: 500 },
  separatorText: { max: 50 },
  formId: { max: 50 },
  completionMessage: { max: 200 },
  sidebarLabel: { max: 50 },
  pageSlug: { max: 50 },
  externalHref: { max: 200 },
  icon: { max: 50 },
  customCodeHead: { max: 10000 },
  clientSecret: { max: 256 },
} as const;

export function isValidPortalSlug(slug: string): boolean {
  return PORTAL_SLUG_PATTERN.test(slug);
}

export type SectionVariant = "card" | "none";
export type SeparatorVariant = "solid" | "dashed" | "none";
export type ImagePosition = "left" | "center" | "right";

export interface FormComponent {
  type: "page:component:auth0:form";
  config: { form_id: string; completion_message: string };
}

export interface HeadingComponent {
  type: "page:component:auth0:typography:heading";
  config: { title: string; description?: string };
}

export interface RichTextComponent {
  type: "page:component:auth0:typography:rich_text";
  config: { content: string };
}

export interface ImageComponent {
  type: "page:component:auth0:media:image";
  config?: { src: string; position?: ImagePosition; height?: number };
}

export interface SeparatorComponent {
  type: "page:component:auth0:structure:separator";
  config?: { variant?: SeparatorVariant; text?: string };
}

// Universal Components: the portal renders them itself and they call the
// My Account or My Organization API. None of them take a config.
export interface UniversalComponent {
  type:
    | "page:component:auth0:my_account:mfa_management"
    | "page:component:auth0:my_account:passkey_management"
    | "page:component:auth0:my_organization:details_edit"
    | "page:component:auth0:my_organization:domain_table";
}

export type LeafComponent =
  | FormComponent
  | HeadingComponent
  | RichTextComponent
  | ImageComponent
  | SeparatorComponent
  | UniversalComponent;

export interface SectionComponent {
  type: "page:component:auth0:structure:section";
  config: {
    title?: string;
    description?: string;
    variant: SectionVariant;
    children: LeafComponent[];
  };
}

export type PageComponent = LeafComponent | SectionComponent;

export interface PortalPage {
  title: string;
  slug: string;
  components?: PageComponent[];
}

export interface SidebarInternalLink {
  type: "sidebar:component:auth0:internal_link:v2";
  config: { label: string; page_slug: string; icon?: string };
}

export interface SidebarExternalLink {
  type: "sidebar:component:auth0:external_link:v2";
  config: { label: string; href: string; icon?: string };
}

export type SidebarComponent = SidebarInternalLink | SidebarExternalLink;

export interface CreatePortalRequest {
  slug: string;
  name: string;
  client: {
    token_endpoint_auth_method: "client_secret_post";
    client_id: string;
    client_secret: string;
  };
  navigation?: { sidebar?: { components?: SidebarComponent[] } };
  pages?: { default?: string; standard?: PortalPage[] };
  custom_code?: { head?: string };
}
