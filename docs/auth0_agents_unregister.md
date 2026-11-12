---
layout: default
parent: auth0 agents
has_toc: false
---
# auth0 agents unregister

(Experimental) Unregisters the agent and removes its API access: all of its grants, then its agent record and its app. It changes the tenant only, and does not touch the config file. Registering the agent again creates a new app and its users must sign in again.

With `--poppy` only the grants on your Poppy APIs are removed, and the agent record and the app are removed only when the app has no other grants; otherwise the other grants are listed. Deleting the app revokes its refresh tokens; when the app is kept, its refresh tokens are not revoked. Access tokens already issued stay valid until they expire.

Without an argument, a terminal asks you to pick the agent. Use `--dry-run` to print what would be removed, and `--force` to skip the confirmation.

**Warning:** This command is experimental and is subject to change in future versions.

## Usage
```
auth0 agents unregister [agent-url|agent-id] [flags]
```

## Examples

```
  auth0 agents unregister
  auth0 agents unregister https://muse.example/agent.json
  auth0 agents unregister https://muse.example/agent.json --dry-run
  auth0 agents unregister https://muse.example/agent.json --poppy
  auth0 agents unregister <agent-id> --force
```


## Flags

```
      --dry-run        Print the plan and change nothing.
      --force          Skip confirmation.
      --json           Output in json format.
      --json-compact   Output in compact json format.
      --poppy          Remove only the grants on Poppy APIs. The agent record and the app are removed only when no other grant remains.
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


