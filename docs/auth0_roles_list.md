---
layout: default
parent: auth0 roles
has_toc: false
---
# auth0 roles list

List your existing roles. To create one, run: `auth0 roles create`.

## Usage
```
auth0 roles list [flags]
```

## Examples

```
  auth0 roles list
  auth0 roles ls
  auth0 roles ls --number 100
  auth0 roles ls -n 100 --json
  auth0 roles ls --csv
```


## Flags

```
      --csv          Output in csv format.
      --json         Output in json format.
  -n, --number int   Number of roles to retrieve. Minimum 1, maximum 1000. (default 100)
```


## Inherited Flags

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


