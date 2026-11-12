---
layout: default
parent: auth0 poppy
has_toc: false
---
# auth0 poppy doctor

(Experimental) Checks the whole discovery chain without changing anything, from the files on your website to the settings of your tenant. A failure names the check and the section of the protocol, and comes with a FIX line telling you what to change.

The checks cover five things: the Poppy files on your website, Auth0's issuer metadata, the settings of your tenant, each registered agent, and your config file. They understand sites on protocol 0.3, 0.4 and 1.x, and run the extra checks of 0.4 and later only on sites that declare one of them.

The checks of the website and the issuer need no login, so they also work against other companies. The checks of the tenant, the agents and the config file run when you are logged in to a tenant, or use the environment credentials of `AUTH0_CLI_AUTH_MODE=env`, and show as skipped otherwise. This command never starts a login by itself: an expired login that needs a person only skips them, and an expired client-credentials login is refreshed. A check whose scope is missing is skipped with the login command that adds it. When the tenant does not list the domain as a company domain, its checks are skipped too, because they would read the wrong tenant.

The command exits with 1 when a check fails, and with `--strict` also when a check could not run. The JSON report holds every check, with a stable code, and says why a check was skipped.

**Warning:** This command is experimental and is subject to change in future versions.

## Usage
```
auth0 poppy doctor [domain] [flags]
```

## Examples

```
  auth0 poppy doctor shop.example
  auth0 poppy doctor
  auth0 poppy doctor shop.example --config ./config/poppy.json
  auth0 poppy doctor shop.example --strict
  auth0 poppy doctor shop.example --json-compact
```


## Flags

```
      --config string   Config file to check. Used only if it exists. Without this flag, and without the default file, the poppy.json your website serves is checked instead. (default "poppy.json")
      --json            Output in json format.
      --json-compact    Output in compact json format.
      --strict          Exit with 1 when a check could not run, as well as when a check fails.
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

- [auth0 poppy config](auth0_poppy_config.md) - Rebuild the Poppy config file from the tenant
- [auth0 poppy doctor](auth0_poppy_doctor.md) - Check the Poppy discovery chain end to end
- [auth0 poppy setup](auth0_poppy_setup.md) - Turn on Poppy for the tenant, an API and a web app


