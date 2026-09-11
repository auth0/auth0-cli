---
layout: default
parent: auth0 guardian policies
has_toc: false
---
# auth0 guardian policies set

Set the tenant-wide multi-factor authentication (MFA) policy.

The policies are mutually exclusive, so this sets a single policy and replaces the existing one. Pass `--policy none` or `--none` (or select none interactively) to clear the policy.

## Usage
```
auth0 guardian policies set [flags]
```

## Examples

```
  auth0 guardian policies set
  auth0 guardian policies set --policy all-applications
  auth0 guardian policies set --policy confidence-score
  auth0 guardian policies set --none
  auth0 guardian policies set --policy all-applications --json
```


## Flags

```
      --json            Output in json format.
      --json-compact    Output in compact json format.
      --none            Clear all MFA policies.
  -p, --policy string   MFA policy to enable. Supported values: all-applications, confidence-score. The policies are mutually exclusive; pass none (or --none) to clear all policies.
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


