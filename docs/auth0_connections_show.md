---
layout: default
parent: auth0 connections
has_toc: false
---
# auth0 connections show

Display the full configuration of a connection, including its strategy-specific options.

## Usage
```
auth0 connections show [flags]
```

## Examples

```
  auth0 connections show
  auth0 connections show <connection-id>
  auth0 connections show <connection-id> --json
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

- [auth0 connections create](auth0_connections_create.md) - Create a new connection
- [auth0 connections delete](auth0_connections_delete.md) - Delete a connection
- [auth0 connections enabled-clients](auth0_connections_enabled-clients.md) - Manage the clients enabled on a connection
- [auth0 connections list](auth0_connections_list.md) - List your connections
- [auth0 connections show](auth0_connections_show.md) - Show a connection
- [auth0 connections update](auth0_connections_update.md) - Update a connection


