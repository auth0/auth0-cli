---
layout: default
parent: auth0 poppy
has_toc: false
---
# auth0 poppy setup

(Experimental) Turns on the Poppy settings for the tenant, an existing API and an existing web app, and writes the config file.

The API and the web app must already exist. Their scopes, MFA policy and Actions are left alone, except that setup adds the scope `poppy:browser-sign-in` to each API that lacks it.

The command shows a plan first and marks the tenant-wide changes, then applies it once you confirm. Use `--dry-run` to see the plan only, and `--force` to skip the question, which is required without a terminal.

In a terminal, it lets you pick the API and the web app, and asks for the values that cannot be listed, such as the OpenAPI URL of an API, the Application Login URI of the web app, and the description of each scope that has none. Without a terminal, it names every missing flag at once.

Running it again changes nothing when the tenant already matches, or continues where a failed run stopped. It finishes by running `auth0 poppy doctor`, unless `--skip-doctor` is set. A failed check does not fail setup, because your website may not serve the config file until you deploy it.

**Warning:** This command is experimental and is subject to change in future versions.

## Usage
```
auth0 poppy setup [flags]
```

## Examples

```
  auth0 poppy setup
  auth0 poppy setup --company-domain shop.example --api https://api.shop.example --app "Shop Web" --scope orders:read="See your orders and their status. Read only."
  auth0 poppy setup --company-domain shop.example --api https://api.shop.example --app "Shop Web" --dry-run
  auth0 poppy setup --company-domain shop.example --api https://api.shop.example --app "Shop Web" --force
  auth0 poppy setup --company-domain shop.example --api https://api.shop.example --app "Shop Web" --api-url https://api.shop.example=https://api.shop.example/openapi.json --login-uri https://shop.example/login --hint checkout=changes:irreversible,personal_data:true --force
```


## Flags

```
      --api stringArray         A Poppy API, by ID or identifier. Repeat for several.
      --api-url stringArray     Where an API's OpenAPI document lives, as <identifier>=<openapi url>. Repeat for several. Asked if the config file has no entry for the API, with <origin>/openapi.json as the default when the identifier is a URL.
      --app string              Client ID or name of the web app used for the browser join.
      --company-domain string   The Company's website host, such as shop.example. With several company domains, picks the one this app serves.
      --company-name string     The Company's name for the config file. Needed when the tenant has no friendly name and the config file has no name.
      --dry-run                 Print the plan and change nothing.
      --force                   Skip confirmation.
      --hint stringArray        Hints for a scope, as name=changes:irreversible,personal_data:true. Repeat for several.
      --json                    Output in json format.
      --json-compact            Output in compact json format.
      --login-uri string        The web app's Application Login URI (initiate_login_uri): the page on your site that starts a sign-in, where the browser join sends people. Asked if the app has none.
      --no-api                  Set up no API: agents use the website only. Sets auth.disabled in the config file and leaves the APIs alone. Cannot be used with --api, --api-url, --scope, --hint or --no-web. In a terminal, the command asks when neither --api nor this flag is given and the config file does not have auth.disabled.
      --no-web                  Set up no browser sign-in: agents use APIs only. Sets web.disabled in the config file and leaves the web app alone. Cannot be used with --app, --login-uri, --web-unscoped or --no-api. In a terminal, the command asks when neither --app nor this flag is given and the config file does not have web.disabled.
      --out string              Path of the config file. (default "poppy.json")
      --overwrite-config        Overwrite a config file that belongs to a different issuer.
      --scope stringArray       The description of a scope that has none, as name="description". With one API, also adds a scope the API lacks. Repeat for several. Existing descriptions are never edited.
      --skip-doctor             Do not run the doctor checks after setup.
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


