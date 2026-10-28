---
layout: default
parent: auth0 apis
has_toc: false
---
# auth0 apis search

[Early Access] Search your APIs using Lucene or SCIM filter syntax.
Results are eventually consistent; use `auth0 apis list` for up-to-date data.

Use '--schema' to see available query parameters.
Use '--query' to filter results via a JSON object (any API-supported parameter works immediately).

## Usage
```
auth0 apis search [flags]
```

## Examples

```
  auth0 apis search
  auth0 apis search --filter 'name:"My API"'
  auth0 apis search --parser scim --filter 'identifier sw "https://internal"' --sort identifier
  auth0 apis search --parser scim --filter 'name co "billing"' -s name:1
  auth0 apis search --parser scim --filter 'updated_at gt "2026-01-01"' -n 200 --csv
  auth0 apis search --filter 'name:"My API"' --fields id,name,identifier --json
  auth0 apis search --schema
  auth0 apis search --query '{"q":"name co \"billing\"","parser":"scim","sort":"name"}'
```


## Flags

```
      --csv              Output in csv format.
      --exclude-fields   Exclude the --fields list from the response instead of returning only those fields.
      --fields string    Comma-separated list of fields to include in the response, e.g. 'id,name,identifier'. 'updated_at' can be filtered and sorted on but is never returned.
      --filter string    Filter expression, sent as the API's 'q' parameter. Lucene syntax by default, or SCIM with --parser scim. Supported fields: id, identifier, name, updated_at. Maximum 5 filter operations.
                         
                         For example: 'name:"My API"' or, with --parser scim, 'name co "billing" and updated_at gt "2026-01-01"'. Equivalent to --query '{"q":"<expression>"}'.
      --json             Output in json format.
      --json-compact     Output in compact json format.
  -n, --number int       Number of APIs to retrieve. Minimum 1, maximum 1000. (default 100)
      --parser string    Syntax of --filter: 'lucene' or 'scim'. (default "lucene")
  -q, --query string     Search with a JSON object of query parameters, sent as-is. Any API-supported parameter works immediately. Run '--schema' to see documented parameters. Returns one page: set "take" (max 100) for the page size, and "from" to the previous response's "next" to fetch the next page.
      --schema           Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
  -s, --sort string      Field to sort by, ascending only: 'name', 'identifier' or 'updated_at'. The 'field:1' form is also accepted. Defaults to insertion order.
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

- [auth0 apis create](auth0_apis_create.md) - Create a new API
- [auth0 apis delete](auth0_apis_delete.md) - Delete an API
- [auth0 apis list](auth0_apis_list.md) - List your APIs
- [auth0 apis open](auth0_apis_open.md) - Open the settings page of an API
- [auth0 apis scopes](auth0_apis_scopes.md) - Manage resources for API scopes
- [auth0 apis search](auth0_apis_search.md) - Search your APIs (Early Access)
- [auth0 apis show](auth0_apis_show.md) - Show an API
- [auth0 apis update](auth0_apis_update.md) - Update an API


