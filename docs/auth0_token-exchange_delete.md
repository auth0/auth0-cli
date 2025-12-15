---
layout: default
parent: auth0 token-exchange
has_toc: false
---
# auth0 token-exchange delete

Delete a token exchange profile.

To delete interactively, use `auth0 token-exchange delete` with no arguments.

To delete non-interactively, supply the profile id and the `--force` flag to skip confirmation.

## Usage
```
auth0 token-exchange delete [flags]
```

## Examples

```
  auth0 token-exchange delete
  auth0 token-exchange rm
  auth0 token-exchange delete <profile-id>
  auth0 token-exchange delete <profile-id> --force
  auth0 token-exchange delete <profile-id> <profile-id2> <profile-idn>
  auth0 token-exchange delete <profile-id> <profile-id2> <profile-idn> --force
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

- [auth0 token-exchange create](auth0_token-exchange_create.md) - Create a new token exchange profile
- [auth0 token-exchange delete](auth0_token-exchange_delete.md) - Delete a token exchange profile
- [auth0 token-exchange list](auth0_token-exchange_list.md) - List your token exchange profiles
- [auth0 token-exchange show](auth0_token-exchange_show.md) - Show a token exchange profile
- [auth0 token-exchange update](auth0_token-exchange_update.md) - Update a token exchange profile


