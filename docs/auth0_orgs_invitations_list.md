---
layout: default
parent: auth0 orgs invitations
has_toc: false
---
# auth0 orgs invitations list

List the invitations of an organization.

To list interactively, use `auth0 orgs invs list` with no flags.

To list non-interactively, supply the organization id through the flags.

Use '--schema' to see available query parameters.
Use '--query' to filter results via a JSON object (any API-supported parameter works immediately).

## Usage
```
auth0 orgs invitations list [flags]
```

## Examples

```
  auth0 orgs invs list
  auth0 orgs invs ls --org-id <org-id>
  auth0 orgs invs list --org-id <org-id> --number 100
  auth0 orgs invs ls --org-id <org-id> -n 50 --json
  auth0 orgs invs ls --org-id <org-id> -n 500 --json-compact
  auth0 orgs invs ls --org-id <org-id> --csv
  auth0 orgs invs list --schema
  auth0 orgs invs list --org-id <org-id> --query '{"sort":"created_at:-1","per_page":20}'
```


## Flags

```
      --csv             Output in csv format.
      --json            Output in json format.
      --json-compact    Output in compact json format.
  -n, --number int      Number of organization invitations to retrieve. Minimum 1, maximum 1000. (default 100)
      --org-id string   ID of the organization.
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

- [auth0 orgs invitations create](auth0_orgs_invitations_create.md) - Create a new invitation to an organization
- [auth0 orgs invitations delete](auth0_orgs_invitations_delete.md) - Delete invitation(s) from an organization
- [auth0 orgs invitations list](auth0_orgs_invitations_list.md) - List invitations of an organization
- [auth0 orgs invitations show](auth0_orgs_invitations_show.md) - Show an organization invitation


