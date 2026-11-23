---
layout: default
parent: auth0 rules
has_toc: false
---
# auth0 rules update

*DEPRECATED!* Rules are deprecated and will be removed in the near future. Users should migrate all rules to actions. See https://auth0.com/docs/customize/actions/migrate/migrate-from-rules-to-actions for more details.

Update a rule.

To update interactively, use `auth0 rules update` with no arguments.

To update non-interactively, supply the rule id and other information through the flags.

Use '--schema' to print the request payload schema and exit.
Use '--data' to supply the full JSON payload (validated against the schema before sending).

## Usage
```
auth0 rules update [flags]
```

## Examples

```
  auth0 rules update <id>
  auth0 rules update <rule-id> --enabled=true
  auth0 rules update <rule-id> --enabled=false --name "My Updated Rule"
  auth0 rules update <rule-id> --enabled=true --name "My Updated Rule" --script "$(cat path/to/script.js)"
  auth0 rules update <rule-id> -e=true -n "My Updated Rule" -s "$(cat path/to/script.js)" --json
  auth0 rules update <rule-id> -e=true -n "My Updated Rule" -s "$(cat path/to/script.js)" --json-compact

  # Discover the payload schema
  auth0 rules update --schema
  auth0 rules update --schema --json

  # JSON input mode (for agents and automation)
  auth0 rules update <rule-id> --data '{"enabled":false,"order":2}'
  auth0 rules update <rule-id> --data @rule.json
  cat rule.json | auth0 rules update <rule-id>
```


## Flags

```
      --data string     JSON payload for the operation, as a JSON string or file path (@file.json). Can also be piped via stdin.
  -e, --enabled         Enable (or disable) a rule. (default true)
      --force           Skip confirmation.
      --json            Output in json format.
      --json-compact    Output in compact json format.
  -n, --name string     Name of the rule.
      --schema          Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
  -s, --script string   Script contents for the rule.
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


