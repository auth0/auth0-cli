---
layout: default
parent: auth0 agents
has_toc: false
---
# auth0 agents update

Update an agent.

To update interactively, use `auth0 agents update` with no arguments.

To update non-interactively, supply the agent ID and the fields to change through the flags. Pass `--metadata null` to clear all metadata. The external ID cannot be changed.

Use '--schema' to print the request payload schema and exit.
Use '--data' to supply the full JSON payload (validated against the schema before sending).

## Usage
```
auth0 agents update [flags]
```

## Examples

```
  auth0 agents update
  auth0 agents update <agent-id> --name "New Name"
  auth0 agents update <agent-id> --metadata '{"team":"platform"}' --json
  auth0 agents update <agent-id> --metadata null

  # Discover the payload schema
  auth0 agents update --schema

  # JSON input mode (for agents and automation)
  auth0 agents update <agent-id> --data '{"name":"New Name"}'
```


## Flags

```
      --data string       JSON payload for the operation, as a JSON string or file path (@file.json). Can also be piped via stdin.
      --json              Output in json format.
      --json-compact      Output in compact json format.
  -m, --metadata string   Metadata for the agent as a JSON object (max 10KB), e.g. '{"team":"ai"}'.
  -n, --name string       Name of the agent (1-255 characters).
      --schema            Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
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


