---
layout: default
parent: auth0 tenant-settings update
has_toc: false
---
# auth0 tenant-settings update set

Enable selected tenant setting flags.

To enable interactively, use `auth0 tenant-settings update set` with no arguments.

To enable non-interactively, supply the flags.

## Usage
```
auth0 tenant-settings update set [flags]
```

## Examples

```
auth0 tenant-settings update set
auth0 tenant-settings update set <setting1> <setting2> <setting3>
auth0 tenant-settings update set flags.enable_client_connections mtls.enable_endpoint_aliases pushed_authorization_requests_supported
auth0 tenant-settings update set flags.enable_sso --json
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

- [auth0 tenant-settings update set](auth0_tenant-settings_update_set.md) - Enable tenant setting flags
- [auth0 tenant-settings update unset](auth0_tenant-settings_update_unset.md) - Disable tenant setting flags


