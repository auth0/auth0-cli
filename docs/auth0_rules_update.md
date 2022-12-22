---
layout: default
---
# auth0 rules update

Update a rule.

To update interactively, use `auth0 rules update` with no arguments.

To update non-interactively, supply the rule id and other information through the flags.

```
auth0 rules update [flags]
```


## Flags

```
  -e, --enabled       Enable (or disable) a rule. (default true)
      --json          Output in json format.
  -n, --name string   Name of the rule.
```


## InheritedFlags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```

## Examples

```
  auth0 rules update <id>
  auth0 rules update <id> --name "My Updated Rule"
  auth0 rules update <id> -n "My Updated Rule" --enabled=false
```


## Related Commands

- [auth0 rules create](auth0_rules_create.md) - Create a new rule
- [auth0 rules delete](auth0_rules_delete.md) - Delete a rule
- [auth0 rules disable](auth0_rules_disable.md) - Disable a rule
- [auth0 rules enable](auth0_rules_enable.md) - Enable a rule
- [auth0 rules list](auth0_rules_list.md) - List your rules
- [auth0 rules show](auth0_rules_show.md) - Show a rule
- [auth0 rules update](auth0_rules_update.md) - Update a rule


