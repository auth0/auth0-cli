---
layout: default
parent: auth0 experimentation experiments
has_toc: false
---
# auth0 experimentation experiments validate

Check whether an experiment is ready to be activated. Returns validation status and any blocking errors.

## Usage
```
auth0 experimentation experiments validate [flags]
```

## Examples

```
  auth0 experimentation experiments validate
  auth0 experimentation experiments validate <experiment-id>
  auth0 experimentation experiments validate <experiment-id> --json
```


## Flags

```
      --json           Output in json format.
      --json-compact   Output in compact json format.
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


