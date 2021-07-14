---
layout: default
---
## auth0 roles permissions remove

Remove a permission from a role

### Synopsis

Remove an existing permission defined in one of your APIs.
To remove a permission try:

    auth0 roles permissions remove <role-id> -p <permission-name>

```
auth0 roles permissions remove [flags]
```

### Examples

```
auth0 roles permissions remove <role-id> -p <permission-name>
auth0 roles permissions rm
```

### Options

```
  -a, --api-id string         API Identifier.
  -h, --help                  help for remove
  -p, --permissions strings   Permissions.
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

* [auth0 roles permissions](auth0_roles_permissions.md)	 - Manage permissions within the role resource

