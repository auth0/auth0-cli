---
layout: default
parent: auth0 token-exchange
has_toc: false
---
# auth0 token-exchange show

Display the name, subject token type, action ID, type and other information about a token exchange profile.

## Usage
```
auth0 token-exchange show [flags]
```

## Examples

```
  auth0 token-exchange show
  auth0 token-exchange show <profile-id>
  auth0 token-exchange show <profile-id> --json
  auth0 token-exchange show <profile-id> --json-compact
```


## Flags

```
      --json           Output in json format.
      --json-compact   Output in compact json format.
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


