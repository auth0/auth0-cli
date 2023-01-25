---
layout: default
parent: auth0 actions
has_toc: false
---
# auth0 actions create

Create a new action.

To create interactively, use `auth0 actions create` with no flags.

To create non-interactively, supply the action name, trigger, code, secrets and dependencies through the flags.

## Usage
```
auth0 actions create [flags]
```

## Examples

```
  auth0 actions create
  auth0 actions create --name myaction
  auth0 actions create --name myaction --trigger post-login
  auth0 actions create --name myaction --trigger post-login --code "$(cat path/to/code.js)"
  auth0 actions create --name myaction --trigger post-login --code "$(cat path/to/code.js)" --dependency "lodash=4.0.0"
  auth0 actions create --name myaction --trigger post-login --code "$(cat path/to/code.js)" --dependency "lodash=4.0.0" --secret "SECRET=value"
  auth0 actions create --name myaction --trigger post-login --code "$(cat path/to/code.js)" --dependency "lodash=4.0.0" --dependency "uuid=9.0.0" --secret "API_KEY=value" --secret "SECRET=value"
  auth0 actions create -n myaction -t post-login -c "$(cat path/to/code.js)" -d "lodash=4.0.0" -d "uuid=9.0.0" -s "API_KEY=value" -s "SECRET=value" --json
```


## Flags

```
  -c, --code string                 Code content for the action.
  -d, --dependency stringToString   Third party npm module, and its version, that the action depends on. (default [])
      --json                        Output in json format.
  -n, --name string                 Name of the action.
  -s, --secret stringToString       Secrets to be used in the action. (default [])
  -t, --trigger string              Trigger of the action. At this time, an action can only target a single trigger at a time.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 actions create](auth0_actions_create.md) - Create a new action
- [auth0 actions delete](auth0_actions_delete.md) - Delete an action
- [auth0 actions deploy](auth0_actions_deploy.md) - Deploy an action
- [auth0 actions list](auth0_actions_list.md) - List your actions
- [auth0 actions open](auth0_actions_open.md) - Open the settings page of an action
- [auth0 actions show](auth0_actions_show.md) - Show an action
- [auth0 actions update](auth0_actions_update.md) - Update an action


