---
layout: default
---
## auth0 users update

Update a user

### Synopsis

Update a user.

```
auth0 users update [flags]
```

### Examples

```
auth0 users update 
auth0 users update <id> 
auth0 users update <id> --name John Doe
auth0 users update -n John Doe --email john.doe@example.com
```

### Options

```
  -c, --connection string   Name of the connection this user should be created in.
  -e, --email string        The user's email.
  -h, --help                help for update
  -n, --name string         The user's full name.
  -p, --password string     Initial password for this user (mandatory for non-SMS connections).
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

* [auth0 users](auth0_users.md)	 - Manage resources for users

