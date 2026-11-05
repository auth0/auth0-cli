import { PLUGIN_SCOPES } from "./scopes.js";

// What to do when the session lacks a scope. A user login asks for every scope the tool uses, not
// only the missing ones, because `auth0 login --scopes` drops the extra scopes of the last login.
// A client-credentials login keeps its grant, so only the missing scopes need adding there.
export function scopeFixes(missing: string[]): string[] {
  const login = [...new Set([...PLUGIN_SCOPES, ...missing])];
  return [
    `Signed in as a user: auth0 login --scopes ${login.join(",")}`,
    `Signed in with client credentials: add ${missing.join(", ")} to the application's Management API grant`,
  ];
}
