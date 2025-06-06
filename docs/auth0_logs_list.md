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
  auth0 logs list --filter "client_id:<client-id> --picker"
  auth0 logs list --filter "client_id:<client-id>"
  auth0 logs list --filter "client_name:<client-name>"
  auth0 logs list --filter "user_id:<user-id>"
  auth0 logs list --filter "user_name:<user-name>"
  auth0 logs list --filter "ip:<ip>"
  auth0 logs list --filter "type:f" # See the full list of type codes at https://auth0.com/docs/logs/log-event-type-codes
  auth0 logs ls -n 250 -p
  auth0 logs ls --json
  auth0 logs ls --csv
```


## Flags

```
      --csv             Output in csv format.
  -f, --filter string   Filter in Lucene query syntax. See https://auth0.com/docs/logs/log-search-query-syntax for more details.
      --json            Output in json format.
  -n, --number int      Number of log entries to show. Minimum 1, maximum 1000. (default 100)
  -p, --picker          Allows to toggle from list of logs and view a selected log in detail
```


## Inherited Flags

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


