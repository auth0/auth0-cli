---
layout: default
parent: auth0 guardian factors duo settings
has_toc: false
---
# auth0 guardian factors duo settings set

Replace the Duo MFA factor settings. This overwrites all Duo settings, so the host, integration key and secret key are all required. To change a single field without clearing the others, use `auth0 guardian factors duo settings update` instead.

## Usage
```
auth0 guardian factors duo settings set [flags]
```

## Examples

```
  auth0 guardian factors duo settings set --ikey <ikey> --skey <skey> --host api-xxxx.duosecurity.com
```


## Flags

```
      --host string    Duo API hostname.
      --ikey string    Duo integration key.
      --json           Output in json format.
      --json-compact   Output in compact json format.
      --skey string    Duo secret key.
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

- [auth0 guardian factors duo settings set](auth0_guardian_factors_duo_settings_set.md) - Set the Duo settings
- [auth0 guardian factors duo settings show](auth0_guardian_factors_duo_settings_show.md) - Show the Duo settings
- [auth0 guardian factors duo settings update](auth0_guardian_factors_duo_settings_update.md) - Update the Duo settings


