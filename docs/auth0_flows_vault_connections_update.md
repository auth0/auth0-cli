---
layout: default
parent: auth0 flows vault connections
has_toc: false
---
# auth0 flows vault connections update

Update a vault connection.

Passing `--setup-file` replaces the connection's setup credentials; the file must contain only the setup body, not a name. Passing `--name` renames the connection; omit it to keep the current name. Run `auth0 flows vault connections create --setup-template --app-id <APP_ID>` to see the setup schema.

Use '--schema' to print the request payload schema and exit.
Use '--data' to supply the JSON payload (validated against the schema before sending).

## Usage
```
auth0 flows vault connections update [flags]
```

## Examples

```
  auth0 flows vault connections update <connection-id> --name "New Name"
  auth0 flows vault connections update <connection-id> --setup-file ./setup.json
  auth0 flows vault connections update --schema
  auth0 flows vault connections update <connection-id> --data '{"name":"New Name"}'
  auth0 flows vault connections update <connection-id> --data @connection.json
```


## Flags

```
      --data string         JSON payload for the operation, as a JSON string or file path (@file.json). Can also be piped via stdin.
      --json                Output in json format.
      --json-compact        Output in compact json format.
      --name string         Name of the Vault connection.
      --schema              Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
  -f, --setup-file string   Path to a JSON file containing the vault connection setup credentials. Run with --setup-template --app-id <APP_ID> to see the expected setup schema for a given app.
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

- [auth0 flows vault connections create](auth0_flows_vault_connections_create.md) - Create a new vault connection
- [auth0 flows vault connections delete](auth0_flows_vault_connections_delete.md) - Delete a vault connection
- [auth0 flows vault connections list](auth0_flows_vault_connections_list.md) - List your vault connections
- [auth0 flows vault connections show](auth0_flows_vault_connections_show.md) - Show a vault connection
- [auth0 flows vault connections update](auth0_flows_vault_connections_update.md) - Update a vault connection


