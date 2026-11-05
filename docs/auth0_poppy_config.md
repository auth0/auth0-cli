---
layout: default
parent: auth0 poppy
has_toc: false
---
# auth0 poppy config

(Experimental) Rebuilds the config file from the tenant without changing the tenant, so CI and deploys can refresh it safely. It reads the tenant, adds the values Auth0 does not store, and writes the file. If the tenant is not set up for Poppy, it exits with an error naming the missing setting. The config file is for one website, so with several company domains pass `--company-domain` to pick the one this app serves. The file is a protocol 0.4 `poppy.json` document and nothing else, so your web SDK can serve it as `/.well-known/poppy.json`. A file written for 0.3 is upgraded, and members of the file that the CLI does not use are kept. The file holds no API identifier and no web app, so without `--api` the APIs are the Poppy APIs of the tenant that the `apis` entries of the existing file are at, and without `--app` the web app is the only app of the tenant that has the browser join on. When that does not settle on one, the command stops and asks for `--api` or `--app`.

**Warning:** This command is experimental and is subject to change in future versions.

## Usage
```
auth0 poppy config [flags]
```

## Examples

```
  auth0 poppy config
  auth0 poppy config --company-domain shop.example --out ./config/auth0-poppy.json
  auth0 poppy config --company-domain shop.example --api https://api.shop.example --app "Shop Web" --json
```


## Flags

```
      --api stringArray         A Poppy API, by id or identifier. Repeat for several. When registering an agent, the Poppy APIs to grant (default: every Poppy API of the tenant).
      --api-url stringArray     Where an API's OpenAPI document lives, as <identifier>=<openapi url>. Repeat for several. Asked if the config file has no entry for the API, with <origin>/openapi.json as the default when the identifier is a URL.
      --app string              Client ID or name of the web app used for the browser join.
      --company-domain string   The Company's website host, such as shop.example. With several company domains, picks the one this app serves.
      --force-config            Overwrite a config file that belongs to a different issuer.
      --hint stringArray        Hints for a scope, as name=changes:irreversible,personal_data:true. Repeat for several.
      --json                    Output in json format.
      --json-compact            Output in compact json format.
      --name string             The Company's name for the config file. When registering an agent, the agent's name.
      --out string              Path of the config file. (default "auth0-poppy.json")
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


