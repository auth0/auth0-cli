---
layout: default
parent: auth0 flows vault connections
has_toc: false
---
# auth0 flows vault connections update

Update a vault connection.

Passing `--file` (or piped stdin) replaces every top-level field present in the file. Passing only `--name` performs a merge. Server-managed fields such as `id`, `ready`, and `fingerprint` are removed before the request is sent.

## Usage
```
auth0 flows vault connections update [flags]
```

## Examples

```
  auth0 flows vault connections update <connection-id> --name "New Name"
  auth0 flows vault connections update <connection-id> --file ./connection.json
  cat connection.json | auth0 flows vault connections update <connection-id> -f -
```


## Flags

```
  -f, --file string    Path to a JSON file with the vault connection body (including its setup secrets). Use '-' to read from stdin.
      --json           Output in json format.
      --json-compact   Output in compact json format.
      --name string    Name of the Vault connection.
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


