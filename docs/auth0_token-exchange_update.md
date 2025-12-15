---
layout: default
parent: auth0 token-exchange
has_toc: false
---
# auth0 token-exchange update

Update a token exchange profile.

To update interactively, use `auth0 token-exchange update` with no arguments.

To update non-interactively, supply the profile id, name, and subject token type through the flags.

Note: Only name and subject token type can be updated. Action ID and type are immutable after creation.

## Usage
```
auth0 token-exchange update [flags]
```

## Examples

```
  auth0 token-exchange update
  auth0 token-exchange update <profile-id>
  auth0 token-exchange update <profile-id> --name "Updated Profile Name"
  auth0 token-exchange update <profile-id> --name "Updated Profile Name" --subject-token-type "urn:ietf:params:oauth:token-type:jwt"
  auth0 token-exchange update <profile-id> -n "Updated Profile Name" -s "urn:ietf:params:oauth:token-type:jwt" --json
  auth0 token-exchange update <profile-id> -n "Updated Profile Name" -s "urn:ietf:params:oauth:token-type:jwt" --json-compact
```


## Flags

```
      --json                        Output in json format.
      --json-compact                Output in compact json format.
  -n, --name string                 Name of the token exchange profile.
  -s, --subject-token-type string   Type of the subject token. Must be a valid URI format (e.g., urn:ietf:params:oauth:token-type:jwt). Cannot use reserved prefixes: http://auth0.com, https://auth0.com, http://okta.com, https://okta.com, urn:ietf, urn:auth0, urn:okta.
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


