---
layout: default
parent: auth0 actions modules actions
has_toc: false
---
# auth0 actions modules actions list

List the actions that import an action module, along with the module version each action is using.

## Usage
```
auth0 actions modules actions list [flags]
```

## Examples

```
  auth0 actions modules actions list
  auth0 actions modules actions ls <module-id>
  auth0 actions modules actions list <module-id> --number 100
  auth0 actions modules actions list <module-id> --json
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

- [auth0 actions modules actions list](auth0_actions_modules_actions_list.md) - List the actions using an action module


