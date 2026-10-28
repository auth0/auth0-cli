---
layout: default
parent: auth0 experimentation segments
has_toc: false
---
# auth0 experimentation segments list

List all segments. To create one, run: `auth0 experimentation segments create`.

Use '--schema' to see available query parameters.
Use '--query' to filter results via a JSON object (any API-supported parameter works immediately).

## Usage
```
auth0 experimentation segments list [flags]
```

## Examples

```
  auth0 experimentation segments list
  auth0 experimentation segments ls
  auth0 experimentation segments list --type self
  auth0 experimentation segments list --json
  auth0 experimentation segments list --csv
  auth0 experimentation segments list --schema
  auth0 experimentation segments list --schema --json
  auth0 experimentation segments list --query '{"type":"self","take":10}'
  auth0 experimentation segments list --query '{"type":"self"}' --json
```


## Flags

```
      --csv            Output in csv format.
      --json           Output in json format.
      --json-compact   Output in compact json format.
  -q, --query string   Filter results with a JSON object of query parameters. Any API-supported parameter works immediately. Run '--schema' to see documented parameters. On offset-paginated endpoints, add "include_totals":true to receive total counts and a pagination hint (without it the API returns a bare array and no hint can be given).
      --schema         Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
      --type string    Filter by type (auth0, self).
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


