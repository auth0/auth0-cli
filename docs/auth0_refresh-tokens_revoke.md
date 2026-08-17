---
layout: default
parent: auth0 refresh-tokens
has_toc: false
---
# auth0 refresh-tokens revoke

Revoke refresh tokens so they can no longer be exchanged for new access tokens.

Pass a token id to revoke a single token. Alternatively, use `--user-id` to revoke all of a user's tokens, optionally narrowing to a single client with `--client-id` and a single API with `--audience` (`--client-id` requires `--user-id`, and `--audience` requires both).

To revoke non-interactively, supply the token id and the `--force` flag to skip confirmation.

## Usage
```
auth0 refresh-tokens revoke [flags]
```

## Examples

```
  auth0 refresh-tokens revoke
  auth0 refresh-tokens revoke <token-id>
  auth0 refresh-tokens revoke <token-id> --force
  auth0 refresh-tokens revoke --user-id <user-id> --force
  auth0 refresh-tokens revoke --user-id <user-id> --client-id <client-id>
  auth0 refresh-tokens revoke --user-id <user-id> --client-id <client-id> --audience <audience>
```


## Flags

```
      --audience string    Narrow a user+client revocation to a single API audience. Requires --user-id and --client-id.
      --client-id string   Narrow a user revocation to a single client. Requires --user-id.
      --force              Skip confirmation.
      --user-id string     Revoke all refresh tokens for this user, instead of a single token by id.
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


