---
layout: default
parent: auth0 experimentation feature-flags variations
has_toc: false
---
# auth0 experimentation feature-flags variations delete

Delete one or more variations of a feature flag.

To delete interactively, use `auth0 experimentation feature-flags variations delete` with no arguments.

To delete non-interactively, supply the feature flag ID, one or more variation IDs, and `--force` to skip confirmation.

## Usage
```
auth0 experimentation feature-flags variations delete [flags]
```

## Examples

```
  auth0 experimentation feature-flags variations delete
  auth0 experimentation feature-flags variations delete <feature-flag-id>
  auth0 experimentation feature-flags variations delete <feature-flag-id> <variation-id>
  auth0 experimentation feature-flags variations delete <feature-flag-id> <variation-id> --force
  auth0 experimentation feature-flags variations delete <feature-flag-id> <variation-id> <variation-id2> <variation-idn> --force
```


## Flags

```
      --force   Skip confirmation.
```


## Inherited Flags

```
      --agent-mode      Output JSON, disable prompts and colors. Auto-enabled for AI agents; set AUTH0_AGENT_MODE=false to disable.
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 experimentation feature-flags variations create](auth0_experimentation_feature-flags_variations_create.md) - Create a new variation
- [auth0 experimentation feature-flags variations delete](auth0_experimentation_feature-flags_variations_delete.md) - Delete one or more variations
- [auth0 experimentation feature-flags variations list](auth0_experimentation_feature-flags_variations_list.md) - List variations of a feature flag
- [auth0 experimentation feature-flags variations show](auth0_experimentation_feature-flags_variations_show.md) - Show a variation
- [auth0 experimentation feature-flags variations update](auth0_experimentation_feature-flags_variations_update.md) - Update a variation


