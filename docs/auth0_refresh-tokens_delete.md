---
layout: default
parent: auth0 refresh-tokens
has_toc: false
---
# auth0 refresh-tokens delete

Delete a refresh token.

To delete interactively, use `auth0 refresh-tokens delete` with no arguments.

To delete non-interactively, supply the token id and the `--force` flag to skip confirmation.

## Usage
```
auth0 refresh-tokens delete [flags]
```

## Examples

```
  auth0 refresh-tokens delete
  auth0 refresh-tokens rm
  auth0 refresh-tokens delete <token-id>
  auth0 refresh-tokens delete <token-id> --force
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

- [auth0 refresh-tokens delete](auth0_refresh-tokens_delete.md) - Delete a refresh token
- [auth0 refresh-tokens revoke](auth0_refresh-tokens_revoke.md) - Revoke refresh tokens
- [auth0 refresh-tokens show](auth0_refresh-tokens_show.md) - Show a refresh token
- [auth0 refresh-tokens update](auth0_refresh-tokens_update.md) - Update a refresh token


