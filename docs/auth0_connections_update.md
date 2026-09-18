---
layout: default
parent: auth0 connections
has_toc: false
---
# auth0 connections update

Update a connection.

To update interactively, use 'auth0 connections update' with no '--data' flag: the
current configuration opens in your editor and the saved result is sent as a PATCH.

To update non-interactively, supply the new configuration through '--data'.

Note: the entire 'options' object is overridden on update, so include all option
fields you want to keep.

## JSON Input (for agents and automation)

Use '--schema' to print the request payload schema, then '--data' to provide
connection data as JSON (inline, @file, or piped stdin). The JSON is validated
against the OpenAPI schema before sending to the API.

## Usage
```
auth0 connections update [flags]
```

## Examples

```
  auth0 connections update <connection-id>
  auth0 connections update <connection-id> --data @connection.json
  auth0 connections update <connection-id> --data '{"options":{...}}'
  auth0 connections update --schema
```


## Flags

```
      --data string    JSON payload for the operation, as a JSON string or file path (@file.json). Can also be piped via stdin.
      --json           Output in json format.
      --json-compact   Output in compact json format.
      --schema         Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
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


