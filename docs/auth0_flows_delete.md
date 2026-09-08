---
layout: default
parent: auth0 flows
has_toc: false
---
# auth0 flows delete

Delete a flow.

To delete interactively, use `auth0 flows delete` with no arguments.

To delete non-interactively, supply the flow id and the `--force` flag to skip confirmation.

## Usage
```
auth0 flows delete [flags]
```

## Examples

```
  auth0 flows delete
  auth0 flows rm
  auth0 flows delete <flow-id>
  auth0 flows delete <flow-id> --force
  auth0 flows delete <flow-id> <flow-id2> <flow-idn>
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

- [auth0 flows create](auth0_flows_create.md) - Create a new flow
- [auth0 flows delete](auth0_flows_delete.md) - Delete a flow
- [auth0 flows executions](auth0_flows_executions.md) - Manage Flow executions
- [auth0 flows list](auth0_flows_list.md) - List your flows
- [auth0 flows open](auth0_flows_open.md) - Open a flow in the Auth0 Dashboard
- [auth0 flows show](auth0_flows_show.md) - Show a flow
- [auth0 flows update](auth0_flows_update.md) - Update a flow
- [auth0 flows vault](auth0_flows_vault.md) - Manage Flow vault connections


