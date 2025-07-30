---
layout: default
parent: auth0 events deliveries
has_toc: false
---
# auth0 events deliveries show

Displays metadata, attempts, and event payload for a specific delivery
associated with an event stream.

If stream ID or delivery ID is not provided, you will be prompted to select them interactively.

## Usage
```
auth0 events deliveries show [stream-id] [delivery-id] [flags]
```

## Examples

```
  auth0 events deliveries show
  auth0 events deliveries show <stream-id>
  auth0 events deliveries show <stream-id> <delivery-id>
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

- [auth0 events deliveries list](auth0_events_deliveries_list.md) - List failed deliveries for an event stream
- [auth0 events deliveries show](auth0_events_deliveries_show.md) - Show details for a specific delivery


