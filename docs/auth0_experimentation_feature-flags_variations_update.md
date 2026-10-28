---
layout: default
parent: auth0 experimentation feature-flags variations
has_toc: false
---
# auth0 experimentation feature-flags variations update

Update a variation.

To update interactively, use `auth0 experimentation feature-flags variations update` with no arguments.

To update non-interactively, supply the IDs and fields to change through the flags.

Use '--schema' to print the request payload schema and exit.
Use '--data' to supply the full JSON payload (validated against the schema before sending).

## Usage
```
auth0 experimentation feature-flags variations update [flags]
```

## Examples

```
  auth0 experimentation feature-flags variations update
  auth0 experimentation feature-flags variations update <feature-flag-id> <variation-id>
  auth0 experimentation feature-flags variations update <feature-flag-id> <variation-id> --name "new-name"

  # Discover the payload schema
  auth0 experimentation feature-flags variations update --schema
  auth0 experimentation feature-flags variations update --schema --json

  # JSON input mode (for agents and automation)
  auth0 experimentation feature-flags variations update <feature-flag-id> <variation-id> --data '{"name":"new-name"}'
  auth0 experimentation feature-flags variations update <feature-flag-id> <variation-id> --data @variation.json
```


## Flags

```
      --data string          JSON payload for the operation, as a JSON string or file path (@file.json). Can also be piped via stdin.
  -d, --description string   Description of the variation.
      --json                 Output in json format.
      --json-compact         Output in compact json format.
  -n, --name string          Name of the variation.
  -o, --overrides string     Parameter overrides as JSON mapping each parameter name directly to its override value. Example: '{"color":"red"}'
      --schema               Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
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


