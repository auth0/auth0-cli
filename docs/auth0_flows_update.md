---
layout: default
parent: auth0 flows
has_toc: false
---
# auth0 flows update

Update a flow.

Passing `--file` (or piped stdin) replaces every top-level field present in the file. Passing only `--name` performs a merge that preserves the flow's actions. Server-managed fields such as `id`, `created_at`, and `updated_at` are removed before the request is sent.

## Usage
```
auth0 flows update [flags]
```

## Examples

```
  auth0 flows update <flow-id> --name "New Name"
  auth0 flows update <flow-id> --file ./flow.json
  cat flow.json | auth0 flows update <flow-id> -f -
```


## Flags

```
  -f, --file string    Path to a JSON file with the flow body. Use '-' to read from stdin.
      --json           Output in json format.
      --json-compact   Output in compact json format.
      --name string    Name of the Flow.
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

- [auth0 flows create](auth0_flows_create.md) - Create a new flow
- [auth0 flows delete](auth0_flows_delete.md) - Delete a flow
- [auth0 flows executions](auth0_flows_executions.md) - Manage Flow executions
- [auth0 flows list](auth0_flows_list.md) - List your flows
- [auth0 flows open](auth0_flows_open.md) - Open a flow in the Auth0 Dashboard
- [auth0 flows show](auth0_flows_show.md) - Show a flow
- [auth0 flows update](auth0_flows_update.md) - Update a flow
- [auth0 flows vault](auth0_flows_vault.md) - Manage Flow vault connections


