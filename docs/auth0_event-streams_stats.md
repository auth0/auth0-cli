---
layout: default
parent: auth0 event-streams
has_toc: false
---
# auth0 event-streams stats

Retrieve metrics over time for a given event stream, including 
successful and failed delivery counts. Supports custom date range filtering.

## Usage
```
auth0 event-streams stats [stream-id] [flags]
```

## Examples

```
  auth0 event-streams stats
  auth0 event-streams stats <stream-id>
  auth0 event-streams stats <stream-id> --from 2025-07-15 --to 2025-07-29
```


## Flags

```
  -f, --from string   Start date for stats (e.g. 2025-07-15, -3d)
      --json          Output in json format.
  -t, --to string     End date for stats (e.g. 2025-07-29)
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 event-streams create](auth0_event-streams_create.md) - Create a new event stream
- [auth0 event-streams delete](auth0_event-streams_delete.md) - Delete an event stream
- [auth0 event-streams deliveries](auth0_event-streams_deliveries.md) - Manage event stream deliveries
- [auth0 event-streams list](auth0_event-streams_list.md) - List your event streams
- [auth0 event-streams redeliver](auth0_event-streams_redeliver.md) - Retry one or more event deliveries for a given stream
- [auth0 event-streams redeliver-many](auth0_event-streams_redeliver-many.md) - Bulk retry failed event deliveries using filters
- [auth0 event-streams show](auth0_event-streams_show.md) - Show an event stream
- [auth0 event-streams stats](auth0_event-streams_stats.md) - View delivery stats for an event stream
- [auth0 event-streams trigger](auth0_event-streams_trigger.md) - Trigger a test event for an event stream
- [auth0 event-streams update](auth0_event-streams_update.md) - Update an event stream


