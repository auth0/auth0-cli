---
layout: default
parent: auth0 flows vault connections
has_toc: false
---
# auth0 flows vault connections show

Display information about a vault connection. Secret values are never returned by the API.

## Usage
```
auth0 flows vault connections show [flags]
```

## Examples

```
  auth0 flows vault connections show
  auth0 flows vault connections show <connection-id>
  auth0 flows vault connections show <connection-id> --json
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

- [auth0 flows vault connections create](auth0_flows_vault_connections_create.md) - Create a new vault connection
- [auth0 flows vault connections delete](auth0_flows_vault_connections_delete.md) - Delete a vault connection
- [auth0 flows vault connections list](auth0_flows_vault_connections_list.md) - List your vault connections
- [auth0 flows vault connections show](auth0_flows_vault_connections_show.md) - Show a vault connection
- [auth0 flows vault connections update](auth0_flows_vault_connections_update.md) - Update a vault connection


