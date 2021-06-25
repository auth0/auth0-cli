---
layout: default
---
## auth0 logs list

Show the application logs

### Synopsis

Show the tenant logs allowing to filter using Lucene query syntax.

```
auth0 logs list [flags]
```

### Examples

```
auth0 logs list
auth0 logs list --filter "client_id:<client-id>"
auth0 logs list --filter "client_name:<client-name>"
auth0 logs list --filter "user_id:<user-id>"
auth0 logs list --filter "user_name:<user-name>"
auth0 logs list --filter "ip:<ip>"
auth0 logs list --filter "type:f" # See the full list of type codes at https://auth0.com/docs/logs/log-event-type-codes
auth0 logs ls -n 100
```

### Options

```
  -f, --filter string   Filter in Lucene query syntax. See https://auth0.com/docs/logs/log-search-query-syntax for more details.
  -h, --help            help for list
  -n, --number int      Number of log entries to show. (default 100)
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

* [auth0 logs](auth0_logs.md)	 - View tenant logs

