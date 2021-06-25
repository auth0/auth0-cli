---
layout: default
---
## auth0 rules create

Create a new rule

### Synopsis

Create a new rule.

```
auth0 rules create [flags]
```

### Examples

```
auth0 rules create
auth0 rules create --name "My Rule"
auth0 rules create -n "My Rule" --template "Empty rule"
auth0 rules create -n "My Rule" -t "Empty rule" --enabled=false
```

### Options

```
  -e, --enabled           Enable (or disable) a rule. (default true)
  -h, --help              help for create
  -n, --name string       Name of the rule.
  -t, --template string   Template to use for the rule.
```

### Options inherited from parent commands

```
      --debug           Enable debug mode.
      --force           Skip confirmation.
      --format string   Command output format. Options: json.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```

### SEE ALSO

* [auth0 rules](auth0_rules.md)	 - Manage resources for rules

