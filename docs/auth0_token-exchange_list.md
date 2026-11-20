---
layout: default
parent: auth0 token-exchange
has_toc: false
---
# auth0 token-exchange list

List your existing token exchange profiles. To create one, run: `auth0 token-exchange create`.

Use '--schema' to see available query parameters.
Use '--query' to filter results via a JSON object (any API-supported parameter works immediately).

## Usage
```
auth0 token-exchange list [flags]
```

## Examples

```
  auth0 token-exchange list
  auth0 token-exchange ls
  auth0 token-exchange ls --json
  auth0 token-exchange ls --json-compact
  auth0 token-exchange ls --csv

  # Discover the query schema
  auth0 token-exchange list --schema
  auth0 token-exchange list --schema --json

  # JSON query (for agents and automation)
  auth0 token-exchange list --query '{"take":10}'
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

- [auth0 token-exchange create](auth0_token-exchange_create.md) - Create a new token exchange profile
- [auth0 token-exchange delete](auth0_token-exchange_delete.md) - Delete a token exchange profile
- [auth0 token-exchange list](auth0_token-exchange_list.md) - List your token exchange profiles
- [auth0 token-exchange show](auth0_token-exchange_show.md) - Show a token exchange profile
- [auth0 token-exchange update](auth0_token-exchange_update.md) - Update a token exchange profile


