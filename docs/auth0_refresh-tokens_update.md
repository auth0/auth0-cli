---
layout: default
parent: auth0 refresh-tokens
has_toc: false
---
# auth0 refresh-tokens update

Update the metadata on a refresh token.

Metadata is the only writable field on a refresh token. The pairs you pass replace the existing metadata; passing no pairs clears it.

## Usage
```
auth0 refresh-tokens update [flags]
```

## Examples

```
  auth0 refresh-tokens update <token-id> --metadata key=value
  auth0 refresh-tokens update <token-id> -m key1=value1 -m key2=value2
  auth0 refresh-tokens update <token-id> --metadata key=value --json
```


## Flags

```
      --json                      Output in json format.
      --json-compact              Output in compact json format.
  -m, --metadata stringToString   Metadata key/value pairs to set on the refresh token, e.g. --metadata key=value. Repeat the flag or comma-separate pairs for multiple values. Passing no pairs clears the metadata. (default [])
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

- [auth0 refresh-tokens delete](auth0_refresh-tokens_delete.md) - Delete a refresh token
- [auth0 refresh-tokens revoke](auth0_refresh-tokens_revoke.md) - Revoke refresh tokens
- [auth0 refresh-tokens show](auth0_refresh-tokens_show.md) - Show a refresh token
- [auth0 refresh-tokens update](auth0_refresh-tokens_update.md) - Update a refresh token


