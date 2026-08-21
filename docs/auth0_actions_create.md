---
layout: default
parent: auth0 actions
has_toc: false
---
# auth0 actions create

Create a new action.

To create interactively, use 'auth0 actions create' with no flags.

To create non-interactively, supply the action name, trigger, code, secrets and dependencies through the flags.

## JSON Input (for agents and automation)

Use '--schema' to print the request payload schema, then '--data' to provide
action data as JSON:
  - Inline JSON: --data '{"name":"my-action",...}'
  - From file: --data @action.json
  - From stdin: pipe data in (e.g. cat action.json | auth0 actions create), or --data -

The JSON is validated against the OpenAPI schema before sending to the API.

## Usage
```
auth0 actions create [flags]
```

## Examples

```
  # Interactive mode
  auth0 actions create

  # Flag-based mode
  auth0 actions create --name myaction --trigger post-login
  auth0 actions create --name myaction --trigger post-login --code "$(cat path/to/code.js)" --module "module_id=mod_123,module_version_id=ver_456"
  auth0 actions create -n myaction -t post-login -c "$(cat path/to/code.js)" -r node18 --json
  auth0 actions create -n myaction -t post-login -c "$(cat path/to/code.js)" -d "lodash=4.0.0" -s "API_KEY=value" --json-compact

  # Discover the payload schema (add --json for machine-readable output)
  auth0 actions create --schema
  auth0 actions create --schema --json

  # JSON input mode (for agents and automation)
  auth0 actions create --data '{"name":"my-action","supported_triggers":[{"id":"post-login","version":"v3"}]}'
  auth0 actions create --data @action.json
  cat action.json | auth0 actions create
  auth0 actions create --data @action.json --json
```


## Flags

```
  -c, --code string                 Code content for the action.
      --data string                 JSON payload for the operation. Can be a JSON string, file path (@file.json), or '-' for stdin.
  -d, --dependency stringToString   Third party npm module, and its version, that the action depends on. (default [])
      --json                        Output in json format.
      --json-compact                Output in compact json format.
  -m, --module stringArray          Action module to associate with the action, as comma-separated key=value pairs matching the API fields: module_id and module_version_id (both required, UUIDs). Can be passed multiple times to associate several modules.
  -n, --name string                 Name of the action.
  -r, --runtime string              Runtime to be used in the action.  Possible values are: node22(recommended), node18, node16, node12
      --schema                      Print the request payload schema for this command and exit. Use with --json for machine-readable output.
  -s, --secret stringToString       Secrets to be used in the action. (default [])
  -t, --trigger string              Trigger of the action. At this time, an action can only target a single trigger at a time.
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

- [auth0 actions create](auth0_actions_create.md) - Create a new action
- [auth0 actions delete](auth0_actions_delete.md) - Delete an action
- [auth0 actions deploy](auth0_actions_deploy.md) - Deploy an action
- [auth0 actions diff](auth0_actions_diff.md) - Show diff between two versions of an Actions
- [auth0 actions list](auth0_actions_list.md) - List your actions
- [auth0 actions modules](auth0_actions_modules.md) - Manage action modules
- [auth0 actions open](auth0_actions_open.md) - Open the settings page of an action
- [auth0 actions show](auth0_actions_show.md) - Show an action
- [auth0 actions update](auth0_actions_update.md) - Update an action


