// Form documents for the Forms that Auth0 Portals pages embed. Each form
// writes to the signed-in user through the forms vault connection.

export type FormKey = "personalInfo" | "privacyConsent" | "communicationPreferences";

export interface FormDefinition {
  key: FormKey;
  name: string;
  document: Record<string, unknown>;
}

export const FORM_NAMES: Record<FormKey, string> = {
  personalInfo: "Profile update (Universal Portals)",
  privacyConsent: "Privacy settings (Universal Portals)",
  communicationPreferences: "Marketing communication preferences (Universal Portals)",
};

export function buildPortalForms(vaultConnectionId: string): FormDefinition[] {
  return [
    { key: "personalInfo", name: FORM_NAMES.personalInfo, document: personalInfoDocument(vaultConnectionId) },
    {
      key: "communicationPreferences",
      name: FORM_NAMES.communicationPreferences,
      document: communicationPreferencesDocument(vaultConnectionId),
    },
    { key: "privacyConsent", name: FORM_NAMES.privacyConsent, document: privacyDocument(vaultConnectionId) },
  ];
}

function field(id: string, label: string, type: string, extra: Record<string, unknown> = {}): Record<string, unknown> {
  return { id, category: "FIELD", type, label, required: false, sensitive: false, ...extra };
}

function nextButton(id: string, text: string): Record<string, unknown> {
  return { id, category: "BLOCK", type: "NEXT_BUTTON", config: { text } };
}

function formDocument(
  name: string,
  step: { id: string; coordinates: Record<string, number>; components: Record<string, unknown>[]; flowNodeId: string },
  start: Record<string, unknown>,
  ending: Record<string, unknown>,
  flowName: string,
  actions: Record<string, unknown>[],
  connectionId: string,
): Record<string, unknown> {
  return {
    version: "4.0.0",
    form: {
      name,
      languages: { primary: "en" },
      nodes: [
        {
          id: step.id,
          type: "STEP",
          coordinates: step.coordinates,
          config: { components: step.components, next_node: step.flowNodeId },
        },
        {
          id: step.flowNodeId,
          type: "FLOW",
          coordinates: { x: 988, y: 9 },
          config: { flow_id: "#FLOW-1#", next_node: "$ending" },
        },
      ],
      start,
      ending,
    },
    flows: { "#FLOW-1#": { name: flowName, actions } },
    connections: { "#CONN-1#": { id: connectionId } },
  };
}

function updateUserAction(id: string, changes: Record<string, unknown>): Record<string, unknown> {
  return {
    id,
    type: "AUTH0",
    action: "UPDATE_USER",
    allow_failure: false,
    mask_output: false,
    params: {
      connection_id: "#CONN-1#",
      user_id: "{{context.user.user_id}}",
      changes,
    },
  };
}

function personalInfoDocument(connectionId: string): Record<string, unknown> {
  return formDocument(
    FORM_NAMES.personalInfo,
    {
      id: "step_BL3V",
      coordinates: { x: 346, y: -221 },
      flowNodeId: "flow_DcHK",
      components: [
        field("full_name", "Full name", "TEXT", { config: { multiline: false } }),
        field("job_title", "Job title", "TEXT", { config: { multiline: false } }),
        field("mobile_number", "Mobile number", "TEL", { config: { country_picker: true } }),
        field("date_of_birth", "Date of birth", "DATE", { config: { format: "DATE" } }),
        field("linkedin", "LinkedIn", "URL"),
        nextButton("next_button_UmEY", "Update"),
      ],
    },
    { next_node: "step_BL3V", coordinates: { x: 0, y: 0 } },
    { resume_flow: true, coordinates: { x: 1367, y: -7 } },
    "Update profile",
    [
      updateUserAction("update_user_v4C0", {
        user_metadata: {
          linkedin: "{{fields.linkedin}}",
          full_name: "{{fields.full_name}}",
          job_title: "{{fields.job_title}}",
          date_of_birth: "{{fields.date_of_birth}}",
          mobile_number: "{{fields.mobile_number.international_number}}",
        },
      }),
    ],
    connectionId,
  );
}

function communicationPreferencesDocument(connectionId: string): Record<string, unknown> {
  return formDocument(
    FORM_NAMES.communicationPreferences,
    {
      id: "step_4Td2",
      coordinates: { x: 217, y: -218 },
      flowNodeId: "flow_etvH",
      components: [
        field("communication_preferences", "Communication preferences", "CHOICE", {
          config: {
            multiple: true,
            options: [
              { label: "Product Announcements", value: "Product Announcements" },
              { label: "Featured Content", value: "Featured Content" },
              { label: "Digest News", value: "Digest News" },
              { label: "Events", value: "Events" },
            ],
          },
        }),
        nextButton("next_button_US5N", "Update"),
      ],
    },
    { next_node: "step_4Td2", coordinates: { x: -25, y: -98 } },
    { resume_flow: true, coordinates: { x: 1194, y: -59 } },
    "Update preferences",
    [
      updateUserAction("update_user_HitP", {
        app_metadata: { communication_preferences: "{{fields.communication_preferences}}" },
      }),
    ],
    connectionId,
  );
}

function privacyDocument(connectionId: string): Record<string, unknown> {
  const consentField = (id: string, label: string, hint: string): Record<string, unknown> => ({
    id,
    category: "FIELD",
    type: "BOOLEAN",
    label,
    hint,
    required: true,
    sensitive: false,
    config: { default_value: false },
  });
  const learnMore = (text: string): string =>
    `<p>${text} <a href="https://auth0.com" target="_blank">Learn more.</a></p>`;

  return formDocument(
    FORM_NAMES.privacyConsent,
    {
      id: "step_y0Ri",
      coordinates: { x: 500, y: 0 },
      flowNodeId: "flow_AB1r",
      components: [
        consentField("data_sharing", "Data sharing", learnMore("Allow app usage data to improve features.")),
        consentField("profile_visibility", "Profile visibility", learnMore("Show my profile in search results.")),
        consentField(
          "location_tracking",
          "Location tracking",
          learnMore("Allow location tracking for personalized recommendations."),
        ),
        consentField("ad_personalization", "Ad personalization", learnMore("Use my data to personalize ads.")),
        nextButton("next_button_aB8e", "Update"),
      ],
    },
    { next_node: "step_y0Ri", coordinates: { x: 202, y: 247 } },
    { resume_flow: true, coordinates: { x: 1445, y: 247 } },
    "Update privacy settings",
    [
      updateUserAction("update_user_uOZW", {
        app_metadata: {
          data_sharing: "{{fields.data_sharing}}",
          location_tracking: "{{fields.location_tracking}}",
          ad_personalization: "{{fields.ad_personalization}}",
          profile_visibility: "{{fields.profile_visibility}}",
        },
      }),
    ],
    connectionId,
  );
}
