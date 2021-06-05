## auth0 logs list

Show the tenant logs allowing to filter by Client Id.

```
auth0 logs list [flags]
```

### Examples

```
auth0 logs list
auth0 logs list --client-id <id>
auth0 logs ls -n 100
```

### Flags

```
  -c, --client-id string   Client Id of an Auth0 application to filter the logs.
  -h, --help               help for list
  -n, --number int         Number of log entries to show. (default 100)
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

* [auth0 logs](auth0_logs.md)	 - View tenant logs
