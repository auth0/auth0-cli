---
layout: default
parent: auth0 client-grants
has_toc: false
---
# auth0 client-grants search

[Early Access] Search your client grants using Lucene or SCIM filter syntax.
Results are eventually consistent; use `auth0 client-grants list` for up-to-date data.
Filter by client, audience, subject type or organization with '--filter'; the `list` filter flags are not available here.
With '--fields', keep 'allow_all_scopes' next to 'scope', and 'default_for' next to 'client_id', so the Scopes and Client ID columns stay complete.

Use '--schema' to see available query parameters.
Use '--query' to filter results via a JSON object (any API-supported parameter works immediately).

## Usage
```
auth0 client-grants search [flags]
```

## Examples

```
  auth0 client-grants search
  auth0 client-grants search --filter 'audience:"https://api.example.com"'
  auth0 client-grants search --filter 'client_id:<client-id>' --sort audience
  auth0 client-grants search --parser scim --filter 'audience co "example" and organization_id eq "<org-id>"' -s audience:1
  auth0 client-grants search --parser scim --filter 'updated_at gt "2026-01-01"' -n 200 --csv
  auth0 client-grants search --filter 'default_for:third_party_clients' --fields id,default_for,audience --json
  auth0 client-grants search --schema
  auth0 client-grants search --query '{"q":"subject_type eq \"client\"","parser":"scim","sort":"audience"}'
```


## Flags

```
      --csv              Output in csv format.
      --exclude-fields   Exclude the --fields list from the response instead of returning only those fields.
      --fields string    Comma-separated list of fields to include in the response, e.g. 'id,client_id,audience'.
      --filter string    Filter expression, sent as the 'q' query parameter. Uses Lucene syntax, or SCIM syntax with --parser scim. Supported fields: audience,client_id,subject_type,allow_any_organization,organization_id,updated_at,default_for. Maximum 5 filter operations. Equivalent to --query '{"q":"<expression>"}'.
                         
                         For example: 'audience:"https://api.example.com"' or, with --parser scim, 'client_id eq "<client-id>" and subject_type eq "client"'.
      --json             Output in json format.
      --json-compact     Output in compact json format.
  -n, --number int       Number of client grants to retrieve. Minimum 1, maximum 1000. (default 100)
      --parser string    Syntax of --filter: 'lucene' or 'scim'. (default "lucene")
  -q, --query string     Search with a JSON object of query parameters, sent as-is. Any API-supported parameter works immediately. Run '--schema' to see documented parameters. Returns one page: set "take" (max 100) for the page size, and "from" to the previous response's "next" to fetch the next page.
      --schema           Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
  -s, --sort string      Field to sort by, in ascending order only. One of: audience,updated_at. The 'field:1' form is also accepted. Defaults to insertion order.
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

- [auth0 client-grants create](auth0_client-grants_create.md) - Create a new client grant
- [auth0 client-grants delete](auth0_client-grants_delete.md) - Delete a client grant
- [auth0 client-grants list](auth0_client-grants_list.md) - List your client grants
- [auth0 client-grants organizations](auth0_client-grants_organizations.md) - Manage organizations of a client grant
- [auth0 client-grants search](auth0_client-grants_search.md) - Search your client grants (Early Access)
- [auth0 client-grants show](auth0_client-grants_show.md) - Show a client grant
- [auth0 client-grants update](auth0_client-grants_update.md) - Update a client grant


