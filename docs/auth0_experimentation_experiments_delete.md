---
layout: default
parent: auth0 experimentation experiments
has_toc: false
---
# auth0 experimentation experiments delete

Delete an experiment.

Active experiments must be paused or completed before deleting.

To delete non-interactively, supply the experiment ID and use `--force` to skip confirmation.

## Usage
```
auth0 experimentation experiments delete [flags]
```

## Examples

```
  auth0 experimentation experiments delete
  auth0 experimentation experiments delete <experiment-id>
  auth0 experimentation experiments delete <experiment-id> --force
```


## Flags

```
      --force   Skip confirmation.
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


