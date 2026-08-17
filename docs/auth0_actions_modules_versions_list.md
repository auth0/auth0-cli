---
layout: default
parent: auth0 actions modules versions
has_toc: false
---
# auth0 actions modules versions list

List the immutable versions that have been published for an action module.

## Usage
```
auth0 actions modules versions list [flags]
```

## Examples

```
  auth0 actions modules versions list
  auth0 actions modules versions ls <module-id>
  auth0 actions modules versions list <module-id> --number 100
  auth0 actions modules versions list <module-id> --json
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

- [auth0 actions modules versions list](auth0_actions_modules_versions_list.md) - List the versions of an action module
- [auth0 actions modules versions publish](auth0_actions_modules_versions_publish.md) - Publish an action module draft as a new version
- [auth0 actions modules versions rollback](auth0_actions_modules_versions_rollback.md) - Roll an action module back to a previous version
- [auth0 actions modules versions show](auth0_actions_modules_versions_show.md) - Show an action module version


