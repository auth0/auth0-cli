---
layout: default
parent: auth0 flows vault connections
has_toc: false
---
# auth0 flows vault connections create

Create a new vault connection.

Interactive behavior: `auth0 flows vault connections create` asks for the name and app id, then opens an editor seeded with a provider-specific `setup` template so you can enter the connection secrets. Alternatively, supply the whole body (including its `setup` secrets) via `--file` (or piped stdin); `--name` and `--app-id` override the corresponding fields after the file is parsed. Run `auth0 flows vault connections create --example` to print a template.

## Usage
```
auth0 flows vault connections create [flags]
```

## Examples

```
  auth0 flows vault connections create
  auth0 flows vault connections create --file ./connection.json
  auth0 flows vault connections create --file ./connection.json --name "My Connection"
  auth0 flows vault connections create --example > connection.json
  cat connection.json | auth0 flows vault connections create -f -
```


## Flags

```
      --app-id string   Identifier of the app the Vault connection integrates with (e.g. HTTP, SLACK).
      --example         Print an example flow JSON body and exit.
  -f, --file string     Path to a JSON file with the vault connection body (including its setup secrets). Use '-' to read from stdin.
      --json            Output in json format.
      --json-compact    Output in compact json format.
      --name string     Name of the Vault connection.
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


