import { strict as assert } from "node:assert";
import { test } from "node:test";

import { auditPortal } from "../src/audit/checks.js";
import type { PortalSnapshot } from "../src/audit/snapshot.js";

const DOMAIN = "acme.us.auth0.com";
const ME = `https://${DOMAIN}/me/`;
const MY_ORG = `https://${DOMAIN}/my-org/`;

// A healthy consumer portal, shaped like one created by setup.
function healthy(): PortalSnapshot {
  return {
    tenantDomain: DOMAIN,
    portal: {
      id: "up_1",
      slug: "acme",
      client: { client_id: "cli_1" },
      navigation: {
        sidebar: {
          components: [
            { type: "sidebar:component:auth0:internal_link:v2", config: { label: "Profile", page_slug: "profile", icon: "user" } },
            { type: "sidebar:component:auth0:internal_link:v2", config: { label: "Security", page_slug: "security", icon: "shield-check" } },
          ],
        },
      },
      pages: {
        default: "profile",
        standard: [
          {
            title: "Profile",
            slug: "profile",
            components: [
              {
                type: "page:component:auth0:structure:section",
                config: {
                  variant: "card",
                  children: [{ type: "page:component:auth0:form", config: { form_id: "ap_1", completion_message: "Saved" } }],
                },
              },
            ],
          },
          {
            title: "Security",
            slug: "security",
            components: [
              {
                type: "page:component:auth0:structure:section",
                config: {
                  variant: "card",
                  children: [{ type: "page:component:auth0:my_account:mfa_management", config: {} }],
                },
              },
            ],
          },
        ],
      },
    },
    application: {
      client_id: "cli_1",
      grant_types: ["authorization_code", "refresh_token"],
      callbacks: [`https://${DOMAIN}/portals/auth/callback`],
      organization_usage: "allow",
      is_token_endpoint_ip_header_trusted: true,
      session_transfer: { allowed_authentication_methods: ["query"] },
      refresh_token: {
        rotation_type: "non-rotating",
        idle_token_lifetime: 3600,
        token_lifetime: 86400,
        infinite_idle_token_lifetime: false,
        policies: [{ audience: ME, scope: ["read:me:factors"] }],
      },
    },
    grants: [{ audience: ME, scope: ["read:me:factors"] }],
    resourceServers: [ME],
    forms: { ap_1: true },
    applicationConnections: [{ id: "con_db", name: "Username-Password-Authentication" }],
  };
}

function ids(snapshot: PortalSnapshot): string[] {
  return auditPortal(snapshot).map((finding) => finding.id);
}

test("a healthy consumer portal has no findings", () => {
  assert.deepEqual(auditPortal(healthy()), []);
});

test("a missing home page is an error", () => {
  const snapshot = healthy();
  snapshot.portal.pages = { ...snapshot.portal.pages, default: undefined };
  assert.ok(ids(snapshot).includes("P001"));
});

test("a sidebar link in the legacy shape is an error", () => {
  const snapshot = healthy();
  snapshot.portal.navigation = {
    sidebar: { components: [{ type: "sidebar:component:auth0:internal_link", config: { label: "Profile", to: "profile" } }] },
  };
  assert.ok(ids(snapshot).includes("P003"));
});

test("a sidebar link to a missing page is an error", () => {
  const snapshot = healthy();
  snapshot.portal.navigation!.sidebar!.components!.push({
    type: "sidebar:component:auth0:internal_link:v2",
    config: { label: "Billing", page_slug: "billing" },
  });
  assert.ok(ids(snapshot).includes("P004"));
});

test("a page that is not in the sidebar is a warning", () => {
  const snapshot = healthy();
  snapshot.portal.pages!.standard!.push({ title: "Billing", slug: "billing", components: [] });
  const findings = auditPortal(snapshot);
  assert.ok(findings.some((finding) => finding.id === "P005" && finding.severity === "warning"));
});

test("a form the portal uses that does not exist is an error", () => {
  const snapshot = healthy();
  snapshot.forms = { ap_1: false };
  assert.ok(ids(snapshot).includes("P007"));
});

test("client_credentials on the portal application is an error", () => {
  const snapshot = healthy();
  snapshot.application!.grant_types = [...snapshot.application!.grant_types!, "client_credentials"];
  assert.ok(ids(snapshot).includes("P010"));
});

test("a rotating refresh token is an error", () => {
  const snapshot = healthy();
  snapshot.application!.refresh_token!.rotation_type = "rotating";
  assert.ok(ids(snapshot).includes("P013"));
});

test("a refresh token policy missing for a used API is an error", () => {
  const snapshot = healthy();
  snapshot.application!.refresh_token!.policies = [];
  assert.ok(ids(snapshot).includes("P020"));
});

test("no connection available to the application is an error", () => {
  const snapshot = healthy();
  snapshot.applicationConnections = [];
  assert.ok(ids(snapshot).includes("P021"));
});

