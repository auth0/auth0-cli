---
layout: default
parent: auth0 events
has_toc: false
---
# auth0 events delete

Delete an event stream.

To delete interactively, use `auth0 events delete` with no arguments.

To delete non-interactively, supply the event id and the `--force` flag to skip confirmation.

## Usage
```
auth0 events delete [flags]
```

## Examples

```
  auth0 events delete
  auth0 events rm
  auth0 events delete <event-id>
  auth0 events delete <event-id> --force
  auth0 events delete <event-id> <event-id2> <event-idn>
  auth0 events delete <event-id> <event-id2> <event-idn> --force
```


## Flags

```
      --force   Skip confirmation.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 events create](auth0_events_create.md) - Create a new event stream
- [auth0 events delete](auth0_events_delete.md) - Delete an event stream
- [auth0 events list](auth0_events_list.md) - List your event streams
- [auth0 events show](auth0_events_show.md) - Show an event stream
- [auth0 events update](auth0_events_update.md) - Update an event


