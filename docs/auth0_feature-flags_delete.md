---
layout: default
parent: auth0 feature-flags
has_toc: false
---
# auth0 feature-flags delete

Delete a feature flag.

To delete interactively, use `auth0 feature-flags delete` with no arguments.

To delete non-interactively, supply the feature flag ID and use `--force` to skip confirmation.

## Usage
```
auth0 feature-flags delete [flags]
```

## Examples

```
  auth0 feature-flags delete
  auth0 feature-flags delete <feature-flag-id>
  auth0 feature-flags delete <feature-flag-id> --force
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

- [auth0 feature-flags create](auth0_feature-flags_create.md) - Create a new feature flag
- [auth0 feature-flags delete](auth0_feature-flags_delete.md) - Delete a feature flag
- [auth0 feature-flags list](auth0_feature-flags_list.md) - List your feature flags
- [auth0 feature-flags show](auth0_feature-flags_show.md) - Show a feature flag
- [auth0 feature-flags status](auth0_feature-flags_status.md) - Change a feature flag's status
- [auth0 feature-flags update](auth0_feature-flags_update.md) - Update a feature flag
- [auth0 feature-flags variations](auth0_feature-flags_variations.md) - Manage variations of a feature flag


