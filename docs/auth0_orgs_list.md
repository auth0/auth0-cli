---
layout: default
parent: auth0 orgs
has_toc: false
---
# auth0 orgs list

List your existing organizations. To create one, run: `auth0 orgs create`.

Use '--schema' to see available query parameters.
Use '--query' to filter results via a JSON object (any API-supported parameter works immediately).

## Usage
```
auth0 orgs list [flags]
```

## Examples

```
  auth0 orgs list
  auth0 orgs ls
  auth0 orgs ls --json
  auth0 orgs ls --json-compact
  auth0 orgs ls --csv
  auth0 orgs ls -n 100

  # Discover the query schema
  auth0 orgs list --schema
  auth0 orgs list --schema --json

  # JSON query (for agents and automation)
  auth0 orgs list --query '{"per_page":50,"include_totals":true}'
```


## Flags

```
      --csv            Output in csv format.
      --json           Output in json format.
      --json-compact   Output in compact json format.
  -n, --number int     Number of organizations to retrieve. Minimum 1, maximum 1000. (default 100)
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

- [auth0 orgs create](auth0_orgs_create.md) - Create a new organization
- [auth0 orgs delete](auth0_orgs_delete.md) - Delete an organization
- [auth0 orgs invitations](auth0_orgs_invitations.md) - Manage invitations of an organization
- [auth0 orgs list](auth0_orgs_list.md) - List your organizations
- [auth0 orgs members](auth0_orgs_members.md) - Manage members of an organization
- [auth0 orgs open](auth0_orgs_open.md) - Open the settings page of an organization
- [auth0 orgs roles](auth0_orgs_roles.md) - Manage roles of an organization
- [auth0 orgs show](auth0_orgs_show.md) - Show an organization
- [auth0 orgs update](auth0_orgs_update.md) - Update an organization


