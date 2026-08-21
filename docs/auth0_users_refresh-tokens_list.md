---
layout: default
parent: auth0 users refresh-tokens
has_toc: false
---
# auth0 users refresh-tokens list

List the refresh tokens of an existing user.

## Usage
```
auth0 users refresh-tokens list [flags]
```

## Examples

```
  auth0 users refresh-tokens list
  auth0 users refresh-tokens list <user-id>
  auth0 users refresh-tokens list <user-id> --number 100
  auth0 users refresh-tokens list <user-id> -n 100 --json
  auth0 users refresh-tokens list <user-id> --csv
```


## Flags

```
      --csv            Output in csv format.
      --json           Output in json format.
      --json-compact   Output in compact json format.
  -n, --number int     Number of user refresh tokens to retrieve. Minimum 1, maximum 1000. (default 100)
```


## Inherited Flags

```
      --agent-mode      Output JSON, disable prompts and colors. Auto-enabled for AI agents; set AUTH0_AGENT_MODE=false to disable.
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 users refresh-tokens delete](auth0_users_refresh-tokens_delete.md) - Delete all of a user's refresh tokens
- [auth0 users refresh-tokens list](auth0_users_refresh-tokens_list.md) - List a user's refresh tokens


