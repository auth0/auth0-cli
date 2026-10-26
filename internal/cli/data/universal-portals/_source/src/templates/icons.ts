// Lucide icon names for sidebar links. The API accepts any string of 1-50
// characters, so this map is the only place that decides what we ship.
export const SIDEBAR_ICONS = {
  profile: "user",
  security: "shield-check",
  legalPrivacy: "file-text",
  organization: "building-2",
  domains: "globe",
  preferences: "sliders-horizontal",
} as const;

export const ICON_NAME_PATTERN = /^[a-z0-9-]{1,50}$/;
