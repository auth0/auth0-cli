---
layout: default
parent: auth0 events
has_toc: false
---
# auth0 events stats

Retrieve metrics over time for a given event stream, including 
successful and failed delivery counts. Supports custom date range filtering.

## Usage
```
auth0 events stats [stream-id] [flags]
```

## Examples

```
  auth0 events stats
  auth0 events stats <stream-id>
  auth0 events stats <stream-id> --from 2025-07-15 --to 2025-07-29
```


## Flags

```
  -f, --from string   Start date for stats (e.g. 2025-07-15, -3d)
      --json          Output in json format.
  -t, --to string     End date for stats (e.g. 2025-07-29, now)
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
- [auth0 events deliveries](auth0_events_deliveries.md) - Manage event stream deliveries
- [auth0 events list](auth0_events_list.md) - List your event streams
- [auth0 events redeliver](auth0_events_redeliver.md) - Retry one or more event deliveries for a given stream
- [auth0 events redeliver-many](auth0_events_redeliver-many.md) - Bulk retry failed event deliveries using filters
- [auth0 events show](auth0_events_show.md) - Show an event stream
- [auth0 events stats](auth0_events_stats.md) - View delivery stats for an event stream
- [auth0 events trigger](auth0_events_trigger.md) - Trigger a test event for an event stream
- [auth0 events update](auth0_events_update.md) - Update an event


