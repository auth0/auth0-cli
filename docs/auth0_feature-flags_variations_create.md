---
layout: default
parent: auth0 feature-flags variations
has_toc: false
---
# auth0 feature-flags variations create

Create a new variation for a feature flag.

To create interactively, use `auth0 feature-flags variations create` with no flags.

To create non-interactively, supply the feature flag ID, name, and overrides through the flags.

## Usage
```
auth0 feature-flags variations create [flags]
```

## Examples

```
  auth0 feature-flags variations create
  auth0 feature-flags variations create <feature-flag-id>
  auth0 feature-flags variations create <feature-flag-id> --name "treatment" --overrides '{"color":{"value":"red"}}'
```


## Flags

```
  -d, --description string   Description of the variation.
      --json                 Output in json format.
      --json-compact         Output in compact json format.
  -n, --name string          Name of the variation.
  -o, --overrides string     Parameter overrides as JSON. Example: '{"color":{"value":"red"}}'
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 feature-flags variations create](auth0_feature-flags_variations_create.md) - Create a new variation
- [auth0 feature-flags variations delete](auth0_feature-flags_variations_delete.md) - Delete a variation
- [auth0 feature-flags variations list](auth0_feature-flags_variations_list.md) - List variations of a feature flag
- [auth0 feature-flags variations show](auth0_feature-flags_variations_show.md) - Show a variation
- [auth0 feature-flags variations update](auth0_feature-flags_variations_update.md) - Update a variation


