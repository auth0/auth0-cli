---
layout: default
parent: auth0 roles
has_toc: false
---
# auth0 roles delete

Delete a role.

To delete interactively, use `auth0 roles delete`.

To delete non-interactively, supply the role id and the `--force` flag to skip confirmation.

## Usage
```
auth0 roles delete [flags]
```

## Examples

```
  auth0 roles delete
  auth0 roles rm
  auth0 roles delete <role-id>
  auth0 roles delete <role-id> --force
```


## Flags

```
      --force   Skip confirmation.
```


## InheritedFlags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 roles create](auth0_roles_create.md) - Create a new role
- [auth0 roles delete](auth0_roles_delete.md) - Delete a role
- [auth0 roles list](auth0_roles_list.md) - List your roles
- [auth0 roles permissions](auth0_roles_permissions.md) - Manage permissions within the role resource
- [auth0 roles show](auth0_roles_show.md) - Show a role
- [auth0 roles update](auth0_roles_update.md) - Update a role


