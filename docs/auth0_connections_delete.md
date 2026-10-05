---
layout: default
parent: auth0 connections
has_toc: false
---
# auth0 connections delete

Delete a connection.

To delete interactively, use `auth0 connections delete` with no arguments.

To delete non-interactively, supply the connection id and the `--force` flag to skip confirmation.

## Usage
```
auth0 connections delete [flags]
```

## Examples

```
  auth0 connections delete
  auth0 connections rm
  auth0 connections delete <connection-id>
  auth0 connections delete <connection-id> --force
  auth0 connections delete <connection-id> <connection-id2> <connection-idn>
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

- [auth0 connections create](auth0_connections_create.md) - Create a new connection
- [auth0 connections delete](auth0_connections_delete.md) - Delete a connection
- [auth0 connections enabled-clients](auth0_connections_enabled-clients.md) - Manage the clients enabled on a connection
- [auth0 connections list](auth0_connections_list.md) - List your connections
- [auth0 connections show](auth0_connections_show.md) - Show a connection
- [auth0 connections update](auth0_connections_update.md) - Update a connection


