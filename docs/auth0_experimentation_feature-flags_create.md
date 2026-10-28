---
layout: default
parent: auth0 experimentation feature-flags
has_toc: false
---
# auth0 experimentation feature-flags create

Create a new feature flag.

To create interactively, use `auth0 experimentation feature-flags create` with no flags.

To create non-interactively, supply name and parameters through the flags.

Use '--schema' to print the request payload schema and exit.
Use '--data' to supply the full JSON payload (validated against the schema before sending).

## Usage
```
auth0 experimentation feature-flags create [flags]
```

## Examples

```
  auth0 experimentation feature-flags create
  auth0 experimentation feature-flags create --name "dark-mode" --parameters '{"enabled":{"type":"boolean","value":false}}'
  auth0 experimentation feature-flags create -n "checkout-flow" -p '{"variant":{"type":"string","value":"control"}}'

  # Discover the payload schema
  auth0 experimentation feature-flags create --schema
  auth0 experimentation feature-flags create --schema --json

  # JSON input mode (for agents and automation)
  auth0 experimentation feature-flags create --data '{"name":"dark-mode","parameters":{"enabled":{"type":"boolean","value":false}}}'
  auth0 experimentation feature-flags create --data @feature-flag.json
  cat feature-flag.json | auth0 experimentation feature-flags create
```


## Flags

```
      --data string          JSON payload for the operation, as a JSON string or file path (@file.json). Can also be piped via stdin.
  -d, --description string   Description of the feature flag.
      --json                 Output in json format.
      --json-compact         Output in compact json format.
  -n, --name string          Name of the feature flag.
  -p, --parameters string    Parameters schema as JSON. Example: '{"color":{"type":"string","value":"blue"}}'
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

- [auth0 experimentation feature-flags create](auth0_experimentation_feature-flags_create.md) - Create a new feature flag
- [auth0 experimentation feature-flags delete](auth0_experimentation_feature-flags_delete.md) - Delete a feature flag
- [auth0 experimentation feature-flags list](auth0_experimentation_feature-flags_list.md) - List your feature flags
- [auth0 experimentation feature-flags show](auth0_experimentation_feature-flags_show.md) - Show a feature flag
- [auth0 experimentation feature-flags update](auth0_experimentation_feature-flags_update.md) - Update a feature flag
- [auth0 experimentation feature-flags variations](auth0_experimentation_feature-flags_variations.md) - Manage variations of a feature flag


