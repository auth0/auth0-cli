---
layout: default
parent: auth0 experimentation experiments
has_toc: false
---
# auth0 experimentation experiments create

Create a new experiment.

To create interactively, use `auth0 experimentation experiments create` with no flags.

To create non-interactively, supply all required flags.

The allocation strategy is required and cannot be changed after creation. The allocations are optional: omit them to create a draft and add them later with `auth0 experimentation experiments update`.

Use '--schema' to print the request payload schema and exit.
Use '--data' to supply the full JSON payload (validated against the schema before sending).

## Usage
```
auth0 experimentation experiments create [flags]
```

## Examples

```
  auth0 experimentation experiments create
  auth0 experimentation experiments create --name "draft-test" --feature-flag-id ff_abc --authentication-flow authentication --allocation-strategy percentage
  auth0 experimentation experiments create --name "button-color" --feature-flag-id ff_abc --authentication-flow authentication --allocation-strategy percentage --allocations '[{"variation_id":"vid_1","weight":50,"is_control":true},{"variation_id":"vid_2","weight":50,"is_control":false}]'
  auth0 experimentation experiments create -n "gradual-rollout" -f ff_abc -a authentication -s percentage --levels 10,25,50,100 --allocations '[{"variation_id":"vid_1","is_control":true},{"variation_id":"vid_2","is_control":false}]'
  auth0 experimentation experiments create -n "by-region" -f ff_abc -a authentication -s segment --allocations '[{"variation_id":"vid_1","is_control":true,"is_fallback":true},{"variation_id":"vid_2","segment_id":"seg_abc","priority":1,"is_control":false}]'

  # Discover the payload schema
  auth0 experimentation experiments create --schema
  auth0 experimentation experiments create --schema --json

  # JSON input mode (for agents and automation)
  auth0 experimentation experiments create --data '{"name":"draft-test","feature_flag_id":"ff_abc","authentication_flow":"authentication","allocation_strategy":"percentage"}'
  auth0 experimentation experiments create --data @experiment.json
  cat experiment.json | auth0 experimentation experiments create
```


## Flags

```
  -s, --allocation-strategy string   Allocation strategy: percentage or segment.
  -A, --allocations string           JSON array of allocation items ({variation_id, weight, is_control} for percentage, where weight is an integer percentage from 0 to 100; {variation_id, segment_id, priority, is_control, is_fallback} for segment).
  -a, --authentication-flow string   Authentication flow this experiment applies to: authentication, mfa_enrollment, mfa_challenge, password_reset, passkey_enrollment, or all.
      --data string                  JSON payload for the operation, as a JSON string or file path (@file.json). Can also be piped via stdin.
      --default-config string        For Auth0-managed flags only: where non-overridden config keys resolve from: 'tenant' (inherit the tenant's live config) or 'flag' (use the flag's frozen defaults). Rejected for customer-defined flags.
  -d, --description string           Description of the experiment.
  -f, --feature-flag-id string       ID of the feature flag to experiment on.
      --json                         Output in json format.
      --json-compact                 Output in compact json format.
      --levels string                Ramp exposure levels as a comma-separated, strictly-increasing list of integer percentages in [0, 100] (e.g. 10,25,50,100). Requires --allocation-strategy percentage, and the allocations must omit weights. Traffic is controlled by the level schedule, not per-variation weights.
  -n, --name string                  Name of the experiment.
      --schema                       Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
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


