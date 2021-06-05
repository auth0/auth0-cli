## auth0 logs tail

Tail the tenant logs allowing to filter by Client ID.

```
auth0 logs tail [flags]
```

### Examples

```
auth0 logs tail
auth0 logs tail --client-id <id>
auth0 logs tail -n 100
```

### Flags

```
  -c, --client-id string   Client Id of an Auth0 application to filter the logs.
  -h, --help               help for tail
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
