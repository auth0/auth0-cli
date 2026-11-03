---
layout: default
parent: auth0 experimentation experiments
has_toc: false
---
# auth0 experimentation experiments list

List all experiments. To create one, run: `auth0 experimentation experiments create`.

Use '--schema' to see available query parameters.
Use '--query' to filter results via a JSON object (any API-supported parameter works immediately).

## Usage
```
auth0 experimentation experiments list [flags]
```

## Examples

```
  auth0 experimentation experiments list
  auth0 experimentation experiments ls
  auth0 experimentation experiments list --json
  auth0 experimentation experiments list --status active
  auth0 experimentation experiments list --feature-flag-id <id>
  auth0 experimentation experiments list --schema
  auth0 experimentation experiments list --schema --json
  auth0 experimentation experiments list --query '{"status":"active","take":10}'
  auth0 experimentation experiments list --query '{"status":"active"}' --json
```


## Flags

```
      --authentication-flow string   Filter by authentication flow.
      --csv                          Output in csv format.
      --feature-flag-id string       Filter by feature flag ID.
      --json                         Output in json format.
      --json-compact                 Output in compact json format.
  -q, --query string                 Filter results with a JSON object of query parameters. Any API-supported parameter works immediately. Run '--schema' to see documented parameters. On offset-paginated endpoints, add "include_totals":true to receive total counts and a pagination hint (without it the API returns a bare array and no hint can be given).
      --schema                       Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
      --status string                Filter by status (draft, active, paused, completed, archived).
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

- [auth0 experimentation experiments advance-ramp](auth0_experimentation_experiments_advance-ramp.md) - Advance a ramp experiment to its next level
- [auth0 experimentation experiments create](auth0_experimentation_experiments_create.md) - Create a new experiment
- [auth0 experimentation experiments delete](auth0_experimentation_experiments_delete.md) - Delete an experiment
- [auth0 experimentation experiments list](auth0_experimentation_experiments_list.md) - List your experiments
- [auth0 experimentation experiments show](auth0_experimentation_experiments_show.md) - Show an experiment
- [auth0 experimentation experiments update](auth0_experimentation_experiments_update.md) - Update an experiment
- [auth0 experimentation experiments validate](auth0_experimentation_experiments_validate.md) - Validate an experiment


