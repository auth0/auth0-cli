---
layout: default
---
# auth0 roles permissions list

List existing permissions defined in a role. To add a permission, run: `auth0 roles permissions add`.

## Usage
```
auth0 roles permissions list [flags]
```

## Examples

```
  auth0 roles permissions list
  auth0 roles permissions list <role-id>
  auth0 roles permissions ls <role-id> --json
```


## Flags

```
      --json   Output in json format.
```


## InheritedFlags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 roles permissions add](auth0_roles_permissions_add.md) - Add a permission to a role
- [auth0 roles permissions list](auth0_roles_permissions_list.md) - List permissions defined within a role
- [auth0 roles permissions remove](auth0_roles_permissions_remove.md) - Remove a permission from a role


