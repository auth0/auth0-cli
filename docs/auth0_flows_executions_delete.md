---
layout: default
parent: auth0 flows executions
has_toc: false
---
# auth0 flows executions delete

Delete one or more executions of a flow.

Supply the flow id followed by the execution ids. Use `--force` to skip confirmation.

## Usage
```
auth0 flows executions delete [flags]
```

## Examples

```
  auth0 flows executions delete <flow-id> <execution-id>
  auth0 flows executions rm <flow-id> <execution-id> --force
  auth0 flows executions delete <flow-id> <execution-id> <execution-id2>
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

- [auth0 flows executions delete](auth0_flows_executions_delete.md) - Delete a flow execution
- [auth0 flows executions list](auth0_flows_executions_list.md) - List a flow's executions
- [auth0 flows executions show](auth0_flows_executions_show.md) - Show a flow execution


