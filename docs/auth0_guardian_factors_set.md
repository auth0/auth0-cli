---
layout: default
parent: auth0 guardian factors
has_toc: false
---
# auth0 guardian factors set

Enable or disable a single MFA factor.

## Usage
```
auth0 guardian factors set [flags]
```

## Examples

```
  auth0 guardian factors set sms --enabled
  auth0 guardian factors set email --enabled=false
  auth0 guardian factors set push-notification --enabled --json
```


## Flags

```
      --enabled        Whether the factor is enabled.
      --json           Output in json format.
      --json-compact   Output in compact json format.
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

- [auth0 guardian factors duo](auth0_guardian_factors_duo.md) - Manage the Duo multi-factor authentication factor
- [auth0 guardian factors list](auth0_guardian_factors_list.md) - List multi-factor authentication factors
- [auth0 guardian factors phone](auth0_guardian_factors_phone.md) - Manage the phone multi-factor authentication factor
- [auth0 guardian factors push](auth0_guardian_factors_push.md) - Manage the push-notification multi-factor authentication factor
- [auth0 guardian factors set](auth0_guardian_factors_set.md) - Enable or disable a multi-factor authentication factor
- [auth0 guardian factors sms](auth0_guardian_factors_sms.md) - Manage the SMS multi-factor authentication factor (legacy)


