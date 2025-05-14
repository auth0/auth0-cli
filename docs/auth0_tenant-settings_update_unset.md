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
auth0 tenant-settings update unset <setting1> <setting2> <setting3>
auth0 tenant-settings update unset customize_mfa_in_postlogin_action flags.enable_pipeline2
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


