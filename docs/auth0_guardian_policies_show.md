---
layout: default
parent: auth0 guardian policies
has_toc: false
---
# auth0 guardian policies show

Display the tenant-wide multi-factor authentication (MFA) policies.

## Usage
```
auth0 guardian policies show [flags]
```

## Examples

```
  auth0 guardian policies show
  auth0 guardian policies show --json
```


## Flags

```
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

- [auth0 guardian policies set](auth0_guardian_policies_set.md) - Set the multi-factor authentication policy
- [auth0 guardian policies show](auth0_guardian_policies_show.md) - Show the multi-factor authentication policies


