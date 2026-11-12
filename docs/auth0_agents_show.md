---
layout: default
parent: auth0 agents
has_toc: false
---
# auth0 agents show

Display details about an agent, given its ID or its URL (the external agent ID).

It also shows the client ID of the linked app, its metadata status (- for an app that is not an agent app), whether DPoP is required and the APIs the agent has grants on, with their scopes; pass `--poppy` to show only the Poppy APIs. For an agent app it checks the agent's metadata document live, and tells you whether the keys Auth0 stored are still current. A value that cannot be read, for lack of a scope, shows as unknown.

## Usage
```
auth0 agents show [agent-url|agent-id] [flags]
```

## Examples

```
  auth0 agents show
  auth0 agents show <agent-id>
  auth0 agents show <agent-id> --poppy
  auth0 agents show https://muse.example/agent.json
  auth0 agents show <agent-id> --json
```


## Flags

```
      --json           Output in json format.
      --json-compact   Output in compact json format.
      --poppy          Show only the grants on Poppy APIs.
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


