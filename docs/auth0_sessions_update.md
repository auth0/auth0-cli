---
layout: default
parent: auth0 sessions
has_toc: false
---
# auth0 sessions update

Update the metadata on a session.

Metadata is the only writable field on a session. The pairs you pass replace the existing metadata; passing no pairs clears it.

## Usage
```
auth0 sessions update [flags]
```

## Examples

```
  auth0 sessions update <session-id> --metadata key=value
  auth0 sessions update <session-id> -m key1=value1 -m key2=value2
  auth0 sessions update <session-id> --metadata key=value --json
```


## Flags

```
      --json                      Output in json format.
      --json-compact              Output in compact json format.
  -m, --metadata stringToString   Metadata key/value pairs to set on the session, e.g. --metadata key=value. Repeat the flag or comma-separate pairs for multiple values. Passing no pairs clears the metadata. (default [])
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


