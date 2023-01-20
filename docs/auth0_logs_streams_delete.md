---
layout: default
---
# auth0 logs streams delete

Delete a log stream.

To delete interactively, use `auth0 logs streams delete` with no arguments.

To delete non-interactively, supply the log stream id and the `--force` flag to skip confirmation.

## Usage
```
auth0 logs streams delete [flags]
```

## Examples

```
  auth0 logs streams delete
  auth0 logs streams rm
  auth0 logs streams delete <log-stream-id>
  auth0 logs streams delete <log-stream-id> --force
```


## Flags

```
      --force   Skip confirmation.
```


## InheritedFlags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 logs streams create](auth0_logs_streams_create.md) - Create a new log stream
- [auth0 logs streams delete](auth0_logs_streams_delete.md) - Delete a log stream
- [auth0 logs streams list](auth0_logs_streams_list.md) - List all log streams
- [auth0 logs streams open](auth0_logs_streams_open.md) - Open the settings page of a log stream
- [auth0 logs streams show](auth0_logs_streams_show.md) - Show a log stream by ID
- [auth0 logs streams update](auth0_logs_streams_update.md) - Update an existing log stream


