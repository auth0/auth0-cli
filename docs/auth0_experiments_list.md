---
layout: default
parent: auth0 experiments
has_toc: false
---
# auth0 experiments list

List all experiments. To create one, run: `auth0 experiments create`.

## Usage
```
auth0 experiments list [flags]
```

## Examples

```
  auth0 experiments list
  auth0 experiments ls
  auth0 experiments list --json
  auth0 experiments list --status active
  auth0 experiments list --feature-flag-id <id>
```


## Flags

```
      --authentication-flow string   Filter by authentication flow.
      --csv                          Output in csv format.
      --feature-flag-id string       Filter by feature flag ID.
      --json                         Output in json format.
      --json-compact                 Output in compact json format.
      --status string                Filter by status (draft, active, paused, completed, archived).
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


