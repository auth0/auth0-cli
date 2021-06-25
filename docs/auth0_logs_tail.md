---
layout: default
---
## auth0 logs tail

Tail the tenant logs

### Synopsis

Tail the tenant logs allowing to filter using Lucene query syntax.

```
auth0 logs tail [flags]
```

### Examples

```
auth0 logs tail
auth0 logs tail --filter "client_id:<client-id>"
auth0 logs tail --filter "client_name:<client-name>"
auth0 logs tail --filter "user_id:<user-id>"
auth0 logs tail --filter "user_name:<user-name>"
auth0 logs tail --filter "ip:<ip>"
auth0 logs tail --filter "type:f" # See the full list of type codes at https://auth0.com/docs/logs/log-event-type-codes
auth0 logs tail -n 100
```

### Options

```
  -f, --filter string   Filter in Lucene query syntax. See https://auth0.com/docs/logs/log-search-query-syntax for more details.
  -h, --help            help for tail
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

