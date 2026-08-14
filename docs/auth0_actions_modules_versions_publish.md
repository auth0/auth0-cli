---
layout: default
parent: auth0 actions modules versions
has_toc: false
---
# auth0 actions modules versions publish

Snapshot an action module's current draft as a new immutable version.

This is equivalent to the `--publish` flag on `auth0 actions modules create` and `auth0 actions modules update`, but lets you publish a draft on its own without making any other change.

## Usage
```
auth0 actions modules versions publish [flags]
```

## Examples

```
  auth0 actions modules versions publish
  auth0 actions modules versions publish <module-id>
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


