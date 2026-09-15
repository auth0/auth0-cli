---
layout: default
parent: auth0 connections
has_toc: false
---
# auth0 connections list

List your existing connections. To create one, run: `auth0 connections create`.

Use '--schema' to see available query parameters.
Use '--query' to filter results via a JSON object (any API-supported parameter works immediately).

## Usage
```
auth0 connections list [flags]
```

## Examples

```
  auth0 connections list
  auth0 connections ls
  auth0 connections ls --json
  auth0 connections ls --csv
  auth0 connections list --schema
  auth0 connections list --query '{"strategy":["auth0"]}'
  auth0 connections list --query '{"name":"my-connection"}' --json
```


## Flags

```
      --csv            Output in csv format.
      --json           Output in json format.
      --json-compact   Output in compact json format.
  -q, --query string   Filter connections with a JSON object of query parameters (e.g. '{"strategy":["auth0"]}'). Any API-supported parameter works immediately. Run '--schema' to see documented parameters.
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


