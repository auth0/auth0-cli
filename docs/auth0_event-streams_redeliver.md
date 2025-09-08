---
layout: default
parent: auth0 event-streams
has_toc: false
---
# auth0 event-streams redeliver

Retry one or more failed event deliveries for a given event stream. 
If no delivery IDs are provided, you'll be prompted to select from recent failed deliveries.

## Usage
```
auth0 event-streams redeliver [stream-id] [comma-separated-delivery-ids] [flags]
```

## Examples

```
  auth0 events redeliver
  auth0 events redeliver <stream-id>
  auth0 events redeliver <stream-id> evt_abc123,evt_def456
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


