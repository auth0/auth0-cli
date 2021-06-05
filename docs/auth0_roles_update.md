## auth0 roles update

Update a role.

```
auth0 roles update [flags]
```

### Examples

```
auth0 roles update
auth0 roles update <id> --name myrole
auth0 roles update <id> -n myrole --description "awesome role"
```

### Flags

```
  -d, --description string   Description of the role.
  -h, --help                 help for update
  -n, --name string          Name of the role.
```

### Flags inherited from parent commands

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
