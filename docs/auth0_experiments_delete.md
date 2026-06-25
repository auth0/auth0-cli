---
layout: default
parent: auth0 experiments
has_toc: false
---
# auth0 experiments delete

Delete an experiment.

Active experiments must be paused or completed before deleting.

To delete non-interactively, supply the experiment ID and use `--force` to skip confirmation.

## Usage
```
auth0 experiments delete [flags]
```

## Examples

```
  auth0 experiments delete
  auth0 experiments delete <experiment-id>
  auth0 experiments delete <experiment-id> --force
```


## Flags

```
      --force   Skip confirmation.
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


