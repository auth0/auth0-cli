---
layout: default
parent: auth0 experimentation experiments
has_toc: false
---
# auth0 experimentation experiments status

Transition an experiment to a new lifecycle status: active, paused, completed, or archived.

  • active    — start (or resume) the experiment; runs full validation before activating
  • paused    — pause a running experiment; it can be resumed by setting it active again
  • completed — mark the experiment as finished; it can then be archived
  • archived  — archive a completed experiment

To set the status interactively, run `auth0 experimentation experiments status` with no arguments.

## Usage
```
auth0 experimentation experiments status [flags]
```

## Examples

```
  auth0 experimentation experiments status
  auth0 experimentation experiments status <experiment-id>
  auth0 experimentation experiments status <experiment-id> active
  auth0 experimentation experiments status <experiment-id> paused
```


## Flags

```
      --json           Output in json format.
      --json-compact   Output in compact json format.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 experimentation experiments create](auth0_experimentation_experiments_create.md) - Create a new experiment
- [auth0 experimentation experiments delete](auth0_experimentation_experiments_delete.md) - Delete an experiment
- [auth0 experimentation experiments list](auth0_experimentation_experiments_list.md) - List your experiments
- [auth0 experimentation experiments show](auth0_experimentation_experiments_show.md) - Show an experiment
- [auth0 experimentation experiments status](auth0_experimentation_experiments_status.md) - Change an experiment's status
- [auth0 experimentation experiments update](auth0_experimentation_experiments_update.md) - Update an experiment
- [auth0 experimentation experiments validate](auth0_experimentation_experiments_validate.md) - Validate an experiment


