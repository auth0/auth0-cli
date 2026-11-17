---
layout: default
parent: auth0 network-acl
has_toc: false
---
# auth0 network-acl list

List your network ACLs. To create one, run: auth0 network-acl create

## Usage
```
auth0 network-acl list [flags]
```

## Examples

```
  auth0 network-acl list
  auth0 network-acl ls
  auth0 network-acl ls --json
  auth0 network-acl ls --json-compact

  # Discover the query schema
  auth0 network-acl list --schema
  auth0 network-acl list --schema --json

  # JSON query (for agents and automation)
  auth0 network-acl list --query '{"per_page":10,"include_totals":true}'
```


## Flags

```
      --json           Output in JSON format
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

- [auth0 network-acl create](auth0_network-acl_create.md) - Create a new network ACL
- [auth0 network-acl delete](auth0_network-acl_delete.md) - Delete a network ACL
- [auth0 network-acl list](auth0_network-acl_list.md) - List network ACLs
- [auth0 network-acl show](auth0_network-acl_show.md) - Show a network ACL
- [auth0 network-acl update](auth0_network-acl_update.md) - Update a network ACL


