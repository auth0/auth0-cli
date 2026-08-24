---
layout: default
parent: auth0 flows
has_toc: false
---
# auth0 flows create

Create a new flow.

Interactive behavior: `auth0 flows create` asks only for the name and creates a minimal scaffold; it does not open an editor. Pass `--edit` to open an editor and author the flow actions before it is created, or supply the whole body via `--file` (or piped stdin) with an optional `--name` override. Run `auth0 flows create --example > flow.json` to generate an accepted file payload.

## Usage
```
auth0 flows create [flags]
```

## Examples

```
  auth0 flows create
  auth0 flows create --name "My Flow"
  auth0 flows create --name "My Flow" --edit
  auth0 flows create --example > flow.json
  auth0 flows create --file ./flow.json
  cat flow.json | auth0 flows create -f -
```


## Flags

```
      --edit           Open an editor to author the flow graph after entering the name.
      --example        Print an example flow JSON body and exit.
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


