---
layout: default
parent: auth0 roles permissions
has_toc: false
---
# auth0 roles permissions remove

Remove an existing permission defined in one of your APIs.

## Usage
```
auth0 roles permissions remove [flags]
```

## Examples

```
  auth0 roles permissions remove
  auth0 roles permissions rm <role-id> --api-id <api-id>
  auth0 roles permissions rm <role-id> --api-id <api-id> --permissions <permission-name>
  auth0 roles permissions rm <role-id> -a <api-id> -p <permission-name>
  auth0 roles permissions rm <role-id> --resource-server-identifier <api-id> --permissions <permission-name>
```


## Flags

```
  -a, --api-id string                       API Identifier.
  -p, --permissions strings                 Permissions.
      --resource-server-identifier string   API Identifier.
```


## Inherited Flags

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


