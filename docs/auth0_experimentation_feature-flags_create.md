---
layout: default
parent: auth0 experimentation feature-flags
has_toc: false
---
# auth0 experimentation feature-flags create

Create a new feature flag.

To create interactively, use `auth0 experimentation feature-flags create` with no flags.

To create non-interactively, supply name and parameters through the flags.

## Usage
```
auth0 experimentation feature-flags create [flags]
```

## Examples

```
  auth0 experimentation feature-flags create
  auth0 experimentation feature-flags create --name "dark-mode" --parameters '{"enabled":{"type":"boolean","value":false}}'
  auth0 experimentation feature-flags create -n "checkout-flow" -p '{"variant":{"type":"string","value":"control"}}'
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

- [auth0 experimentation feature-flags create](auth0_experimentation_feature-flags_create.md) - Create a new feature flag
- [auth0 experimentation feature-flags delete](auth0_experimentation_feature-flags_delete.md) - Delete a feature flag
- [auth0 experimentation feature-flags list](auth0_experimentation_feature-flags_list.md) - List your feature flags
- [auth0 experimentation feature-flags show](auth0_experimentation_feature-flags_show.md) - Show a feature flag
- [auth0 experimentation feature-flags status](auth0_experimentation_feature-flags_status.md) - Change a feature flag's status
- [auth0 experimentation feature-flags update](auth0_experimentation_feature-flags_update.md) - Update a feature flag
- [auth0 experimentation feature-flags variations](auth0_experimentation_feature-flags_variations.md) - Manage variations of a feature flag


