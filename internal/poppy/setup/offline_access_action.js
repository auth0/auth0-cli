/**
 * Poppy: always issue refresh tokens for Poppy agents.
 *
 * Installed by `auth0 poppy setup --always-issue-refresh-tokens`. The Poppy
 * protocol expects every sign-in to return a refresh token, but Auth0 returns
 * one only when the agent asks for the offline_access scope. When an agent signs
 * in to one of your Poppy APIs, this Action adds offline_access to the scopes of
 * the transaction. Auth0 still checks the scope against the API's settings
 * (allow_offline_access) and drops it when it is not allowed.
 *
 * Every other login is left alone. This Action runs last in the post-login
 * trigger so that it sees the scopes the other Actions have set.
 *
 * The list of Poppy APIs is written by `auth0 poppy setup`. Setting up another API
 * adds it to the list.
 *
 * To remove it, unbind and delete this Action in the dashboard.
 *
 * @param {Event} event - Details about the user and the context in which they are logging in.
 * @param {PostLoginAPI} api - Interface whose methods can be used to change the behavior of the login.
 */
const POPPY_APIS = __POPPY_APIS__;

exports.onExecutePostLogin = async (event, api) => {
  // An agent is acting when the client is linked to an agent record. `auth0 agents
  // register` creates that link for every Poppy agent.
  if (!event.agent || !event.agent.agent_id) {
    return;
  }

  // Only a sign-in to a Poppy API needs a refresh token.
  if (!POPPY_APIS.includes(event.resource_server?.identifier)) {
    return;
  }

  const targetScopes = event.transaction?.target_scopes ?? [];
  if (targetScopes.includes('offline_access')) {
    return;
  }

  api.transaction.addTargetScope('offline_access');
};
