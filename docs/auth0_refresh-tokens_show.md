---
layout: default
parent: auth0 refresh-tokens
has_toc: false
---
# auth0 refresh-tokens show

Display the client, session, device, and expiry information about a refresh token.

## Usage
```
auth0 refresh-tokens show [flags]
```

## Examples

```
  auth0 refresh-tokens show
  auth0 refresh-tokens show <token-id>
  auth0 refresh-tokens show <token-id> --json
```


## Flags

```
      --json           Output in json format.
      --json-compact   Output in compact json format.
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


