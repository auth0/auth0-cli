---
layout: default
parent: auth0 connections enabled-clients
has_toc: false
---
# auth0 connections enabled-clients update

Update which applications (clients) have this connection enabled.

To update interactively, run without '--data': the tenant's applications are listed
with the currently-enabled ones pre-selected, and only your changes are sent.

To update non-interactively, supply the desired client statuses through '--data' as a
JSON array of objects with 'client_id' and 'status' fields (up to 50 per request).

## Usage
```
auth0 connections enabled-clients update [flags]
```

## Examples

```
  auth0 connections enabled-clients update
  auth0 connections enabled-clients update <connection-id>
  auth0 connections enabled-clients update <connection-id> --data @clients.json
  auth0 connections enabled-clients update <connection-id> --data '[{"client_id":"abc","status":true}]'
```


## Flags

```
      --data string   JSON payload for the operation, as a JSON string or file path (@file.json). Can also be piped via stdin.
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

- [auth0 connections enabled-clients show](auth0_connections_enabled-clients_show.md) - Show the clients enabled on a connection
- [auth0 connections enabled-clients update](auth0_connections_enabled-clients_update.md) - Update the clients enabled on a connection


