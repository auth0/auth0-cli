---
layout: default
parent: auth0 feature-flags
has_toc: false
---
# auth0 feature-flags update

Update a feature flag.

To update interactively, use `auth0 feature-flags update` with no arguments.

To update non-interactively, supply the feature flag ID and fields to change through the flags.

## Usage
```
auth0 feature-flags update [flags]
```

## Examples

```
  auth0 feature-flags update
  auth0 feature-flags update <feature-flag-id>
  auth0 feature-flags update <feature-flag-id> --name "new-name"
  auth0 feature-flags update <feature-flag-id> --parameters '{"enabled":{"type":"boolean","value":true}}'
```


## Flags

```
  -d, --description string   Description of the feature flag.
      --json                 Output in json format.
      --json-compact         Output in compact json format.
  -n, --name string          Name of the feature flag.
  -p, --parameters string    Parameters schema as JSON. Example: '{"color":{"type":"string","value":"blue"}}'
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


