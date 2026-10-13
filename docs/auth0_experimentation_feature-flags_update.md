---
layout: default
parent: auth0 experimentation feature-flags
has_toc: false
---
# auth0 experimentation feature-flags update

Update a feature flag.

To update interactively, use `auth0 experimentation feature-flags update` with no arguments.

To update non-interactively, supply the feature flag ID and fields to change through the flags.

Use '--schema' to print the request payload schema and exit.
Use '--data' to supply the full JSON payload (validated against the schema before sending).

## Usage
```
auth0 experimentation feature-flags update [flags]
```

## Examples

```
  auth0 experimentation feature-flags update
  auth0 experimentation feature-flags update <feature-flag-id>
  auth0 experimentation feature-flags update <feature-flag-id> --name "new-name"
  auth0 experimentation feature-flags update <feature-flag-id> --parameters '{"enabled":{"type":"boolean","value":true}}'
  auth0 experimentation feature-flags update <feature-flag-id> --status active

  # Discover the payload schema
  auth0 experimentation feature-flags update --schema
  auth0 experimentation feature-flags update --schema --json

  # JSON input mode (for agents and automation)
  auth0 experimentation feature-flags update <feature-flag-id> --data '{"status":"active"}'
  auth0 experimentation feature-flags update <feature-flag-id> --data @feature-flag.json
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
      --status string        Transition the feature flag to a new status (active, archived). Archiving is irreversible.
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


