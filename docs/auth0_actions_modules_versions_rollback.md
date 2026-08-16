---
layout: default
parent: auth0 actions modules versions
has_toc: false
---
# auth0 actions modules versions rollback

Copy the code, dependencies, and secrets of a previously published version back into the module's draft.

This does not create a new version; it only replaces the draft. Publish afterwards to snapshot the restored draft as a new version.

## Usage
```
auth0 actions modules versions rollback [flags]
```

## Examples

```
  auth0 actions modules versions rollback
  auth0 actions modules versions rollback <module-id> <version-id>
  auth0 actions modules versions rollback <module-id> <version-id> --force
```


## Flags

```
      --force          Skip confirmation.
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


