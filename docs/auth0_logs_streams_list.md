---
layout: default
parent: auth0 logs streams
has_toc: false
---
# auth0 logs streams list

List your existing log streams. To create one, run: `auth0 logs streams create`.

Use '--schema' to see the query parameters the API documents for this operation.
Use '--query' to pass query parameters as a JSON object and get the raw API response.

## Usage
```
auth0 logs streams list [flags]
```

## Examples

```
  auth0 logs streams list
  auth0 logs streams ls
  auth0 logs streams ls --json
  auth0 logs streams ls --json-compact
  auth0 logs streams ls --csv

  # Discover the query schema
  auth0 logs streams list --schema
  auth0 logs streams list --schema --json
```


## Flags

```
      --csv            Output in csv format.
      --json           Output in json format.
      --json-compact   Output in compact json format.
  -q, --query string   Filter results with a JSON object of query parameters. Any API-supported parameter works immediately. Run '--schema' to see documented parameters. On offset-paginated endpoints, add "include_totals":true to receive total counts and a pagination hint (without it the API returns a bare array and no hint can be given).
      --schema         Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
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

- [auth0 logs streams create](auth0_logs_streams_create.md) - Create a new log stream
- [auth0 logs streams delete](auth0_logs_streams_delete.md) - Delete a log stream
- [auth0 logs streams list](auth0_logs_streams_list.md) - List all log streams
- [auth0 logs streams open](auth0_logs_streams_open.md) - Open the settings page of a log stream
- [auth0 logs streams show](auth0_logs_streams_show.md) - Show a log stream by ID
- [auth0 logs streams update](auth0_logs_streams_update.md) - Update an existing log stream


