---
layout: default
parent: auth0 poppy
has_toc: false
---
# auth0 poppy setup

(Experimental) Turns on the Poppy settings for the tenant, an existing API and an existing web app, and writes the config file. The API and the web app must already exist, and their scopes, MFA policy and Actions are left alone. The command shows a plan first and marks the tenant-wide changes, then applies it once you confirm; use `--dry-run` to see the plan only, and `--yes` to skip the question, which is required without a terminal. In a terminal it asks for any required value you did not pass, such as the OpenAPI URL of an API or the login route of the web app. Running it again changes nothing when the tenant already matches, or continues where a failed run stopped. It finishes by running `auth0 poppy doctor`, including the checks that read the tenant, unless `--skip-doctor` is set; a failed check does not fail setup, because your website may not serve the config file until you deploy it. With `--guest-api-access` the guest grant carries only the scopes you choose, with `--guest-scopes` or `--guest-all-scopes`; in a terminal setup asks, and it also asks whether to let signed-out guests use the API when no guest grant exists yet. Setup adds the scope `poppy:browser-sign-in` to each API when it is missing, with a default description unless you give one with `--scope`; every other scope offered to agents needs a description, which a terminal asks for and otherwise comes from `--scope <name>=<description>`.

**Warning:** This command is experimental and is subject to change in future versions.

## Usage
```
auth0 poppy setup [flags]
```

## Examples

```
  auth0 poppy setup --company-domain shop.example --api https://api.shop.example --app "Shop Web" --scope orders:read="See your orders and their status. Read only."
  auth0 poppy setup --company-domain shop.example --api https://api.shop.example --app "Shop Web" --dry-run
  auth0 poppy setup --company-domain shop.example --api https://api.shop.example --app "Shop Web" --guest-api-access --guest-scopes orders:read --yes
  auth0 poppy setup --company-domain shop.example --api https://api.shop.example --app "Shop Web" --api-url https://api.shop.example=https://api.shop.example/openapi.json --login-uri https://shop.example/login --hint checkout=changes:irreversible,personal_data:true --yes
```


## Flags

```
      --api stringArray         A Poppy API, by id or identifier. Repeat for several. When registering an agent, the Poppy APIs to grant (default: every Poppy API of the tenant).
      --api-url stringArray     Where an API's OpenAPI document lives, as <identifier>=<openapi url>. Repeat for several. Asked if the config file has no entry for the API, with <origin>/openapi.json as the default when the identifier is a URL.
      --app string              Client ID or name of the web app used for the browser join.
      --company-domain string   The Company's website host, such as shop.example. With several company domains, picks the one this app serves.
      --dry-run                 Print the plan and change nothing.
      --force-config            Overwrite a config file that belongs to a different issuer.
      --guest-all-scopes        The guest grant carries every scope of the API. Only affects the guest grant; use --scope (singular) to describe scopes. Needs --guest-api-access. Mutually exclusive with --guest-scopes.
      --guest-api-access        Also create the client grant that lets signed-out guests reach the API. Choose its scopes with --guest-scopes or --guest-all-scopes; in a terminal, setup asks.
      --guest-scopes strings    Comma-separated scopes the guest grant carries, which every API must define. Only affects the guest grant; use --scope (singular) to describe scopes. Needs --guest-api-access. Without a terminal, pass this or --guest-all-scopes. Mutually exclusive with --guest-all-scopes.
      --hint stringArray        Hints for a scope, as name=changes:irreversible,personal_data:true. Repeat for several.
      --json                    Output in json format.
      --json-compact            Output in compact json format.
      --login-uri string        The web app's login route, used for the browser join. Asked if the app has none.
      --name string             The Company's name for the config file. When registering an agent, the agent's name.
      --out string              Path of the config file. (default "auth0-poppy.json")
      --scope stringArray       The description of a scope that has none, as name="description". With one API, also adds a scope the API lacks. Repeat for several. Existing descriptions are never edited.
      --skip-doctor             Do not run the doctor checks after setup.
      --yes                     Skip confirmation.
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


