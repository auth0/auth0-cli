---
layout: default
parent: auth0 users roles
has_toc: false
---
# auth0 users roles remove

Remove existing roles from a user.

## Usage
```
auth0 users roles remove [flags]
```

## Examples

```
  auth0 users roles remove <user-id>
  auth0 users roles remove <user-id> --roles <role-id1,role-id2>
  auth0 users roles rm <user-id> -r "rol_1eKJp3jV04SiU04h,rol_2eKJp3jV04SiU04h" --json
  auth0 users roles rm <user-id> -r "rol_1eKJp3jV04SiU04h,rol_2eKJp3jV04SiU04h" --json-compact
```


## Flags

```
      --json            Output in json format.
      --json-compact    Output in compact json format.
  -r, --roles strings   Roles to assign to a user.
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


