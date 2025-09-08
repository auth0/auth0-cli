---
layout: default
parent: auth0 event-streams deliveries
has_toc: false
---
# auth0 event-streams deliveries list

List all failed delivery attempts associated with a specific event stream.
Optionally filter by event type(s) using the --type flag.

## Usage
```
auth0 event-streams deliveries list [event-stream-id] [flags]
```

## Examples

```
  auth0 events deliveries list
  auth0 events deliveries list <event-stream-id>
  auth0 events deliveries list <event-stream-id> --type user.created
  auth0 events deliveries list --json
  auth0 events deliveries list --csv
  auth0 events deliveries list --picker
```


## Flags

```
      --csv            Output in CSV format.
  -f, --from string    Filter deliveries from this date (e.g. 2025-07-25, yesterday, -2d)
      --json           Output in JSON format.
  -n, --n int          Number of results to return, defaults to 50 (default 50)
  -p, --picker         Allows to toggle from list of events and view a selected event in detail
  -t, --to string      Filter deliveries up to this date (e.g. 2025-07-29, today)
      --type strings   Filter deliveries by one or more event types (comma-separated)
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 event-streams deliveries list](auth0_event-streams_deliveries_list.md) - List failed deliveries for an event stream
- [auth0 event-streams deliveries show](auth0_event-streams_deliveries_show.md) - Show details for a specific delivery


