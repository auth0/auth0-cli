---
layout: default
---
## auth0 rules update

Update a rule

### Synopsis

Update a rule.

```
auth0 rules update [flags]
```

### Examples

```
auth0 rules update <id> 
auth0 rules update <id> --name "My Updated Rule"
auth0 rules update <id> -n "My Updated Rule" --enabled=false
```

### Options

```
  -e, --enabled       Enable (or disable) a rule. (default true)
  -h, --help          help for update
  -n, --name string   Name of the rule.
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

