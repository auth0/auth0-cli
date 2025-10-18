---
layout: default
parent: auth0 actions
has_toc: false
---
# auth0 actions delete

Delete an action.

To delete interactively, use `auth0 actions delete` with no arguments.

To delete non-interactively, supply the action id and the `--force` flag to skip confirmation.

## Usage
```
auth0 actions delete [flags]
```

## Examples

```
  auth0 actions delete
  auth0 actions rm
  auth0 actions delete <action-id>
  auth0 actions delete <action-id> --force
  auth0 actions delete <action-id> <action-id2> <action-idn>
  auth0 actions delete <action-id> <action-id2> <action-idn> --force
```


## Flags

```
      --force   Skip confirmation.
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


