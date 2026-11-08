---
layout: default
has_toc: false
has_children: true
---
# auth0 agents

Manage agents (Early Access): first-class agent principals in your tenant. Machine to machine applications can be linked to an agent with `auth0 apps update <app-id> --agent-id <agent-id>`.

These commands need the read:agents, create:agents, update:agents and delete:agents scopes, which are not requested by default. To grant them, run: `auth0 login --scopes read:agents,create:agents,update:agents,delete:agents`.

To manage skills for your AI coding assistants, use `auth0 agent` instead.

## Commands

- [auth0 agents create](auth0_agents_create.md) - Create a new agent
- [auth0 agents delete](auth0_agents_delete.md) - Delete an agent
- [auth0 agents list](auth0_agents_list.md) - List your agents
- [auth0 agents show](auth0_agents_show.md) - Show an agent
- [auth0 agents update](auth0_agents_update.md) - Update an agent

