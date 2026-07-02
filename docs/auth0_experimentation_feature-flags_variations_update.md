---
layout: default
parent: auth0 experimentation feature-flags variations
has_toc: false
---
# auth0 experimentation feature-flags variations update

Update a variation.

To update interactively, use `auth0 experimentation feature-flags variations update` with no arguments.

To update non-interactively, supply the IDs and fields to change through the flags.

## Usage
```
auth0 experimentation feature-flags variations update [flags]
```

## Examples

```
  auth0 experimentation feature-flags variations update
  auth0 experimentation feature-flags variations update <feature-flag-id> <variation-id>
  auth0 experimentation feature-flags variations update <feature-flag-id> <variation-id> --name "new-name"
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

- [auth0 experimentation feature-flags variations create](auth0_experimentation_feature-flags_variations_create.md) - Create a new variation
- [auth0 experimentation feature-flags variations delete](auth0_experimentation_feature-flags_variations_delete.md) - Delete a variation
- [auth0 experimentation feature-flags variations list](auth0_experimentation_feature-flags_variations_list.md) - List variations of a feature flag
- [auth0 experimentation feature-flags variations show](auth0_experimentation_feature-flags_variations_show.md) - Show a variation
- [auth0 experimentation feature-flags variations update](auth0_experimentation_feature-flags_variations_update.md) - Update a variation


