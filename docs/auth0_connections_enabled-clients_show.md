---
layout: default
parent: auth0 connections enabled-clients
has_toc: false
---
# auth0 connections enabled-clients show

List the applications (clients) that have this connection enabled.

## Usage
```
auth0 connections enabled-clients show [flags]
```

## Examples

```
  auth0 connections enabled-clients show
  auth0 connections enabled-clients show <connection-id>
  auth0 connections enabled-clients show <connection-id> --json
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

- [auth0 connections enabled-clients show](auth0_connections_enabled-clients_show.md) - Show the clients enabled on a connection
- [auth0 connections enabled-clients update](auth0_connections_enabled-clients_update.md) - Update the clients enabled on a connection


