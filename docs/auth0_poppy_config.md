---
layout: default
parent: auth0 poppy
has_toc: false
---
# auth0 poppy config

(Experimental) Rebuilds the config file from the tenant without changing the tenant, so CI and deploys can refresh it safely.

It reads the tenant, adds the values Auth0 does not store, and writes a protocol 1.0 `poppy.json` document, which your web SDK can serve as `/.well-known/poppy.json`. A file written for 0.3 or 0.4 is upgraded, and members of the file that the CLI does not use are kept. If the tenant is not set up for Poppy, it exits with an error naming the missing setting.

The file is for one website, so with several company domains pass `--company-domain` to pick the one this app serves. Without `--api`, the APIs are the Poppy APIs that the existing file points to, or the only Poppy API of the tenant. Without `--app`, the web app is the only app that has the browser join on. When there are several, a terminal lets you pick, and otherwise the command stops and asks for `--api` or `--app`.

In a terminal, it also asks whether signing in to the browser gives an agent full access to the user's account on your website (`web.unscoped`), unless you pass `--web-unscoped` or the file already has a value.

**Warning:** This command is experimental and is subject to change in future versions.

## Usage
```
auth0 poppy config [flags]
```

## Examples

```
  auth0 poppy config
  auth0 poppy config --company-domain shop.example --out ./config/poppy.json
  auth0 poppy config --company-domain shop.example --api https://api.shop.example --app "Shop Web" --json
```


## Flags

```
      --api stringArray         A Poppy API, by ID or identifier. Repeat for several.
      --api-url stringArray     Where an API's OpenAPI document lives, as <identifier>=<openapi url>. Repeat for several. Asked if the config file has no entry for the API, with <origin>/openapi.json as the default when the identifier is a URL.
      --app string              Client ID or name of the web app used for the browser join.
      --company-domain string   The Company's website host, such as shop.example. With several company domains, picks the one this app serves.
      --company-name string     The Company's name for the config file. Needed when the tenant has no friendly name and the config file has no name.
      --hint stringArray        Hints for a scope, as name=changes:irreversible,personal_data:true. Repeat for several.
      --json                    Output in json format.
      --json-compact            Output in compact json format.
      --no-api                  Leave the APIs out: agents use the website only. Sets auth.disabled in the config file. Cannot be used with --api, --api-url, --hint or --no-web. In a terminal, the command asks when neither --api nor this flag is given, the config file does not have auth.disabled, and the tenant has a Poppy API.
      --no-web                  Leave the browser sign-in out: agents use APIs only. Sets web.disabled in the config file. Cannot be used with --app, --web-unscoped or --no-api. In a terminal, the command asks when neither --app nor this flag is given, the config file does not have web.disabled, and an app has the browser join on.
      --out string              Path of the config file. (default "poppy.json")
      --overwrite-config        Overwrite a config file that belongs to a different issuer.
      --web-unscoped            Set web.unscoped in the config file: true when your website treats poppy:browser-sign-in as the account's full access on the website. auth0 poppy doctor then reminds you to check the description of poppy:browser-sign-in. Asked in a terminal when the config file has no value.
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


