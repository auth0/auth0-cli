---
layout: default
----## index roles update

Update a role

### Synopsis

Update a role.

```
index roles update [flags]
```

### Examples

```
auth0 roles update
auth0 roles update <id> --name myrole
auth0 roles update <id> -n myrole --description "awesome role"
```

### Options

```
  -d, --description string   Description of the role.
  -h, --help                 help for update
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

* [index roles](index_roles.md)	 - Manage resources for roles

###### Auto generated by spf13/cobra on 25-Jun-2021