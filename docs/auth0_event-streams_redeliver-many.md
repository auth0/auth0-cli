---
layout: default
parent: auth0 event-streams
has_toc: false
---
# auth0 event-streams redeliver-many

Retry multiple failed event deliveries for a given event stream. 
You can filter by event type and date range. 
All filters are combined using AND logic. 
If no filters are passed, all failed events are retried

## Usage
```
auth0 event-streams redeliver-many [stream-id] [flags]
```

## Examples

```
  auth0 events redeliver-many
  auth0 events redeliver-many <stream-id>
  auth0 events redeliver-many <stream-id> --type=user.created,user.deleted --from=-2d
```


## Flags

```
  -f, --from string   Start date for filtering (e.g. 2025-07-25, -2d, yesterday)
  -t, --to string     End date for filtering (e.g. 2025-07-29, today)
      --type string   Comma-separated event types (e.g. user.created,user.deleted)
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
- [auth0 event-streams update](auth0_event-streams_update.md) - Update an event


