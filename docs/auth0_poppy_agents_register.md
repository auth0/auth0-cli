---
layout: default
parent: auth0 poppy agents
has_toc: false
---
# auth0 poppy agents register

(Experimental) Registers an agent from the URL of its metadata document. The command previews the document, creates or updates the agent app, creates the agent record and grants the agent your Poppy APIs. It changes the tenant only: the config file is not written, and is read only to show the hints of the scopes on offer. By default it grants every Poppy API of the tenant. In a terminal it asks which scopes to grant; otherwise pass `--scopes` or `--all-scopes`. DPoP is required only with `--require-dpop`. Running it again refreshes the agent app from the current document and changes nothing else, which is also how you pick up keys the agent has rotated.

**Warning:** This command is experimental and is subject to change in future versions.

## Usage
```
auth0 poppy agents register [agent-url] [flags]
```

## Examples

```
  auth0 poppy agents register https://muse.example/agent.json
  auth0 poppy agents register https://muse.example/agent.json --api https://api.shop.example --scopes orders:read --yes
  auth0 poppy agents register https://muse.example/agent.json --all-scopes --require-dpop
```


## Flags

```
      --all-scopes        Grant every offered scope. Mutually exclusive with --scopes.
      --api stringArray   A Poppy API, by id or identifier. Repeat for several. When registering an agent, the Poppy APIs to grant (default: every Poppy API of the tenant).
      --json              Output in json format.
      --json-compact      Output in compact json format.
      --name string       The Company's name for the config file. When registering an agent, the agent's name.
      --out string        Path of the config file. (default "auth0-poppy.json")
      --require-dpop      Require DPoP-bound tokens for the agent. Off by default.
      --scopes strings    Comma-separated list of scopes to grant. Without a terminal, pass this or --all-scopes. Mutually exclusive with --all-scopes.
      --yes               Skip confirmation.
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


