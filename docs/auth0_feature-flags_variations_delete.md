---
layout: default
parent: auth0 feature-flags variations
has_toc: false
---
# auth0 feature-flags variations delete

Delete a variation.

To delete interactively, use `auth0 feature-flags variations delete` with no arguments.

## Usage
```
auth0 feature-flags variations delete [flags]
```

## Examples

```
  auth0 feature-flags variations delete
  auth0 feature-flags variations delete <feature-flag-id> <variation-id>
  auth0 feature-flags variations delete <feature-flag-id> <variation-id> --force
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

- [auth0 feature-flags variations create](auth0_feature-flags_variations_create.md) - Create a new variation
- [auth0 feature-flags variations delete](auth0_feature-flags_variations_delete.md) - Delete a variation
- [auth0 feature-flags variations list](auth0_feature-flags_variations_list.md) - List variations of a feature flag
- [auth0 feature-flags variations show](auth0_feature-flags_variations_show.md) - Show a variation
- [auth0 feature-flags variations update](auth0_feature-flags_variations_update.md) - Update a variation


