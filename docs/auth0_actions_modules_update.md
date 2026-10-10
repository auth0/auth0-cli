---
layout: default
parent: auth0 actions modules
has_toc: false
---
# auth0 actions modules update

Update an action module.

The module name is immutable and cannot be changed. Only the fields you pass are updated; omitted fields keep their current values.

Updates edit the module's draft. Pass `--publish` to also snapshot the draft as a new immutable version once the update succeeds.

## Usage
```
auth0 actions modules update [flags]
```

## Examples

```
  auth0 actions modules update <module-id> --code "$(cat path/to/module.js)"
  auth0 actions modules update <module-id> --dependency "lodash=4.0.0" --secret "API_KEY=value"
  auth0 actions modules update <module-id> --code "$(cat path/to/module.js)" --publish
  auth0 actions modules update <module-id> -c "$(cat path/to/module.js)" --json

  # Discover the payload schema
  auth0 actions modules update --schema
  auth0 actions modules update --schema --json

  # JSON input mode (for agents and automation)
  auth0 actions modules update <module-id> --data '{"code":"module.exports = {};"}'
  auth0 actions modules update <module-id> --data @module.json
```


## Flags

```
  -c, --code string                 Code content of the action module.
      --data string                 JSON payload for the operation, as a JSON string or file path (@file.json). Can also be piped via stdin.
  -d, --dependency stringToString   Third party npm module, and its version, that the action module depends on. (default [])
      --json                        Output in json format.
      --json-compact                Output in compact json format.
      --publish                     Publish the module's draft as a new immutable version once the create or update succeeds.
      --schema                      Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
  -s, --secret stringToString       Secrets to be used in the action module. (default [])
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

- [auth0 actions modules actions](auth0_actions_modules_actions.md) - Manage the actions using an action module
- [auth0 actions modules create](auth0_actions_modules_create.md) - Create a new action module
- [auth0 actions modules delete](auth0_actions_modules_delete.md) - Delete an action module
- [auth0 actions modules list](auth0_actions_modules_list.md) - List your action modules
- [auth0 actions modules show](auth0_actions_modules_show.md) - Show an action module
- [auth0 actions modules update](auth0_actions_modules_update.md) - Update an action module
- [auth0 actions modules versions](auth0_actions_modules_versions.md) - Manage action module versions


