---
layout: default
parent: auth0 orgs
has_toc: false
---
# auth0 orgs search

[Early Access] Search your organizations using Lucene or SCIM filter syntax.
Results are eventually consistent; use `auth0 orgs list` for up-to-date data.
'metadata' can be filtered on, but 'metadata' and 'client' are never returned.

Use '--schema' to see available query parameters.
Use '--query' to filter results via a JSON object (any API-supported parameter works immediately).

## Usage
```
auth0 orgs search [flags]
```

## Examples

```
  auth0 orgs search
  auth0 orgs search --filter 'name:"acme"'
  auth0 orgs search --filter 'metadata.tier:gold' --sort display_name
  auth0 orgs search --parser scim --filter 'display_name co "Acme"' -s name:1
  auth0 orgs search --parser scim --filter 'created_at gt "2026-01-01"' -n 200 --csv
  auth0 orgs search --filter 'id:<org-id>' --json

  # Discover the query schema
  auth0 orgs search --schema
  auth0 orgs search --schema --json

  # JSON query (for agents and automation)
  auth0 orgs search --query '{"q":"name sw \"acme\"","parser":"scim","sort":"name"}'
```


## Flags

```
      --csv             Output in csv format.
      --filter string   Filter expression, sent as the 'q' query parameter. Uses Lucene syntax, or SCIM syntax with --parser scim. Supported fields: id,name,display_name,created_at,metadata.{key}.
                         Maximum 5 filter operations. Equivalent to --query '{"q":"<expression>"}'.
                        
                        For example: 'name:"acme"' or, with --parser scim, 'display_name co "Acme" and created_at gt "2026-01-01"'.
      --json            Output in json format.
      --json-compact    Output in compact json format.
  -n, --number int      Number of organizations to retrieve. Minimum 1, maximum 1000. (default 100)
      --parser string   Syntax of --filter: 'lucene' or 'scim'. (default "lucene")
  -q, --query string    Search with a JSON object of query parameters, sent as-is. Any API-supported parameter works immediately. Run '--schema' to see documented parameters. Returns one page: set "take" (max 100) for the page size, and "from" to the previous response's "next" to fetch the next page.
      --schema          Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
  -s, --sort string     Field to sort by, in ascending order only. One of: name,display_name,created_at. The 'field:1' form is also accepted. Defaults to insertion order.
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

- [auth0 orgs create](auth0_orgs_create.md) - Create a new organization
- [auth0 orgs delete](auth0_orgs_delete.md) - Delete an organization
- [auth0 orgs invitations](auth0_orgs_invitations.md) - Manage invitations of an organization
- [auth0 orgs list](auth0_orgs_list.md) - List your organizations
- [auth0 orgs members](auth0_orgs_members.md) - Manage members of an organization
- [auth0 orgs open](auth0_orgs_open.md) - Open the settings page of an organization
- [auth0 orgs roles](auth0_orgs_roles.md) - Manage roles of an organization
- [auth0 orgs search](auth0_orgs_search.md) - Search your organizations (Early Access)
- [auth0 orgs show](auth0_orgs_show.md) - Show an organization
- [auth0 orgs update](auth0_orgs_update.md) - Update an organization


