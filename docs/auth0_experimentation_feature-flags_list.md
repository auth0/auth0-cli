---
layout: default
parent: auth0 experimentation feature-flags
has_toc: false
---
# auth0 experimentation feature-flags list

List all feature flags. To create one, run: `auth0 experimentation feature-flags create`.

Use '--schema' to see available query parameters.
Use '--query' to filter results via a JSON object (any API-supported parameter works immediately).

## Usage
```
auth0 experimentation feature-flags list [flags]
```

## Examples

```
  auth0 experimentation feature-flags list
  auth0 experimentation feature-flags ls
  auth0 experimentation feature-flags list --json
  auth0 experimentation feature-flags list --status active
  auth0 experimentation feature-flags list --schema
  auth0 experimentation feature-flags list --schema --json
  auth0 experimentation feature-flags list --query '{"status":"active","take":10}'
  auth0 experimentation feature-flags list --query '{"type":"self"}' --json
```


## Flags

```
      --csv             Output in csv format.
      --json            Output in json format.
      --json-compact    Output in compact json format.
  -q, --query string    Filter results with a JSON object of query parameters. Any API-supported parameter works immediately. Run '--schema' to see documented parameters. On offset-paginated endpoints, add "include_totals":true to receive total counts and a pagination hint (without it the API returns a bare array and no hint can be given).
      --schema          Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
      --status string   Filter by status (draft, active, archived).
      --type string     Filter by type (auth0, self).
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


