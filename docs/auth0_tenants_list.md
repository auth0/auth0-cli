---
layout: default
parent: auth0 tenants
has_toc: false
---
# auth0 tenants list

List your tenants.

## Usage
```
auth0 tenants list [flags]
```

## Examples

```
  auth0 tenants list
  auth0 tenants ls
  auth0 tenants ls --json
  auth0 tenants ls --json-compact
  auth0 tenants ls --csv
```


## Flags

```
      --csv            Output in csv format.
      --json           Output in json format.
      --json-compact   Output in compact json format.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 tenants list](auth0_tenants_list.md) - List your tenants
- [auth0 tenants open](auth0_tenants_open.md) - Open the settings page of the tenant
- [auth0 tenants use](auth0_tenants_use.md) - Set the active tenant


