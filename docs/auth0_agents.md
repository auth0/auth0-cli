---
layout: default
has_toc: false
has_children: true
---
# auth0 agents

Manage agents (Early Access): first-class agent principals in your tenant. Machine to machine applications can be linked to an agent with `auth0 apps update <app-id> --agent-id <agent-id>`.

These commands need the read:agents, create:agents, update:agents and delete:agents scopes, which are available through a machine to machine (M2M) login only. Authorize an M2M application for the Auth0 Management API with these scopes, then log in with it.

To let an agent that publishes a metadata document sign in to your company, use `auth0 agents register <agent-url>`; `auth0 agents unregister` undoes it. `list` and `show` also report the agent's application, its metadata status and the Poppy APIs it can use.

To manage skills for your AI coding assistants, use `auth0 agent` instead.

## Commands

- [auth0 agents create](auth0_agents_create.md) - Create a new agent
- [auth0 agents delete](auth0_agents_delete.md) - Delete an agent
- [auth0 agents list](auth0_agents_list.md) - List your agents
- [auth0 agents register](auth0_agents_register.md) - Register an agent so it can sign in to your company
- [auth0 agents show](auth0_agents_show.md) - Show an agent
- [auth0 agents unregister](auth0_agents_unregister.md) - Unregister an agent and remove its API access
- [auth0 agents update](auth0_agents_update.md) - Update an agent

