---
layout: default
parent: auth0 apps
has_toc: false
---
# auth0 apps search

[Early Access] Search your applications using Lucene or SCIM filter syntax.
Results are eventually consistent; use `auth0 apps list` for up-to-date data.
Client secrets and keys are never returned.

Use '--schema' to see available query parameters.
Use '--query' to filter results via a JSON object (any API-supported parameter works immediately).

## Usage
```
auth0 apps search [flags]
```

## Examples

```
  auth0 apps search
  auth0 apps search --filter 'name:"My App"'
  auth0 apps search --filter 'app_type:spa' --sort name
  auth0 apps search --parser scim --filter 'name co "billing"' -s name:1
  auth0 apps search --parser scim --filter 'updated_at gt "2026-01-01"' -n 200 --csv
  auth0 apps search --filter 'client_grant.organization_id:<org-id>' --fields client_id,name --json
  auth0 apps search --schema
  auth0 apps search --query '{"q":"name co \"billing\"","parser":"scim","sort":"name"}'
```


## Flags

```
      --csv              Output in csv format.
      --exclude-fields   Exclude the --fields list from the response instead of returning only those fields.
      --fields string    Comma-separated list of fields to include in the response, e.g. 'client_id,name,app_type'.
      --filter string    Filter expression, sent as the 'q' query parameter. Uses Lucene syntax, or SCIM syntax with --parser scim. Supported fields: client_id,external_client_id,name,app_type,is_first_party,updated_at,metadata.{key},client_grant.organization_id,client_grant.allow_any_organization. Maximum 5 filter operations. Equivalent to --query '{"q":"<expression>"}'.
                         
                         For example: 'app_type:spa' or, with --parser scim, 'name co "billing" and is_first_party eq true'.
      --json             Output in json format.
      --json-compact     Output in compact json format.
  -n, --number int       Number of apps to retrieve. Minimum 1, maximum 1000. (default 100)
      --parser string    Syntax of --filter: 'lucene' or 'scim'. (default "lucene")
  -q, --query string     Search with a JSON object of query parameters, sent as-is. Any API-supported parameter works immediately. Run '--schema' to see documented parameters. Returns one page: set "take" (max 100) for the page size, and "from" to the previous response's "next" to fetch the next page.
      --schema           Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
  -s, --sort string      Field to sort by, in ascending order only. One of: name,updated_at. The 'field:1' form is also accepted. Defaults to insertion order.
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

- [auth0 apps create](auth0_apps_create.md) - Create a new application
- [auth0 apps delete](auth0_apps_delete.md) - Delete an application
- [auth0 apps list](auth0_apps_list.md) - List your applications
- [auth0 apps open](auth0_apps_open.md) - Open the settings page of an application
- [auth0 apps search](auth0_apps_search.md) - Search your applications (Early Access)
- [auth0 apps session-transfer](auth0_apps_session-transfer.md) - Manage session transfer settings for an application
- [auth0 apps show](auth0_apps_show.md) - Show an application
- [auth0 apps update](auth0_apps_update.md) - Update an application
- [auth0 apps use](auth0_apps_use.md) - Choose a default application for the Auth0 CLI


