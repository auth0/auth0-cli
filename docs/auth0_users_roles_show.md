---
layout: default
parent: auth0 users roles
has_toc: false
---
# auth0 users roles show

Display information about an existing user's assigned roles.

## Usage
```
auth0 users roles show [flags]
```

## Examples

```
  auth0 users roles show
  auth0 users roles show <user-id>
  auth0 users roles show <user-id> --number 100
  auth0 users roles show <user-id> -n 100 --json
```


## Flags

```
      --json         Output in json format.
  -n, --number int   Number of user roles to retrieve. Minimum 1, maximum 1000. (default 50)
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 users roles assign](auth0_users_roles_assign.md) - Assign roles to a user
- [auth0 users roles remove](auth0_users_roles_remove.md) - Remove roles from a user
- [auth0 users roles show](auth0_users_roles_show.md) - Show a user's roles


