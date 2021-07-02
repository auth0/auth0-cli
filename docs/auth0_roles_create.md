---
layout: default
---
## auth0 roles create

Create a new role

### Synopsis

Create a new role.

```
auth0 roles create [flags]
```

### Examples

```
auth0 roles create
auth0 roles create --name myrole
auth0 roles create -n myrole --description "awesome role"
```

### Options

```
  -d, --description string   Description of the role.
  -h, --help                 help for create
  -n, --name string          Name of the role.
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

* [auth0 roles](auth0_roles.md)	 - Manage resources for roles

