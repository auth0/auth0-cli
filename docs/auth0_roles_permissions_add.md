---
layout: default
---
## auth0 roles permissions add

Add a permission to a role

### Synopsis

Add an existing permission defined in one of your APIs.
To add a permission try:

    auth0 roles permissions add <role-id> -p <permission-name>

```
auth0 roles permissions add [flags]
```

### Examples

```
auth0 roles permissions add <role-id> -p <permission-name>
auth0 roles permissions add
```

### Options

```
  -a, --api-id string         API Identifier.
  -h, --help                  help for add
  -p, --permissions strings   Permissions.
```

### Options inherited from parent commands

```
      --debug           Enable debug mode.
      --force           Skip confirmation.
      --json            Output in json format.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```

### SEE ALSO

* [auth0 roles permissions](auth0_roles_permissions.md)	 - Manage permissions within the role resource

