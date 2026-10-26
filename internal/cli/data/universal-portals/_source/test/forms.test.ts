import { strict as assert } from "node:assert";
import { test } from "node:test";

import { buildPortalForms, FORM_NAMES, type FormKey } from "../src/forms.js";

const VAULT = "ac_test_vault";

function updateAction(document: Record<string, unknown>): Record<string, unknown> {
  const flows = document.flows as Record<string, { actions: Record<string, unknown>[] }>;
  return flows["#FLOW-1#"].actions[0];
}

test("builds the three portal forms with their beta names", () => {
  const forms = buildPortalForms(VAULT);
  assert.deepEqual(
    forms.map((form) => form.key).sort(),
    ["communicationPreferences", "personalInfo", "privacyConsent"],
  );
  for (const form of forms) {
    assert.equal(form.name, FORM_NAMES[form.key]);
  }
});

test("every form routes its user update through the vault connection", () => {
  for (const form of buildPortalForms(VAULT)) {
    const action = updateAction(form.document);
    assert.equal(action.action, "UPDATE_USER");
    assert.equal((action.params as { connection_id: string }).connection_id, "#CONN-1#");
    assert.deepEqual(form.document.connections, { "#CONN-1#": { id: VAULT } });
  }
});

test("each form writes its answers to the metadata it is meant to update", () => {
  const expected: Record<FormKey, string> = {
    personalInfo: "user_metadata",
    communicationPreferences: "app_metadata",
    privacyConsent: "app_metadata",
  };
  for (const form of buildPortalForms(VAULT)) {
    const changes = (updateAction(form.document).params as { changes: Record<string, unknown> }).changes;
    assert.ok(expected[form.key] in changes, `${form.key} should write ${expected[form.key]}`);
  }
});

test("every form has a start node and a step that links to its flow", () => {
  for (const form of buildPortalForms(VAULT)) {
    const nodes = (form.document.form as { nodes: { id: string; type: string; config?: { next_node?: string } }[] }).nodes;
    const step = nodes.find((node) => node.type === "STEP");
    const flow = nodes.find((node) => node.type === "FLOW");
    assert.ok(step && flow, `${form.key} is missing a step or flow node`);
    assert.equal(step.config?.next_node, flow.id);
  }
});
