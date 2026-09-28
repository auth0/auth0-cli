---
layout: default
parent: auth0 logs
has_toc: false
---
# auth0 logs list

Display the tenant logs allowing to filter using Lucene query syntax.

Use '--schema' to see available query parameters.
Use '--query' to filter results via a JSON object (any API-supported parameter works immediately).

## Usage
```
auth0 logs list [flags]
```

## Examples

```
  auth0 logs list
  auth0 logs list --filter "client_id:<client-id> --picker"
  auth0 logs list --filter "client_id:<client-id>"
  auth0 logs list --filter "client_name:<client-name>"
  auth0 logs list --filter "user_id:<user-id>"
  auth0 logs list --filter "user_name:<user-name>"
  auth0 logs list --filter "ip:<ip>"
  auth0 logs list --filter "type:f" # See the full list of type codes at https://auth0.com/docs/logs/log-event-type-codes
  auth0 logs ls -n 250 -p
  auth0 logs ls --json
  auth0 logs ls --json-compact
  auth0 logs ls --csv
  auth0 logs list --schema
  auth0 logs list --schema --json
  auth0 logs list --query '{"per_page":5,"include_totals":true}'
  auth0 logs list --query '{"per_page":5}' --json
```


## Flags

```
      --csv             Output in csv format.
  -f, --filter string   Filter in Lucene query syntax. See https://auth0.com/docs/logs/log-search-query-syntax for more details.
      --json            Output in json format.
      --json-compact    Output in compact json format.
  -n, --number int      Number of log entries to show. Minimum 1, maximum 1000. (default 100)
  -p, --picker          Allows to toggle from list of logs and view a selected log in detail
  -q, --query string    Filter results with a JSON object of query parameters. Any API-supported parameter works immediately. Run '--schema' to see documented parameters. On offset-paginated endpoints, add "include_totals":true to receive total counts and a pagination hint (without it the API returns a bare array and no hint can be given).
      --schema          Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
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

- [auth0 logs list](auth0_logs_list.md) - Show the tenant logs
- [auth0 logs streams](auth0_logs_streams.md) - Manage resources for log streams
- [auth0 logs tail](auth0_logs_tail.md) - Tail the tenant logs


