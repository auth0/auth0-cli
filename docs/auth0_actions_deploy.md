---
layout: default
parent: auth0 actions
has_toc: false
---
# auth0 actions deploy

Before an action can be bound to a flow, the action must be deployed.

The selected action will be deployed and added to the collection of available actions for flows. Additionally, a new draft version of the deployed action will be created for future editing. Because secrets and dependencies are tied to versions, any saved secrets or dependencies will be available to the new draft.

## Usage
```
auth0 actions deploy [flags]
```

## Examples

```
  auth0 actions deploy
  auth0 actions deploy <action-id>
  auth0 actions deploy <action-id> --json
  auth0 actions deploy <action-id> --json-compact
```


## Flags

```
      --json           Output in json format.
      --json-compact   Output in compact json format.
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


