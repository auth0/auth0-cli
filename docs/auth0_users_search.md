---
layout: default
---
# auth0 users search

Search for users. To create one, run: `auth0 users create`.

## Usage
```
auth0 users search [flags]
```

## Examples

```
  auth0 users search
  auth0 users search --query user_id:"<user-id>"
  auth0 users search --query name:"Bob" --sort "name:1"
  auth0 users search -q name:"Bob" -s "name:1" --number 200
  auth0 users search -q name:"Bob" -s "name:1" -n 200 --json
```


## Flags

```
      --json           Output in json format.
  -n, --number int     Number of users, that match the search criteria, to retrieve. Minimum 1, maximum 1000. If limit is hit, refine the search query. (default 50)
  -q, --query string   Query in Lucene query syntax. See https://auth0.com/docs/users/user-search/user-search-query-syntax for more details.
  -s, --sort string    Field to sort by. Use 'field:order' where 'order' is '1' for ascending and '-1' for descending. e.g. 'created_at:1'.
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
- [auth0 users search](auth0_users_search.md) - Search for users
- [auth0 users show](auth0_users_show.md) - Show an existing user
- [auth0 users unblock](auth0_users_unblock.md) - Remove brute-force protection blocks for a given user
- [auth0 users update](auth0_users_update.md) - Update a user


