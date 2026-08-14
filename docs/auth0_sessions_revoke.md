---
layout: default
parent: auth0 sessions
has_toc: false
---
# auth0 sessions revoke

Revoke a session and all of its associated refresh tokens.

Unlike `delete`, revoke also invalidates every refresh token tied to the session.

To revoke non-interactively, supply the session id and the `--force` flag to skip confirmation.

## Usage
```
auth0 sessions revoke [flags]
```

## Examples

```
  auth0 sessions revoke
  auth0 sessions revoke <session-id>
  auth0 sessions revoke <session-id> --force
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

- [auth0 sessions delete](auth0_sessions_delete.md) - Delete a session
- [auth0 sessions revoke](auth0_sessions_revoke.md) - Revoke a session
- [auth0 sessions show](auth0_sessions_show.md) - Show a session
- [auth0 sessions update](auth0_sessions_update.md) - Update a session


