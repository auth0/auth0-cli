---
layout: default
parent: auth0 users
has_toc: false
---
# auth0 users search-by-email

Search for users. To create one, run: `auth0 users create`.

## Usage
```
auth0 users search-by-email [flags]
```

## Examples

```
  auth0 users search-by-email
  auth0 users search-by-email <user-email>,
  auth0 users search-by-email <user-email> -p
```


## Flags

```
      --csv            Output in csv format.
      --json           Output in json format.
      --json-compact   Output in compact json format.
  -p, --picker         Allows to toggle from list of users and view a user in detail
```


## Inherited Flags

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
- [auth0 users search-by-email](auth0_users_search-by-email.md) - Search for users
- [auth0 users show](auth0_users_show.md) - Show an existing user
- [auth0 users update](auth0_users_update.md) - Update a user


