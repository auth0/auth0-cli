---
layout: default
parent: auth0 actions
has_toc: false
---
# auth0 actions update

Update an action.

To update interactively, use `auth0 actions update` with no arguments.

To update non-interactively, supply the action id, name, code, secrets and dependencies through the flags.

## Usage
```
auth0 actions update [flags]
```

## Examples

```
  auth0 actions update <action-id>
  auth0 actions update <action-id> --runtime node18
  auth0 actions update <action-id> --name myaction --runtime node18
  auth0 actions update <action-id> --name myaction --code "$(cat path/to/code.js) --r node18"
  auth0 actions update <action-id> --name myaction --code "$(cat path/to/code.js)" --dependency "lodash=4.0.0"
  auth0 actions update <action-id> --name myaction --code "$(cat path/to/code.js)" --dependency "lodash=4.0.0" --secret "SECRET=value"
  auth0 actions update <action-id> --name myaction --code "$(cat path/to/code.js)" --dependency "lodash=4.0.0" --dependency "uuid=9.0.0" --secret "API_KEY=value" --secret "SECRET=value"
  auth0 actions update <action-id> -n myaction -c "$(cat path/to/code.js)" -r node18 -d "lodash=4.0.0" -d "uuid=9.0.0" -s "API_KEY=value" -s "SECRET=value" --json
  auth0 actions update <action-id> -n myaction -c "$(cat path/to/code.js)" -r node18 -d "lodash=4.0.0" -d "uuid=9.0.0" -s "API_KEY=value" -s "SECRET=value" --json-compact
```


## Flags

```
  -c, --code string                 Code content for the action.
  -d, --dependency stringToString   Third party npm module, and its version, that the action depends on. (default [])
      --force                       Skip confirmation.
      --json                        Output in json format.
      --json-compact                Output in compact json format.
  -n, --name string                 Name of the action.
  -r, --runtime string              Runtime to be used in the action.  Possible values are: node22(recommended), node18, node16, node12
  -s, --secret stringToString       Secrets to be used in the action. (default [])
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
- [auth0 actions diff](auth0_actions_diff.md) - Show diff between two versions of an Actions
- [auth0 actions list](auth0_actions_list.md) - List your actions
- [auth0 actions open](auth0_actions_open.md) - Open the settings page of an action
- [auth0 actions show](auth0_actions_show.md) - Show an action
- [auth0 actions update](auth0_actions_update.md) - Update an action


