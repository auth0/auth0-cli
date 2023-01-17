---
layout: default
---
# auth0 users roles assign

Assign existing roles to a user.

## Usage
```
auth0 users roles assign [flags]
```

## Examples

```
  auth0 users roles assign <user-id>
  auth0 users roles add <user-id> --roles <role-id1,role-id2>
  auth0 users roles add <user-id> -r "rol_1eKJp3jV04SiU04h,rol_2eKJp3jV04SiU04h" --json
```


## Flags

```
      --json            Output in json format.
  -r, --roles strings   Roles to assign to a user.
```


## InheritedFlags

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


