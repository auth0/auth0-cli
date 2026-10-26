import { strict as assert } from "node:assert";
import { test } from "node:test";

import type { PageComponent, PortalPage, SidebarComponent } from "../src/portal-model.js";
import { LIMITS } from "../src/portal-model.js";
import { ICON_NAME_PATTERN, SIDEBAR_ICONS } from "../src/templates/icons.js";
import { businessTemplate } from "../src/templates/business.js";
import { consumerTemplate } from "../src/templates/consumer.js";
import type { FormIds, PortalTemplate } from "../src/templates/types.js";

const FORMS: FormIds = {
  personalInfo: "ap_personal",
  privacyConsent: "ap_privacy",
  communicationPreferences: "ap_comms",
};

function sidebarPageSlugs(sidebar: SidebarComponent[]): string[] {
  return sidebar.map((item) => {
    assert.equal(item.type, "sidebar:component:auth0:internal_link:v2");
    return item.config.page_slug;
  });
}

function formIdsUsed(pages: PortalPage[]): string[] {
  return pages.flatMap((page) =>
    (page.components ?? []).flatMap((component) =>
      component.type === "page:component:auth0:structure:section"
        ? component.config.children.flatMap((child) =>
            child.type === "page:component:auth0:form" ? [child.config.form_id] : [],
          )
        : [],
    ),
  );
}

function allComponents(pages: PortalPage[]): PageComponent[] {
  return pages.flatMap((page) => page.components ?? []);
}

function templatesUnderTest(): PortalTemplate[] {
  return [consumerTemplate(FORMS), businessTemplate(FORMS)];
}

for (const template of templatesUnderTest()) {
  test(`${template.id}: every sidebar link points to an existing page`, () => {
    const pageSlugs = template.pages.map((page) => page.slug);
    for (const slug of sidebarPageSlugs(template.sidebar)) {
      assert.ok(pageSlugs.includes(slug), `sidebar link to missing page "${slug}"`);
    }
  });

  test(`${template.id}: every page is in the sidebar`, () => {
    const linked = sidebarPageSlugs(template.sidebar);
    for (const page of template.pages) {
      assert.ok(linked.includes(page.slug), `page "${page.slug}" is not in the navigation`);
    }
  });

  test(`${template.id}: the default page is one of the pages`, () => {
    assert.ok(template.pages.some((page) => page.slug === template.defaultPage));
  });

  test(`${template.id}: every sidebar icon matches the allowed name pattern`, () => {
    for (const item of template.sidebar) {
      const icon = item.config.icon;
      assert.ok(icon !== undefined && ICON_NAME_PATTERN.test(icon), `bad icon "${icon}"`);
    }
  });

  test(`${template.id}: every page title fits the limits`, () => {
    for (const page of template.pages) {
      assert.ok(page.title.length >= LIMITS.pageTitle.min && page.title.length <= LIMITS.pageTitle.max);
    }
  });

  test(`${template.id}: every component is a section, and sections hold only leaf components`, () => {
    for (const component of allComponents(template.pages)) {
      assert.equal(component.type, "page:component:auth0:structure:section", `top-level ${component.type} is not in a section`);
      const sectionConfig = (component as Extract<PageComponent, { type: "page:component:auth0:structure:section" }>).config;
      assert.ok(sectionConfig.children.length <= LIMITS.sectionChildren.max);
      for (const child of sectionConfig.children) {
        assert.notEqual(child.type, "page:component:auth0:structure:section", "sections cannot be nested");
      }
    }
  });

  test(`${template.id}: every form used exists in the supplied form ids`, () => {
    const known = Object.values(FORMS);
    for (const formId of formIdsUsed(template.pages)) {
      assert.ok(known.includes(formId), `unknown form "${formId}"`);
    }
  });
}

test("consumer: uses only the My Account API and no organization components", () => {
  const consumer = consumerTemplate(FORMS);
  assert.deepEqual(consumer.apis, ["my-account"]);
  for (const component of allComponents(consumer.pages)) {
    const children = (component as Extract<PageComponent, { type: "page:component:auth0:structure:section" }>).config.children;
    for (const child of children) {
      assert.ok(!child.type.includes("my_organization"), `organization component in consumer: ${child.type}`);
    }
  }
});

test("business: needs both APIs and its home page is the organization page", () => {
  const business = businessTemplate(FORMS);
  assert.deepEqual(business.apis, ["my-organization", "my-account"]);
  assert.equal(business.defaultPage, "organization");
});

test("business: the security page is the same definition as consumer", () => {
  const consumerSecurity = consumerTemplate(FORMS).pages.find((page) => page.slug === "security");
  const businessSecurity = businessTemplate(FORMS).pages.find((page) => page.slug === "security");
  assert.deepEqual(businessSecurity, consumerSecurity);
});

test("business: preferences uses the personal info and comms forms only", () => {
  const preferences = businessTemplate(FORMS).pages.find((page) => page.slug === "preferences");
  assert.ok(preferences);
  assert.deepEqual(formIdsUsed([preferences]).sort(), [FORMS.communicationPreferences, FORMS.personalInfo].sort());
});

test("sidebar icons are the ones named in the icon map", () => {
  const usedIcons = [...consumerTemplate(FORMS).sidebar, ...businessTemplate(FORMS).sidebar].map(
    (item) => item.config.icon,
  );
  for (const icon of usedIcons) {
    assert.ok(Object.values(SIDEBAR_ICONS).includes(icon as never), `icon "${icon}" not in the icon map`);
  }
});
