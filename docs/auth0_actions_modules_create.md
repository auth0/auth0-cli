---
layout: default
parent: auth0 actions modules
has_toc: false
---
# auth0 actions modules create

Create a new action module.

To create interactively, use `auth0 actions modules create` with no flags.

To create non-interactively, supply the name and code (and optionally dependencies, secrets, and publish) through flags.

## Usage
```
auth0 actions modules create [flags]
```

## Examples

```
  auth0 actions modules create
  auth0 actions modules create --name mymodule --code "$(cat path/to/module.js)"
  auth0 actions modules create --name mymodule --code "$(cat path/to/module.js)" --publish
  auth0 actions modules create --name mymodule --code "$(cat path/to/module.js)" --dependency "lodash=4.0.0" --secret "API_KEY=value"
  auth0 actions modules create --name mymodule --code "$(cat path/to/module.js)" --api-version v1
  auth0 actions modules create -n mymodule -c "$(cat path/to/module.js)" -d "lodash=4.0.0" -s "API_KEY=value" --json

  # Discover the payload schema
  auth0 actions modules create --schema
  auth0 actions modules create --schema --json

  # JSON input mode (for agents and automation)
  auth0 actions modules create --data '{"name":"mymodule","code":"module.exports = {};"}'
  auth0 actions modules create --data @module.json
  cat module.json | auth0 actions modules create
```


## Flags

```
      --api-version string          API version of the action module.
  -c, --code string                 Code content of the action module.
      --data string                 JSON payload for the operation, as a JSON string or file path (@file.json). Can also be piped via stdin.
  -d, --dependency stringToString   Third party npm module, and its version, that the action module depends on. (default [])
      --json                        Output in json format.
      --json-compact                Output in compact json format.
  -n, --name string                 Name of the action module. Must start with a lowercase letter or digit and contain only lowercase letters, digits, underscores, and hyphens.
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


