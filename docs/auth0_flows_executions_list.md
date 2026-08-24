---
layout: default
parent: auth0 flows executions
has_toc: false
---
# auth0 flows executions list

List the executions produced by a flow.

## Usage
```
auth0 flows executions list [flags]
```

## Examples

```
  auth0 flows executions list <flow-id>
  auth0 flows executions ls <flow-id> --number 100
  auth0 flows executions list <flow-id> --json
```


## Flags

```
      --csv            Output in csv format.
      --from string    Cursor id from which to start selection.
      --json           Output in json format.
      --json-compact   Output in compact json format.
  -n, --number int     Number of executions to retrieve. Fetched across pages. (default 100)
      --take int       Number of executions to retrieve per page.
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


