// The only module that reads the runtime environment and decides how the script reaches the
// Management API. There are three ways it can be started:
//   - By the Auth0 CLI's plugin framework: a local proxy and its token are in the environment.
//   - By the Auth0 CLI itself, which passes its own path and the tenant in AUTH0_CLI_BIN and
//     AUTH0_CLI_TENANT_DOMAIN. Requests go through `auth0 api`.
//   - By hand, with nothing set: `auth0` on the PATH, and the tenant the CLI is logged in to.

import { ApiClient, createApiClient, FetchLike } from "./api.js";
import { UsageError } from "./args.js";
import { createCliApiClient, findActiveTenant, RunCommand, runCommand } from "./cli-api.js";

export type Transport = { kind: "proxy"; url: string; token: string } | { kind: "cli"; bin: string };

export interface PluginContext {
  tenantDomain: string;
  transport: Transport;
}

// The --tenant flag must name the tenant a proxy is bound to. The optional consequence says what
// the user can rely on, such as that nothing was written. When the script calls the CLI, the
// flag selects the tenant instead, so it always matches.
export function checkTenantFlag(requested: string | undefined, context: PluginContext, consequence = ""): void {
  if (requested && requested !== context.tenantDomain) {
    throw new UsageError(
      `--tenant ${requested} does not match the tenant the CLI is running against (${context.tenantDomain}).${consequence}`,
    );
  }
}

// The tenant is taken from the --tenant flag first, then AUTH0_CLI_TENANT_DOMAIN, then the CLI's
// active tenant.
export async function readContext(
  env: NodeJS.ProcessEnv,
  requestedTenant?: string,
  run: RunCommand = runCommand,
): Promise<PluginContext> {
  const proxyUrl = env.AUTH0_CLI_PROXY_URL;
  const proxyToken = env.AUTH0_CLI_PROXY_TOKEN;

  if (proxyUrl || proxyToken) {
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
        `The Auth0 CLI started this command with an incomplete environment. Missing: ${missing.join(", ")}`,
      );
    }
    return { tenantDomain, transport: { kind: "proxy", url: proxyUrl, token: proxyToken } };
  }

  const bin = env.AUTH0_CLI_BIN || "auth0";
  const tenantDomain = requestedTenant || env.AUTH0_CLI_TENANT_DOMAIN || (await findActiveTenant(bin, run));
  return { tenantDomain, transport: { kind: "cli", bin } };
}

// Builds the API client for the context, in one place for every command.
export function apiFor(context: PluginContext, options: { fetchImpl?: FetchLike; run?: RunCommand } = {}): ApiClient {
  const { transport } = context;
  if (transport.kind === "proxy") return createApiClient(transport.url, transport.token, options.fetchImpl);
  return createCliApiClient({ bin: transport.bin, tenant: context.tenantDomain, run: options.run });
}
