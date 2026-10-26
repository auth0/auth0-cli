// The only module that reads the plugin runtime environment. The CLI sets
// these variables when it starts the plugin.

import { UsageError } from "./args.js";

export interface PluginContext {
  proxyUrl: string;
  proxyToken: string;
  tenantDomain: string;
}

// The --tenant flag must name the tenant the CLI is running against. The optional
// consequence says what the user can rely on, such as that nothing was written.
export function checkTenantFlag(requested: string | undefined, context: PluginContext, consequence = ""): void {
  if (requested && requested !== context.tenantDomain) {
    throw new UsageError(
      `--tenant ${requested} does not match the tenant the CLI is running against (${context.tenantDomain}).${consequence}`,
    );
  }
}

export function readContext(env: NodeJS.ProcessEnv): PluginContext {
  const proxyUrl = env.AUTH0_CLI_PROXY_URL;
  const proxyToken = env.AUTH0_CLI_PROXY_TOKEN;
  const tenantDomain = env.AUTH0_CLI_TENANT_DOMAIN;

  const missing = [
    ["AUTH0_CLI_PROXY_URL", proxyUrl],
    ["AUTH0_CLI_PROXY_TOKEN", proxyToken],
    ["AUTH0_CLI_TENANT_DOMAIN", tenantDomain],
  ]
    .filter(([, value]) => !value)
    .map(([name]) => name);

  if (missing.length > 0 || !proxyUrl || !proxyToken || !tenantDomain) {
    throw new Error(
      `Run this plugin through the Auth0 CLI (auth0 universal-portals). Missing: ${missing.join(", ")}`,
    );
  }

  return { proxyUrl, proxyToken, tenantDomain };
}
