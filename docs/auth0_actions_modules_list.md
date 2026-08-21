---
layout: default
parent: auth0 actions modules
has_toc: false
---
# auth0 actions modules list

List the action modules in your tenant.

## Usage
```
auth0 actions modules list [flags]
```

## Examples

```
  auth0 actions modules list
  auth0 actions modules ls
  auth0 actions modules list --number 100
  auth0 actions modules list -n 100 --json
  auth0 actions modules list --csv
```


## Flags

```
      --csv            Output in csv format.
      --json           Output in json format.
      --json-compact   Output in compact json format.
  -n, --number int     Number of action modules to retrieve. Minimum 1, maximum 1000. (default 100)
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


