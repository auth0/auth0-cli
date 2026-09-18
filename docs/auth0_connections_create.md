---
layout: default
parent: auth0 connections
has_toc: false
---
# auth0 connections create

Create a new connection.

To create interactively, use 'auth0 connections create' with no flags.

To create non-interactively, supply the connection name and strategy through the flags,
or the whole payload through '--data'.

## JSON Input (for agents and automation)

Use '--schema' to print the request payload schema, then '--data' to provide
connection data as JSON:
  - Inline JSON: --data '{"name":"my-connection","strategy":"auth0"}'
  - From file: --data @connection.json
  - From stdin: pipe data in (e.g. cat connection.json | auth0 connections create)

The JSON is validated against the OpenAPI schema before sending to the API.

## Usage
```
auth0 connections create [flags]
```

## Examples

```
  # Interactive mode
  auth0 connections create

  # Flag-based mode
  auth0 connections create --name my-db --strategy auth0

  # JSON mode
  auth0 connections create --schema
  auth0 connections create --data @connection.json
  auth0 connections create --data '{"name":"my-db","strategy":"auth0"}'
  cat connection.json | auth0 connections create
```


## Flags

```
      --data string       JSON payload for the operation, as a JSON string or file path (@file.json). Can also be piped via stdin.
      --json              Output in json format.
      --json-compact      Output in compact json format.
  -n, --name string       Name of the connection.
      --schema            Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
  -s, --strategy string   Strategy of the connection. Determines the identity provider (e.g. auth0, google-oauth2, samlp, oidc, waad, ad, oauth2).
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


