import type { ApiClient } from "../api.js";
import type { Prompter } from "../prompt.js";
import { PluginError } from "../plugin-error.js";
import type { Ui } from "../ui.js";

// The types and labels the Business setup steps share, so each step module can
// import them without importing the others.

export class BusinessAccessError extends PluginError {
  // Always set here, so the base class's optional type is narrowed.
  declare readonly fixes: string[];

  constructor(message: string, fixes: string[]) {
    super(message, "business_access", fixes);
  }
}

export interface BusinessAccess {
  userId: string;
  organizationId: string;
  organizationName: string;
  hasOrganizationRole: boolean;
}

export interface UserRecord {
  user_id: string;
  email?: string;
  name?: string;
}

export interface OrganizationRecord {
  id: string;
  name: string;
  display_name?: string;
}

export interface BusinessInput {
  // Email or user ID. Undefined when the user has to be asked for.
  userQuery?: string;
  organization?: string;
}

export interface BusinessDeps {
  api: ApiClient;
  ui: Ui;
  prompt: Prompter;
  interactive: boolean;
  tenantDomain: string;
}

export function userLabel(user: UserRecord): string {
  return user.email ?? user.user_id;
}
