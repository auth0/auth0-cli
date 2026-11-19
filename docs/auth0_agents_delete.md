---
layout: default
parent: auth0 agents
has_toc: false
---
# auth0 agents delete

Delete an agent.

Linked applications are not deleted and keep the deleted agent's ID; run `auth0 apps update <app-id> --unlink-agent` to clear it.

To delete interactively, use `auth0 agents delete` with no arguments.

To delete non-interactively, supply the agent ID and use `--force` to skip confirmation.

## Usage
```
auth0 agents delete [flags]
```

## Examples

```
  auth0 agents delete
  auth0 agents rm
  auth0 agents delete <agent-id>
  auth0 agents delete <agent-id> --force
  auth0 agents delete <agent-id> <agent-id2> --force
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

- [auth0 agents create](auth0_agents_create.md) - Create a new agent
- [auth0 agents delete](auth0_agents_delete.md) - Delete an agent
- [auth0 agents list](auth0_agents_list.md) - List your agents
- [auth0 agents register](auth0_agents_register.md) - Register an agent so it can sign in to your company
- [auth0 agents show](auth0_agents_show.md) - Show an agent
- [auth0 agents unregister](auth0_agents_unregister.md) - Unregister an agent and remove its API access
- [auth0 agents update](auth0_agents_update.md) - Update an agent


