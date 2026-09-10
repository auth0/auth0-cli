---
layout: default
parent: auth0 roles
has_toc: false
---
# auth0 roles create

Create a new role.

To create interactively, use `auth0 roles create` with no arguments.

To create non-interactively, supply the role name and description through the flags.

Use '--schema' to print the request payload schema and exit.
Use '--data' to supply the full JSON payload (validated against the schema before sending).

## Usage
```
auth0 roles create [flags]
```

## Examples

```
  auth0 roles create
  auth0 roles create --name myrole --description "awesome role"
  auth0 roles create -n myrole -d "awesome role" --json-compact
  auth0 roles create -n myrole -d "awesome role" --json

  # Discover the payload schema
  auth0 roles create --schema
  auth0 roles create --schema --json

  # JSON input mode (for agents and automation)
  auth0 roles create --data '{"name":"myrole","description":"awesome role"}'
  auth0 roles create --data @role.json
  cat role.json | auth0 roles create
```


## Flags

```
      --data string          JSON payload for the operation, as a JSON string or file path (@file.json). Can also be piped via stdin.
  -d, --description string   Description of the role.
      --json                 Output in json format.
      --json-compact         Output in compact json format.
  -n, --name string          Name of the role.
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

- [auth0 roles create](auth0_roles_create.md) - Create a new role
- [auth0 roles delete](auth0_roles_delete.md) - Delete a role
- [auth0 roles list](auth0_roles_list.md) - List your roles
- [auth0 roles permissions](auth0_roles_permissions.md) - Manage permissions within the role resource
- [auth0 roles show](auth0_roles_show.md) - Show a role
- [auth0 roles update](auth0_roles_update.md) - Update a role


