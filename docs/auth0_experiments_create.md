---
layout: default
parent: auth0 experiments
has_toc: false
---
# auth0 experiments create

Create a new experiment.

To create interactively, use `auth0 experiments create` with no flags.

To create non-interactively, supply all required flags.

## Usage
```
auth0 experiments create [flags]
```

## Examples

```
  auth0 experiments create
  auth0 experiments create --name "button-color" --feature-flag-id ff_abc --authentication-flow login --allocation-strategy percentage --assignment-config '{"subject":"device"}' --allocations '[{"variation_id":"vid_1","weight":0.5,"is_control":true},{"variation_id":"vid_2","weight":0.5,"is_control":false}]'
```


## Flags

```
  -s, --allocation-strategy string   Allocation strategy: percentage or segment.
  -A, --allocations string           JSON array of allocation items ({variation_id, weight, is_control} for percentage; {variation_id, segment_id, is_control} for segment).
      --assignment-config string     JSON object configuring how users are assigned to variations (e.g. '{"subject":"device"}').
  -a, --authentication-flow string   Authentication flow this experiment applies to (e.g. login, signup).
  -d, --description string           Description of the experiment.
  -f, --feature-flag-id string       ID of the feature flag to experiment on.
      --json                         Output in json format.
      --json-compact                 Output in compact json format.
  -n, --name string                  Name of the experiment.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 experiments archive](auth0_experiments_archive.md) - Archive an experiment
- [auth0 experiments complete](auth0_experiments_complete.md) - Complete an experiment
- [auth0 experiments create](auth0_experiments_create.md) - Create a new experiment
- [auth0 experiments delete](auth0_experiments_delete.md) - Delete an experiment
- [auth0 experiments list](auth0_experiments_list.md) - List your experiments
- [auth0 experiments pause](auth0_experiments_pause.md) - Pause an experiment
- [auth0 experiments show](auth0_experiments_show.md) - Show an experiment
- [auth0 experiments start](auth0_experiments_start.md) - Start an experiment
- [auth0 experiments update](auth0_experiments_update.md) - Update an experiment
- [auth0 experiments validate](auth0_experiments_validate.md) - Validate an experiment


