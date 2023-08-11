---
layout: default
parent: auth0 rules
has_toc: false
---
# auth0 rules delete

*DEPRECATED!* Rules are deprecated and will be removed in the near future. Users should migrate all rules to actions. See https://auth0.com/docs/customize/actions/migrate/migrate-from-rules-to-actions for more details.

Delete a rule.

To delete interactively, use `auth0 rules delete` with no arguments.

To delete non-interactively, supply the rule id and the `--force` flag to skip confirmation.

## Usage
```
auth0 rules delete [flags]
```

## Examples

```
  auth0 rules delete 
  auth0 rules rm
  auth0 rules delete <rule-id>
  auth0 rules delete <rule-id> --force
```


## Flags

```
      --force   Skip confirmation.
```


## Inherited Flags

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