test("business components without organization usage is an error", () => {
  const snapshot = healthy();
  snapshot.portal.pages!.standard![0].components!.push({
    type: "page:component:auth0:structure:section",
    config: {
      variant: "card",
      children: [{ type: "page:component:auth0:my_organization:details_edit", config: {} }],
    },
  });
  snapshot.application!.organization_usage = "deny";
  snapshot.resourceServers.push(MY_ORG);
  snapshot.grants.push({ audience: MY_ORG, scope: ["read:my_org:details"] });
  snapshot.application!.refresh_token!.policies!.push({ audience: MY_ORG, scope: ["read:my_org:details"] });
  assert.ok(ids(snapshot).includes("P016"));
});

test("an organization with no connection in common with the application is an error", () => {
  const snapshot = healthy();
  snapshot.organization = { id: "org_1", name: "acme", connections: ["con_other"] };
  assert.ok(ids(snapshot).includes("P022"));
});

test("an organization that shares a connection with the application passes", () => {
  const snapshot = healthy();
  snapshot.organization = { id: "org_1", name: "acme", connections: ["con_db"] };
  assert.deepEqual(auditPortal(snapshot), []);
});

test("custom code without a custom domain is a warning", () => {
  const snapshot = healthy();
  snapshot.portal.custom_code = { head: "<script></script>" };
  assert.ok(auditPortal(snapshot).some((finding) => finding.id === "P008" && finding.severity === "warning"));
});

test("DPoP enabled on the portal application is an error with an automatic fix", () => {
  const snapshot = healthy();
  snapshot.application!.require_proof_of_possession = true;
  const finding = auditPortal(snapshot).find((item) => item.id === "P023");
  assert.equal(finding?.severity, "error");
  assert.ok(finding?.remedy?.apply, "the fix can be applied");
});

test("session transfer not enabled is a warning, and has no automatic fix", () => {
  const snapshot = healthy();
  snapshot.application!.session_transfer = { allowed_authentication_methods: [] };
  const finding = auditPortal(snapshot).find((item) => item.id === "P024");
  assert.equal(finding?.severity, "warning");
  assert.equal(finding?.remedy?.apply, undefined);
});

// One table row per check: a healthy snapshot, one change, and the finding that
// change must produce. Each row becomes its own named test.
const BREAKS: { id: string; name: string; breakIt: (snapshot: PortalSnapshot) => void }[] = [
  {
    id: "P002",
    name: "the home page missing from the sidebar is a warning",
    breakIt: (snapshot) => {
      snapshot.portal.navigation!.sidebar!.components = [];
    },
  },
  {
    id: "P003",
    name: "a legacy sidebar link is an error",
    breakIt: (snapshot) => {
      snapshot.portal.navigation!.sidebar!.components = [
        { type: "sidebar:component:auth0:internal_link", config: { label: "Profile", to: "profile" } },
      ];
    },
  },
  {
    id: "P006",
    name: "a page with no components is a warning",
    breakIt: (snapshot) => {
      snapshot.portal.pages!.standard!.push({ title: "Billing", slug: "billing", components: [] });
      snapshot.portal.navigation!.sidebar!.components!.push({
        type: "sidebar:component:auth0:internal_link:v2",
        config: { label: "Billing", page_slug: "billing" },
      });
    },
  },
  {
    id: "P009",
    name: "a portal with no application is an error",
    breakIt: (snapshot) => {
      snapshot.portal.client = undefined;
      snapshot.application = undefined;
    },
  },
  {
    id: "P011",
    name: "a Management API grant on the portal application is an error",
    breakIt: (snapshot) => {
      snapshot.grants.push({ id: "cgr_mgmt", audience: `https://${DOMAIN}/api/v2/`, scope: ["read:users"] });
    },
  },
  {
    id: "P012",
    name: "a missing callback URL is an error",
    breakIt: (snapshot) => {
      snapshot.application!.callbacks = [];
    },
  },
  {
    id: "P014",
    name: "a refresh token with no idle expiry is an error",
    breakIt: (snapshot) => {
      snapshot.application!.refresh_token!.infinite_idle_token_lifetime = true;
    },
  },
  {
    id: "P017",
    name: "an untrusted token endpoint IP header is a warning",
    breakIt: (snapshot) => {
      snapshot.application!.is_token_endpoint_ip_header_trusted = false;
    },
  },
  {
    id: "P019",
    name: "a missing client grant for a used API is an error",
    breakIt: (snapshot) => {
      snapshot.grants = [];
    },
  },
];

for (const row of BREAKS) {
  test(`${row.id}: ${row.name}`, () => {
    const snapshot = healthy();
    row.breakIt(snapshot);
    const finding = auditPortal(snapshot).find((item) => item.id === row.id);
    assert.ok(finding, `${row.id} should be reported`);
  });
}
