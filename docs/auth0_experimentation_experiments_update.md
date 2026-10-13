---
layout: default
parent: auth0 experimentation experiments
has_toc: false
---
# auth0 experimentation experiments update

Update an experiment, including transitioning its status with --status (active, paused, completed, archived).

Note: the feature flag and allocation strategy cannot be changed after creation, and authentication flow, allocations, default config, and ramp levels can only be changed while the experiment is a draft.

To update interactively, use `auth0 experimentation experiments update` with no arguments.

Use '--schema' to print the request payload schema and exit.
Use '--data' to supply the full JSON payload (validated against the schema before sending).

## Usage
```
auth0 experimentation experiments update [flags]
```

## Examples

```
  auth0 experimentation experiments update
  auth0 experimentation experiments update <experiment-id>
  auth0 experimentation experiments update <experiment-id> --name "new-name"
  auth0 experimentation experiments update <experiment-id> --levels 10,50,100
  auth0 experimentation experiments update <experiment-id> --allocations '[{"variation_id":"vid","weight":100,"is_control":true}]'
  auth0 experimentation experiments update <experiment-id> --status active

  # Discover the payload schema
  auth0 experimentation experiments update --schema
  auth0 experimentation experiments update --schema --json

  # JSON input mode (for agents and automation)
  auth0 experimentation experiments update <experiment-id> --data '{"status":"active"}'
  auth0 experimentation experiments update <experiment-id> --data @experiment.json
  cat experiment.json | auth0 experimentation experiments update <experiment-id>
```


## Flags

```
  -A, --allocations string           JSON array of allocation items ({variation_id, weight, is_control} for percentage, where weight is an integer percentage from 1 to 100; {variation_id, segment_id, priority, is_control, is_fallback} for segment).
  -a, --authentication-flow string   Authentication flow this experiment applies to: authentication, mfa_enrollment, mfa_challenge, password_reset, passkey_enrollment, or all.
      --data string                  JSON payload for the operation, as a JSON string or file path (@file.json). Can also be piped via stdin.
      --default-config string        For Auth0-managed flags only: where non-overridden config keys resolve from — 'tenant' (inherit the tenant's live config) or 'flag' (use the flag's frozen defaults). Rejected for customer-defined flags.
  -d, --description string           Description of the experiment.
      --json                         Output in json format.
      --json-compact                 Output in compact json format.
      --levels string                Ramp exposure levels as a comma-separated, strictly-increasing list of integer percentages in [0, 100] (e.g. 10,25,50,100). Requires --allocation-strategy percentage, and the allocations must omit weights — traffic is controlled by the level schedule, not per-variation weights.
  -n, --name string                  Name of the experiment.
      --schema                       Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
      --status string                Transition the experiment to a new status (active, paused, completed, archived).
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


