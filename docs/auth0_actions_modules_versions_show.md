---
layout: default
parent: auth0 actions modules versions
has_toc: false
---
# auth0 actions modules versions show

Display the code, dependencies, and secrets captured by a specific action module version.

## Usage
```
auth0 actions modules versions show [flags]
```

## Examples

```
  auth0 actions modules versions show
  auth0 actions modules versions show <module-id> <version-id>
  auth0 actions modules versions show <module-id> <version-id> --json
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

- [auth0 actions modules versions list](auth0_actions_modules_versions_list.md) - List the versions of an action module
- [auth0 actions modules versions publish](auth0_actions_modules_versions_publish.md) - Publish an action module draft as a new version
- [auth0 actions modules versions rollback](auth0_actions_modules_versions_rollback.md) - Roll an action module back to a previous version
- [auth0 actions modules versions show](auth0_actions_modules_versions_show.md) - Show an action module version


