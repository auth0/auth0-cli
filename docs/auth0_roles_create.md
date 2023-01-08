---
layout: default
---
# auth0 roles create

Create a new role.

To create interactively, use `auth0 roles create` with no arguments.

To create non-interactively, supply the role name and description through the flags.

## Usage
```
auth0 roles create [flags]
```

## Examples

```
  auth0 roles create
  auth0 roles create --name myrole --description "awesome role"
  auth0 roles create -n myrole -d "awesome role --json"
```


## Flags

```
  -d, --description string   Description of the role.
      --json                 Output in json format.
  -n, --name string          Name of the role.
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


