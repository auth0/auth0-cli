---
layout: default
parent: auth0 logs
has_toc: false
---
# auth0 logs list

Display the tenant logs allowing to filter using Lucene query syntax.

## Usage
```
auth0 logs list [flags]
```

## Examples

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


## Flags

```
  -f, --filter string   Filter in Lucene query syntax. See https://auth0.com/docs/logs/log-search-query-syntax for more details.
      --json            Output in json format.
  -n, --number int      Number of log entries to show. (default 100)
```


## InheritedFlags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 logs list](auth0_logs_list.md) - Show the tenant logs
- [auth0 logs streams](auth0_logs_streams.md) - Manage resources for log streams
- [auth0 logs tail](auth0_logs_tail.md) - Tail the tenant logs


