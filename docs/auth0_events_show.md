---
layout: default
parent: auth0 events
has_toc: false
---
# auth0 events show

Display the name, type, status, subscriptions and other information about an event stream

## Usage
```
auth0 events show [flags]
```

## Examples

```
  auth0 events show
  auth0 events show <event-id>
  auth0 events show <event-id> --json
```


## Flags

```
      --json   Output in json format.
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


