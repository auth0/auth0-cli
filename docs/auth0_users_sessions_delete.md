---
layout: default
parent: auth0 users sessions
has_toc: false
---
# auth0 users sessions delete

Delete all sessions for a user.

This deletes every session the user has, not a single session. To delete one session by its id, use `auth0 sessions delete`.

To delete non-interactively, supply the user id and the `--force` flag to skip confirmation.

## Usage
```
auth0 users sessions delete [flags]
```

## Examples

```
  auth0 users sessions delete
  auth0 users sessions rm <user-id>
  auth0 users sessions delete <user-id> --force
```


## Flags

```
      --force   Skip confirmation.
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

- [auth0 users sessions delete](auth0_users_sessions_delete.md) - Delete all of a user's sessions
- [auth0 users sessions list](auth0_users_sessions_list.md) - List a user's sessions


