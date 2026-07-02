---
layout: default
parent: auth0 experimentation feature-flags
has_toc: false
---
# auth0 experimentation feature-flags list

List all feature flags. To create one, run: `auth0 experimentation feature-flags create`.

## Usage
```
auth0 experimentation feature-flags list [flags]
```

## Examples

```
  auth0 experimentation feature-flags list
  auth0 experimentation feature-flags ls
  auth0 experimentation feature-flags list --json
  auth0 experimentation feature-flags list --status active
```


## Flags

```
      --csv             Output in csv format.
      --json            Output in json format.
      --json-compact    Output in compact json format.
      --status string   Filter by status (draft, active, archived).
      --type string     Filter by type (auth0, self).
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 experimentation feature-flags create](auth0_experimentation_feature-flags_create.md) - Create a new feature flag
- [auth0 experimentation feature-flags delete](auth0_experimentation_feature-flags_delete.md) - Delete a feature flag
- [auth0 experimentation feature-flags list](auth0_experimentation_feature-flags_list.md) - List your feature flags
- [auth0 experimentation feature-flags show](auth0_experimentation_feature-flags_show.md) - Show a feature flag
- [auth0 experimentation feature-flags status](auth0_experimentation_feature-flags_status.md) - Change a feature flag's status
- [auth0 experimentation feature-flags update](auth0_experimentation_feature-flags_update.md) - Update a feature flag
- [auth0 experimentation feature-flags variations](auth0_experimentation_feature-flags_variations.md) - Manage variations of a feature flag


