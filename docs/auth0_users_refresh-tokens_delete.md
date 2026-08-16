---
layout: default
parent: auth0 users refresh-tokens
has_toc: false
---
# auth0 users refresh-tokens delete

Delete all refresh tokens for a user.

This deletes every refresh token the user has, not a single token. To delete one token by its id, use `auth0 refresh-tokens delete`.

To delete non-interactively, supply the user id and the `--force` flag to skip confirmation.

## Usage
```
auth0 users refresh-tokens delete [flags]
```

## Examples

```
  auth0 users refresh-tokens delete
  auth0 users refresh-tokens rm <user-id>
  auth0 users refresh-tokens delete <user-id> --force
```


## Flags

```
      --force   Skip confirmation.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 users refresh-tokens delete](auth0_users_refresh-tokens_delete.md) - Delete all of a user's refresh tokens
- [auth0 users refresh-tokens list](auth0_users_refresh-tokens_list.md) - List a user's refresh tokens


