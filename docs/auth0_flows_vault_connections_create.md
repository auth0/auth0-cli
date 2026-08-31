---
layout: default
parent: auth0 flows vault connections
has_toc: false
---
# auth0 flows vault connections create

Create a new vault connection.

Prompts for name and app id, then asks whether to add setup credentials. Use `--setup-file` to supply credentials non-interactively. Run `--setup-template --app-id <APP_ID>` to print the setup credentials template for a given app.

## Usage
```
auth0 flows vault connections create [flags]
```

## Examples

```
  auth0 flows vault connections create
  auth0 flows vault connections create --name "My Connection" --app-id SLACK
  auth0 flows vault connections create --name "My Connection" --app-id SLACK --setup-file ./setup.json
  auth0 flows vault connections create --setup-template --app-id SLACK > setup.json
```


## Flags

```
      --app-id string       Identifier of the app the Vault connection integrates with (e.g. HTTP, SLACK).
      --json                Output in json format.
      --json-compact        Output in compact json format.
      --name string         Name of the Vault connection.
  -f, --setup-file string   Path to a JSON file containing the vault connection setup credentials. Run with --setup-template --app-id <APP_ID> to see the expected setup schema for a given app.
      --setup-template      Print the setup credentials template for the given --app-id and exit.
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


