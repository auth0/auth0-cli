---
layout: default
---
# auth0 rules create

Create a new rule.

To create interactively, use `auth0 rules create` with no arguments.

To create non-interactively, supply the name, template and other information through the flags.

```
auth0 rules create [flags]
```


## Flags

```
  -e, --enabled           Enable (or disable) a rule. (default true)
      --json              Output in json format.
  -n, --name string       Name of the rule.
  -t, --template string   Template to use for the rule.
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
  auth0 rules create
  auth0 rules create --name "My Rule"
  auth0 rules create -n "My Rule" --template "Empty rule"
  auth0 rules create -n "My Rule" -t "Empty rule" --enabled=false
```


## Related Commands

- [auth0 rules create](auth0_rules_create.md) - Create a new rule
- [auth0 rules delete](auth0_rules_delete.md) - Delete a rule
- [auth0 rules disable](auth0_rules_disable.md) - Disable a rule
- [auth0 rules enable](auth0_rules_enable.md) - Enable a rule
- [auth0 rules list](auth0_rules_list.md) - List your rules
- [auth0 rules show](auth0_rules_show.md) - Show a rule
- [auth0 rules update](auth0_rules_update.md) - Update a rule


