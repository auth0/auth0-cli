---
layout: default
parent: auth0 flows vault connections
has_toc: false
---
# auth0 flows vault connections delete

Delete a vault connection.

To delete interactively, use `auth0 flows vault connections delete` with no arguments.

To delete non-interactively, supply the connection id and the `--force` flag.

## Usage
```
auth0 flows vault connections delete [flags]
```

## Examples

```
  auth0 flows vault connections delete
  auth0 flows vault connections rm
  auth0 flows vault connections delete <connection-id>
  auth0 flows vault connections delete <connection-id> --force
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

- [auth0 flows vault connections create](auth0_flows_vault_connections_create.md) - Create a new vault connection
- [auth0 flows vault connections delete](auth0_flows_vault_connections_delete.md) - Delete a vault connection
- [auth0 flows vault connections list](auth0_flows_vault_connections_list.md) - List your vault connections
- [auth0 flows vault connections show](auth0_flows_vault_connections_show.md) - Show a vault connection
- [auth0 flows vault connections update](auth0_flows_vault_connections_update.md) - Update a vault connection


