---
layout: default
parent: auth0 sessions
has_toc: false
---
# auth0 sessions show

Display the device, clients, and expiry information about a session.

## Usage
```
auth0 sessions show [flags]
```

## Examples

```
  auth0 sessions show
  auth0 sessions show <session-id>
  auth0 sessions show <session-id> --json
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

- [auth0 sessions delete](auth0_sessions_delete.md) - Delete a session
- [auth0 sessions revoke](auth0_sessions_revoke.md) - Revoke a session
- [auth0 sessions show](auth0_sessions_show.md) - Show a session
- [auth0 sessions update](auth0_sessions_update.md) - Update a session


