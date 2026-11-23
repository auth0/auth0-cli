---
layout: default
parent: auth0 orgs members
has_toc: false
---
# auth0 orgs members list

List the members of an organization.

Use '--schema' to see available query parameters.
Use '--query' to filter results via a JSON object (any API-supported parameter works immediately).

## Usage
```
auth0 orgs members list [flags]
```

## Examples

```
  auth0 orgs members list
  auth0 orgs members ls <org-id>
  auth0 orgs members list <org-id> --number 100
  auth0 orgs members ls <org-id> -n 100 --json
  auth0 orgs members ls <org-id> -n 100 --json-compact
  auth0 orgs members ls <org-id> --csv

  # Discover the query schema
  auth0 orgs members list --schema
  auth0 orgs members list --schema --json

  # JSON query (for agents and automation)
  auth0 orgs members list <org-id> --query '{"fields":"user_id,email","include_fields":true}'
```


## Flags

```
      --csv            Output in csv format.
      --json           Output in json format.
      --json-compact   Output in compact json format.
  -n, --number int     Number of organization members to retrieve. Minimum 1, maximum 1000. (default 100)
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

- [auth0 orgs members list](auth0_orgs_members_list.md) - List members of an organization


