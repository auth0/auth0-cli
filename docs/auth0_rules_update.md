---
layout: default
---
# auth0 rules update

Update a rule.

To update interactively, use `auth0 rules update` with no arguments.

To update non-interactively, supply the rule id and other information through the flags.

## Usage
```
auth0 rules update [flags]
```

## Examples

```
  auth0 rules update <id>
  auth0 rules update <rule-id> --enabled true
  auth0 rules update <rule-id> --enabled true --name "My Updated Rule"
  auth0 rules update <rule-id> --enabled true --name "My Updated Rule" --script "$(cat path/to/script.js)"
  auth0 rules update <rule-id> -e true -n "My Updated Rule" -s "$(cat path/to/script.js)" --json
```


## Flags

```
  -e, --enabled         Enable (or disable) a rule. (default true)
      --json            Output in json format.
  -n, --name string     Name of the rule.
  -s, --script string   Script contents for the rule.
```


## InheritedFlags

```
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


