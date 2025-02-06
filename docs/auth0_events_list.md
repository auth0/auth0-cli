---
layout: default
parent: auth0 events
has_toc: false
---
# auth0 events list

List your existing event streams. To create one, run: `auth0 events create`.

## Usage
```
auth0 events list [flags]
```

## Examples

```
  auth0 events list
  auth0 events ls
  auth0 events ls --json
  auth0 events ls --csv
```


## Flags

```
      --csv    Output in csv format.
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


