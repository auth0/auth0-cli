---
layout: default
parent: auth0 poppy agents
has_toc: false
---
# auth0 poppy agents remove

(Experimental) Removes the agent's Poppy access: its grants to your Poppy APIs. It changes the tenant only, and does not touch the config file. When the agent has no other grants it also removes the agent record and the app, so registering the agent again creates a new app and its users must sign in again. When the app has other grants, only the Poppy access is removed and the other grants are listed, unless you pass `--remove-all-grants`; in a terminal you are asked which to do. This command does not revoke tokens already issued: access tokens stay valid until they expire. Use `--dry-run` to print what would be removed, and `--force` to skip the confirmation.

**Warning:** This command is experimental and is subject to change in future versions.

## Usage
```
auth0 poppy agents remove [agent-url|agent-id] [flags]
```

## Examples

```
  auth0 poppy agents remove https://muse.example/agent.json
  auth0 poppy agents remove https://muse.example/agent.json --dry-run
  auth0 poppy agents remove <agent-id> --remove-all-grants --force
```


## Flags

```
      --dry-run             Print the plan and change nothing.
      --force               Skip confirmation.
      --json                Output in json format.
      --json-compact        Output in compact json format.
      --remove-all-grants   Also delete the app's other grants, then the agent record and the app.
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


