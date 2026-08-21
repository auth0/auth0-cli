---
layout: default
parent: auth0 actions modules
has_toc: false
---
# auth0 actions modules delete

Delete an action module.

To delete interactively, use `auth0 actions modules delete` with no arguments.

To delete non-interactively, supply the module id and the `--force` flag to skip confirmation.

A module that is in use by a deployed action version cannot be deleted; such modules are hidden from the interactive picker.

## Usage
```
auth0 actions modules delete [flags]
```

## Examples

```
  auth0 actions modules delete
  auth0 actions modules rm
  auth0 actions modules delete <module-id>
  auth0 actions modules delete <module-id> --force
  auth0 actions modules delete <module-id> <module-id2> <module-idn>
  auth0 actions modules delete <module-id> <module-id2> <module-idn> --force
```


## Flags

```
      --force   Skip confirmation.
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


