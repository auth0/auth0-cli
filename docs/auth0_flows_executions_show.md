---
layout: default
parent: auth0 flows executions
has_toc: false
---
# auth0 flows executions show

Display information about a flow execution.

## Usage
```
auth0 flows executions show [flags]
```

## Examples

```
  auth0 flows executions show <flow-id> <execution-id>
  auth0 flows executions show <flow-id> <execution-id> --json
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

- [auth0 flows executions delete](auth0_flows_executions_delete.md) - Delete a flow execution
- [auth0 flows executions list](auth0_flows_executions_list.md) - List a flow's executions
- [auth0 flows executions show](auth0_flows_executions_show.md) - Show a flow execution


