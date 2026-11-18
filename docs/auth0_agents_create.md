---
layout: default
parent: auth0 agents
has_toc: false
---
# auth0 agents create

Create a new agent.

To create interactively, use `auth0 agents create` with no flags.

To create non-interactively, supply at least the name through the flags.

Use '--schema' to print the request payload schema and exit.
Use '--data' to supply the full JSON payload (validated against the schema before sending).

## Usage
```
auth0 agents create [flags]
```

## Examples

```
  auth0 agents create
  auth0 agents create --name "Support Bot"
  auth0 agents create -n "Support Bot" --client-id <client-id> --external-id bot-42 --metadata '{"team":"support"}' --json
  auth0 agents create -n "Support Bot" --metadata @metadata.json

  # Discover the payload schema
  auth0 agents create --schema

  # JSON input mode (for agents and automation)
  auth0 agents create --data '{"name":"Support Bot"}'
  auth0 agents create --data @agent.json
```


## Flags

```
  -c, --client-id string     ID of an existing machine to machine application to link to the agent on creation.
      --data string          JSON payload for the operation, as a JSON string or file path (@file.json). Can also be piped via stdin.
  -e, --external-id string   Your own identifier for the agent. It cannot be changed after creation.
      --json                 Output in json format.
      --json-compact         Output in compact json format.
  -m, --metadata string      Metadata for the agent as a JSON object (max 10KB), e.g. '{"team":"ai"}', or a file (@metadata.json). On update, null or {} clears it.
  -n, --name string          Name of the agent (1-255 characters).
      --schema               Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
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


