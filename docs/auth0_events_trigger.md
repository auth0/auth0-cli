---
layout: default
parent: auth0 events
has_toc: false
---
# auth0 events trigger

Manually trigger a test event for a specific Event Stream.

Use this to simulate an event like `user.created` or `user.updated`.
You can optionally provide a JSON payload from a file to simulate request content.

## Usage
```
auth0 events trigger <event-id> [flags]
```

## Examples

```
  auth0 events trigger <event-id> --type user.created
  auth0 events trigger <event-id> --type user.updated --payload ./test-event.json
```


## Flags

```
      --payload string   Path to a JSON file with a custom payload (optional)
      --type string      Type of event to simulate (e.g., user.created) [required]
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


