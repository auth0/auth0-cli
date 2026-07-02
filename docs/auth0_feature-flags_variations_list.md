---
layout: default
parent: auth0 feature-flags variations
has_toc: false
---
# auth0 feature-flags variations list

List all variations for a given feature flag.

## Usage
```
auth0 feature-flags variations list [flags]
```

## Examples

```
  auth0 feature-flags variations list
  auth0 feature-flags variations list <feature-flag-id>
  auth0 feature-flags variations list <feature-flag-id> --json
```


## Flags

```
      --csv            Output in csv format.
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

- [auth0 feature-flags variations create](auth0_feature-flags_variations_create.md) - Create a new variation
- [auth0 feature-flags variations delete](auth0_feature-flags_variations_delete.md) - Delete a variation
- [auth0 feature-flags variations list](auth0_feature-flags_variations_list.md) - List variations of a feature flag
- [auth0 feature-flags variations show](auth0_feature-flags_variations_show.md) - Show a variation
- [auth0 feature-flags variations update](auth0_feature-flags_variations_update.md) - Update a variation


