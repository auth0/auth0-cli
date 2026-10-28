---
layout: default
parent: auth0 experimentation experiments
has_toc: false
---
# auth0 experimentation experiments advance-ramp

Advance a ramp experiment to the next exposure level in its schedule.

The target level must be the immediate next level configured on the experiment. Configure ramp levels with `auth0 experimentation experiments create --levels` (or `update --levels`).

Use '--schema' to print the request payload schema and exit.
Use '--data' to supply the full JSON payload (validated against the schema before sending).

## Usage
```
auth0 experimentation experiments advance-ramp [flags]
```

## Examples

```
  auth0 experimentation experiments advance-ramp
  auth0 experimentation experiments advance-ramp <experiment-id> --target-level 25

  # Discover the payload schema
  auth0 experimentation experiments advance-ramp --schema
  auth0 experimentation experiments advance-ramp --schema --json

  # JSON input mode (for agents and automation)
  auth0 experimentation experiments advance-ramp <experiment-id> --data '{"target_level":25}'
```


## Flags

```
      --data string        JSON payload for the operation, as a JSON string or file path (@file.json). Can also be piped via stdin.
      --json               Output in json format.
      --json-compact       Output in compact json format.
      --schema             Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
      --target-level int   The target percentage level to advance to. Must be the immediate next level in the schedule.
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


