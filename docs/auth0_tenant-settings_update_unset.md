---
layout: default
parent: auth0 tenant-settings update
has_toc: false
---
# auth0 tenant-settings update unset

Disable selected tenant setting flags.

To disable interactively, use `auth0 tenant-settings update unset` with no arguments.

To disable non-interactively, supply the flags.

## Usage
```
auth0 tenant-settings update unset [flags]
```

## Examples

```
auth0 tenant-settings update unset
auth0 tenant-settings update unset <flag1> <flag2> <flag3>
auth0 tenant-settings update unset enable_client_connections enable_apis_section enable_pipeline2
```




## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 tenant-settings update set](auth0_tenant-settings_update_set.md) - Enable tenant setting flags
- [auth0 tenant-settings update unset](auth0_tenant-settings_update_unset.md) - Disable tenant setting flags


