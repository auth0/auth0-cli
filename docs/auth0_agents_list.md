---
layout: default
parent: auth0 agents
has_toc: false
---
# auth0 agents list

List your agents. To create one, run: `auth0 agents create`.

Besides the agent, each row shows the client ID of the linked app, its metadata status and the APIs the agent has grants on; pass `--poppy` to show only the Poppy APIs. A value that cannot be read, for lack of a scope, shows as unknown.

Use '--schema' to see available query parameters.
Use '--query' to filter results via a JSON object (any API-supported parameter works immediately).

## Usage
```
auth0 agents list [flags]
```

## Examples

```
  auth0 agents list
  auth0 agents ls
  auth0 agents list --poppy
  auth0 agents list --number 10
  auth0 agents ls -n 10 --json
  auth0 agents list --csv
  auth0 agents list --schema
  auth0 agents list --query '{"take":10}'
```


## Flags

```
      --csv            Output in csv format.
      --json           Output in json format.
      --json-compact   Output in compact json format.
  -n, --number int     Number of agents to retrieve. Minimum 1, maximum 1000. (default 100)
      --poppy          Show only the grants on Poppy APIs.
  -q, --query string   Filter results with a JSON object of query parameters. Any API-supported parameter works immediately. Run '--schema' to see documented parameters. On offset-paginated endpoints, add "include_totals":true to receive total counts and a pagination hint (without it the API returns a bare array and no hint can be given).
      --schema         Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
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

- [auth0 agents create](auth0_agents_create.md) - Create a new agent
- [auth0 agents delete](auth0_agents_delete.md) - Delete an agent
- [auth0 agents list](auth0_agents_list.md) - List your agents
- [auth0 agents register](auth0_agents_register.md) - Register an agent so it can sign in to your company
- [auth0 agents show](auth0_agents_show.md) - Show an agent
- [auth0 agents unregister](auth0_agents_unregister.md) - Unregister an agent and remove its API access
- [auth0 agents update](auth0_agents_update.md) - Update an agent


