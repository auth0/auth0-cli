---
layout: default
parent: auth0 agents
has_toc: false
---
# auth0 agents register

(Experimental) Registers an agent from the URL of its metadata document. The command previews the document, creates or updates the agent app, creates the agent record and grants the agent the APIs you choose. It changes the tenant only, and does not write the config file.

Pass `--api` for each API to grant, by ID or identifier, or `--poppy` to grant every Poppy API of the tenant. In a terminal, without either, it asks which APIs to grant. Choose the scopes with `--scopes` or `--all-scopes`; in a terminal it asks otherwise. To let the agent browse your website on behalf of a person, include the scope `poppy:browser-sign-in`. DPoP is required only with `--require-dpop`.

In a terminal the command shows what it will do and asks first; `--force` skips the question. Without a terminal it applies the change. With `--poppy`, or `--config`, it reads the config file, if there is one, to show the hints of the scopes on offer.

Running it again refreshes the agent app from the current document and changes nothing else, which is also how you pick up keys the agent has rotated.

**Warning:** This command is experimental and is subject to change in future versions.

## Usage
```
auth0 agents register [agent-url] [flags]
```

## Examples

```
  auth0 agents register https://muse.example/agent.json
  auth0 agents register https://muse.example/agent.json --api https://api.shop.example --scopes orders:read,poppy:browser-sign-in
  auth0 agents register https://muse.example/agent.json --api https://api.shop.example --api https://api.crm.example --scopes https://api.shop.example=orders:read,https://api.crm.example=contacts:read
  auth0 agents register https://muse.example/agent.json --poppy --all-scopes
  auth0 agents register https://muse.example/agent.json --api https://api.shop.example --all-scopes --require-dpop
```


## Flags

```
      --all-scopes        Grant every offered scope. Mutually exclusive with --scopes.
      --api stringArray   An API to grant, by ID or identifier. Repeat for several. In a terminal, without this or --poppy, you are asked which APIs to grant.
      --config string     Config file to read scope hints from. Used only if it exists. (default "poppy.json")
      --force             Skip confirmation.
      --json              Output in json format.
      --json-compact      Output in compact json format.
      --name string       The agent's name.
      --poppy             Limit the APIs to Poppy APIs. On its own it grants every Poppy API of the tenant. With --api, every API named must be a Poppy API.
      --require-dpop      Require DPoP-bound tokens for the agent. Off by default.
      --scopes strings    Scopes to grant. Repeat the flag or separate scopes with commas. With several APIs, each API gets the listed scopes it offers; to give a scope to one API only, write <api>=<scope>. Without a terminal, pass this or --all-scopes. Mutually exclusive with --all-scopes.
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


