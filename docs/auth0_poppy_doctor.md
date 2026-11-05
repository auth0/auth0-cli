---
layout: default
parent: auth0 poppy
has_toc: false
---
# auth0 poppy doctor

(Experimental) Checks the whole discovery chain without changing anything: the files on your website, Auth0's metadata, the tenant settings and each registered agent. Each failure comes with the spec section and a FIX line telling you what to change. It understands sites on protocol 0.3 and 0.4, and runs the extra checks of 0.4 (D15 to D17) only on sites that declare it. The checks that only need the company domain (D1 to D7, and D15 to D17) work without a login, so they also work against other companies. The checks that read your tenant (D8 to D13: tenant settings, the web app, the MFA policy, and each agent's metadata, keys and grants, and D14, which compares the config file with the tenant) run when you are logged in to a tenant, or use the environment credentials of `AUTH0_CLI_AUTH_MODE=env`, and show as skipped otherwise. This command never starts a login by itself: an expired or missing session only skips those checks. They read the tenant with `read:tenant_settings`, `read:clients`, `read:mfa_policies`, `read:client_grants`, `read:agents`, `read:resource_servers` and `read:client_credentials`; a check whose scope is missing is skipped with the login command that adds it. The command exits with 1 when a check fails.

**Warning:** This command is experimental and is subject to change in future versions.

## Usage
```
auth0 poppy doctor [domain] [flags]
```

## Examples

```
  auth0 poppy doctor shop.example
  auth0 poppy doctor shop.example --config ./config/auth0-poppy.json
  auth0 poppy doctor shop.example --json-compact
```


## Flags

```
      --config string   Config file to check. Used only if it exists. (default "auth0-poppy.json")
      --json            Output in json format.
      --json-compact    Output in compact json format.
```


## Inherited Flags

```
      --agent-mode      Output JSON, disable prompts and colors. Auto-enabled for AI agents; set AUTH0_AGENT_MODE=false to disable.
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 poppy agents](auth0_poppy_agents.md) - Manage the agents your company allows
- [auth0 poppy config](auth0_poppy_config.md) - Rebuild the Poppy config file from the tenant
- [auth0 poppy doctor](auth0_poppy_doctor.md) - Check the Poppy discovery chain end to end
- [auth0 poppy setup](auth0_poppy_setup.md) - Turn on Poppy for the tenant, an API and a web app


