---
layout: default
---
# auth0 roles permissions add

Add an existing permission defined in one of your APIs. To add a permission, run:

`auth0 roles permissions add <role-id> -p <permission-name>`

```
auth0 roles permissions add [flags]
```


## Flags

```
  -a, --api-id string         API Identifier.
  -p, --permissions strings   Permissions.
```


## InheritedFlags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```

## Examples

```
  auth0 roles permissions add
  auth0 roles permissions add <role-id> -p <permission-name>
```


## Related Commands

- [auth0 roles permissions add](auth0_roles_permissions_add.md) - Add a permission to a role
- [auth0 roles permissions list](auth0_roles_permissions_list.md) - List permissions defined within a role
- [auth0 roles permissions remove](auth0_roles_permissions_remove.md) - Remove a permission from a role


