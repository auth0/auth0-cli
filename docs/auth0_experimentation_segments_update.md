---
layout: default
parent: auth0 experimentation segments
has_toc: false
---
# auth0 experimentation segments update

Update a segment.

To update interactively, use `auth0 experimentation segments update` with no arguments.

To update non-interactively, supply the segment ID and fields to change through the flags.

Use '--schema' to print the request payload schema and exit.
Use '--data' to supply the full JSON payload (validated against the schema before sending).

## Usage
```
auth0 experimentation segments update [flags]
```

## Examples

```
  auth0 experimentation segments update
  auth0 experimentation segments update <segment-id>
  auth0 experimentation segments update <segment-id> --name "New Name"
  auth0 experimentation segments update <segment-id> --rules '[{"match":{"domain":{"contains":["newdomain.com"]}}}]'
  auth0 experimentation segments update <segment-id> --status archived

  # Discover the payload schema
  auth0 experimentation segments update --schema
  auth0 experimentation segments update --schema --json

  # JSON input mode (for agents and automation)
  auth0 experimentation segments update <segment-id> --data '{"status":"active"}'
  auth0 experimentation segments update <segment-id> --data @segment.json
```


## Flags

```
      --data string          JSON payload for the operation, as a JSON string or file path (@file.json). Can also be piped via stdin.
  -d, --description string   Description of the segment.
      --json                 Output in json format.
      --json-compact         Output in compact json format.
  -n, --name string          Name of the segment.
  -r, --rules match          Rules for matching users, as a JSON array. Each rule has a match and/or `not_match` object that maps an attribute to a condition.
                             Attributes: client_id, connection, connection_type, organization_id, domain, device_type, browser, platform, user_agent, country, region.
                             Conditions: contains, starts_with, ends_with, exists, or a plain list ["a","b"] for an exact match.
                             When omitted, an editor opens pre-filled with a commented template (// and /* */ comments are stripped on save).
                             Example: '[{"match":{"domain":{"ends_with":["example.com"]}}}]'
      --schema               Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
      --status string        Transition the segment to a new status (active, archived). Archiving is irreversible.
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

- [auth0 experimentation segments create](auth0_experimentation_segments_create.md) - Create a new segment
- [auth0 experimentation segments delete](auth0_experimentation_segments_delete.md) - Delete a segment
- [auth0 experimentation segments list](auth0_experimentation_segments_list.md) - List your segments
- [auth0 experimentation segments show](auth0_experimentation_segments_show.md) - Show a segment
- [auth0 experimentation segments update](auth0_experimentation_segments_update.md) - Update a segment


