---
layout: default
parent: auth0 experiments
has_toc: false
---
# auth0 experiments status

Transition an experiment to a new lifecycle status: active, paused, completed, or archived.

  • active    — start (or resume) the experiment; runs full validation before activating
  • paused    — pause a running experiment; it can be resumed by setting it active again
  • completed — mark the experiment as finished; it can then be archived
  • archived  — archive a completed experiment

To set the status interactively, run `auth0 experiments status` with no arguments.

## Usage
```
auth0 experiments status [flags]
```

## Examples

```
  auth0 experiments status
  auth0 experiments status <experiment-id>
  auth0 experiments status <experiment-id> active
  auth0 experiments status <experiment-id> paused
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

- [auth0 experiments create](auth0_experiments_create.md) - Create a new experiment
- [auth0 experiments delete](auth0_experiments_delete.md) - Delete an experiment
- [auth0 experiments list](auth0_experiments_list.md) - List your experiments
- [auth0 experiments show](auth0_experiments_show.md) - Show an experiment
- [auth0 experiments status](auth0_experiments_status.md) - Change an experiment's status
- [auth0 experiments update](auth0_experiments_update.md) - Update an experiment
- [auth0 experiments validate](auth0_experiments_validate.md) - Validate an experiment


