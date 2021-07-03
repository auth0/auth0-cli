---
layout: default
---
## auth0 users create

Create a new user

### Synopsis

Create a new user.

```
auth0 users create [flags]
```

### Examples

```
auth0 users create 
auth0 users create --name "John Doe" 
auth0 users create -n "John Doe" --email john@example.com
auth0 users create -n "John Doe" --e john@example.com --connection "Username-Password-Authentication"
```

### Options

```
  -c, --connection string   Name of the connection this user should be created in.
  -e, --email string        The user's email.
  -h, --help                help for create
  -n, --name string         The user's full name.
  -p, --password string     Initial password for this user (mandatory for non-SMS connections).
  -u, --username string     The user's username. Only valid if the connection requires a username.
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

