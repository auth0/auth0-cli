// What to do when the session lacks a scope. The advice depends on how the Auth0 CLI is signed in:
// a user login can ask for more scopes, while a client-credentials login (machine to machine) has
// only the scopes granted to its application, so they must be added there.
export function scopeFixes(scopes: string[]): string[] {
  return [
    `Signed in as a user: auth0 login --scopes ${scopes.join(",")}`,
    `Signed in with client credentials: add ${scopes.join(", ")} to the application's Management API grant`,
  ];
}
