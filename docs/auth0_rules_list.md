---
layout: default
parent: auth0 rules
has_toc: false
---
# auth0 rules list

*DEPRECATED!* Rules are deprecated and will be removed in the near future. Users should migrate all rules to actions. See https://auth0.com/docs/customize/actions/migrate/migrate-from-rules-to-actions for more details.

List your existing rules. To create one, run: `auth0 rules create`.

Use '--schema' to see available query parameters.
Use '--query' to filter results via a JSON object (any API-supported parameter works immediately).

## Usage
```
auth0 rules list [flags]
```

## Examples

```
  auth0 rules list
  auth0 rules ls
  auth0 rules ls --json
  auth0 rules ls --json-compact
  auth0 rules ls --csv

  # Discover the query schema
  auth0 rules list --schema
  auth0 rules list --schema --json

  # JSON query (for agents and automation)
  auth0 rules list --query '{"enabled":true,"per_page":10,"include_totals":true}'
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

- [auth0 rules create](auth0_rules_create.md) - Create a new rule
- [auth0 rules delete](auth0_rules_delete.md) - Delete a rule
- [auth0 rules disable](auth0_rules_disable.md) - Disable a rule
- [auth0 rules enable](auth0_rules_enable.md) - Enable a rule
- [auth0 rules list](auth0_rules_list.md) - List your rules
- [auth0 rules show](auth0_rules_show.md) - Show a rule
- [auth0 rules update](auth0_rules_update.md) - Update a rule


