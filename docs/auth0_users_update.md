---
layout: default
parent: auth0 users
has_toc: false
---
# auth0 users update

Update a user.

To update interactively, use `auth0 users update` with no arguments.

To update non-interactively, supply the user id and other information through the available flags.

## Usage
```
auth0 users update [flags]
```

## Examples

```
  auth0 users update 
  auth0 users update <user-id> 
  auth0 users update <user-id> --name "John Doe"
  auth0 users update <user-id> --name "John Doe" --email john.doe@example.com
```


## Flags

```
  -c, --connection string   Name of the database connection this user should be created in.
  -e, --email string        The user's email.
      --json                Output in json format.
  -n, --name string         The user's full name.
  -p, --password string     Initial password for this user (mandatory for non-SMS connections).
```


## InheritedFlags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 users blocks](auth0_users_blocks.md) - Manage brute-force protection user blocks
- [auth0 users create](auth0_users_create.md) - Create a new user
- [auth0 users delete](auth0_users_delete.md) - Delete a user
- [auth0 users import](auth0_users_import.md) - Import users from schema
- [auth0 users open](auth0_users_open.md) - Open the user's settings page
- [auth0 users roles](auth0_users_roles.md) - Manage a user's roles
- [auth0 users search](auth0_users_search.md) - Search for users
- [auth0 users show](auth0_users_show.md) - Show an existing user
- [auth0 users unblock](auth0_users_unblock.md) - Remove brute-force protection blocks for a given user
- [auth0 users update](auth0_users_update.md) - Update a user


