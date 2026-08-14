---
layout: default
parent: auth0 users sessions
has_toc: false
---
# auth0 users sessions list

List the active sessions of an existing user.

## Usage
```
auth0 users sessions list [flags]
```

## Examples

```
  auth0 users sessions list
  auth0 users sessions list <user-id>
  auth0 users sessions list <user-id> --number 100
  auth0 users sessions list <user-id> -n 100 --json
  auth0 users sessions list <user-id> --csv
```


## Flags

```
      --csv            Output in csv format.
      --json           Output in json format.
      --json-compact   Output in compact json format.
  -n, --number int     Number of user sessions to retrieve. Minimum 1, maximum 1000. (default 100)
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 users sessions delete](auth0_users_sessions_delete.md) - Delete all of a user's sessions
- [auth0 users sessions list](auth0_users_sessions_list.md) - List a user's sessions


