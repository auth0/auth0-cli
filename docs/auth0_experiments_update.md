---
layout: default
parent: auth0 experiments
has_toc: false
---
# auth0 experiments update

Update an experiment.

Note: feature flag, authentication flow, and allocation strategy cannot be changed after creation. To change an experiment's status, use `auth0 experiments status`.

To update interactively, use `auth0 experiments update` with no arguments.

## Usage
```
auth0 experiments update [flags]
```

## Examples

```
  auth0 experiments update
  auth0 experiments update <experiment-id>
  auth0 experiments update <experiment-id> --name "new-name"
  auth0 experiments update <experiment-id> --assignment-config '{"subject":"device"}'
  auth0 experiments update <experiment-id> --allocations '[{"variation_id":"vid","weight":100,"is_control":true}]'
```


## Flags

```
      --allocations string         JSON array of allocation items ({variation_id, weight, is_control} for percentage, where weight is an integer percentage from 1 to 100; {variation_id, segment_id, is_control} for segment).
      --assignment-config string   JSON object configuring how users are assigned to variations (e.g. '{"subject":"device"}').
  -d, --description string         Description of the experiment.
      --json                       Output in json format.
      --json-compact               Output in compact json format.
  -n, --name string                Name of the experiment.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 experiments create](auth0_experiments_create.md) - Create a new experiment
- [auth0 experiments delete](auth0_experiments_delete.md) - Delete an experiment
- [auth0 experiments list](auth0_experiments_list.md) - List your experiments
- [auth0 experiments show](auth0_experiments_show.md) - Show an experiment
- [auth0 experiments status](auth0_experiments_status.md) - Change an experiment's status
- [auth0 experiments update](auth0_experiments_update.md) - Update an experiment
- [auth0 experiments validate](auth0_experiments_validate.md) - Validate an experiment


