---
layout: default
parent: auth0 agents
has_toc: false
---
# auth0 agents show

Display details about an agent.

## Usage
```
auth0 agents show [flags]
```

## Examples

```
  auth0 agents show
  auth0 agents show <agent-id>
  auth0 agents show <agent-id> --json
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

- [auth0 agents create](auth0_agents_create.md) - Create a new agent
- [auth0 agents delete](auth0_agents_delete.md) - Delete an agent
- [auth0 agents list](auth0_agents_list.md) - List your agents
- [auth0 agents show](auth0_agents_show.md) - Show an agent
- [auth0 agents update](auth0_agents_update.md) - Update an agent


