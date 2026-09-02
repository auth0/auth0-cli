---
layout: default
parent: auth0 flows
has_toc: false
---
# auth0 flows list

List your existing flows. To create one, run: `auth0 flows create`.

## Usage
```
auth0 flows list [flags]
```

## Examples

```
  auth0 flows list
  auth0 flows ls
  auth0 flows ls --number 100
  auth0 flows ls --hydrate
  auth0 flows ls --json
```


## Flags

```
      --csv            Output in csv format.
      --hydrate        Hydrate the response with the number of forms referencing each flow.
      --json           Output in json format.
      --json-compact   Output in compact json format.
  -n, --number int     Number of flows to retrieve. Fetched across pages. (default 100)
      --synchronous    Filter to synchronous (true) or asynchronous (false) flows.
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


