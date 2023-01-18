---
layout: default
parent: auth0 rules
has_toc: false
---
# auth0 rules create

Create a new rule.

To create interactively, use `auth0 rules create` with no arguments.

To create non-interactively, supply the name, template and other information through the flags.

## Usage
```
auth0 rules create [flags]
```

## Examples

```
  auth0 rules create
  auth0 rules create --enabled true
  auth0 rules create --enabled true --name "My Rule" 
  auth0 rules create --enabled true --name "My Rule" --template "Empty rule"
  auth0 rules create --enabled true --name "My Rule" --template "Empty rule" --script "$(cat path/to/script.js)"
  auth0 rules create -e true -n "My Rule" -t "Empty rule" -s "$(cat path/to/script.js)" --json
  echo "{\"name\":\"piping-name\",\"script\":\"console.log('test')\"}" | auth0 rules create
```


## Flags

```
  -e, --enabled           Enable (or disable) a rule. (default true)
      --json              Output in json format.
  -n, --name string       Name of the rule.
  -s, --script string     Script contents for the rule.
  -t, --template string   Template to use for the rule.
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


