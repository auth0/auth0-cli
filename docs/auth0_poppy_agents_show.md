---
layout: default
parent: auth0 poppy agents
has_toc: false
---
# auth0 poppy agents show

(Experimental) Shows one agent's details, given its URL or agent ID. It also checks the agent's metadata document live and tells you whether the keys Auth0 stored are still current, so you know when the agent needs to be registered again.

**Warning:** This command is experimental and is subject to change in future versions.

## Usage
```
auth0 poppy agents show [agent-url|agent-id] [flags]
```

## Examples

```
  auth0 poppy agents show https://muse.example/agent.json
  auth0 poppy agents show <agent-id>
  auth0 poppy agents show https://muse.example/agent.json --json
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

- [auth0 poppy agents list](auth0_poppy_agents_list.md) - List the registered agents
- [auth0 poppy agents register](auth0_poppy_agents_register.md) - Register an agent so it can sign in to your company
- [auth0 poppy agents remove](auth0_poppy_agents_remove.md) - Remove an agent's Poppy access
- [auth0 poppy agents show](auth0_poppy_agents_show.md) - Show the details of an agent


