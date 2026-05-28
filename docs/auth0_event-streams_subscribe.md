---
layout: default
parent: auth0 event-streams
has_toc: false
---
# auth0 event-streams subscribe

Subscribe to events emitted by your tenant via Server-Sent Events (SSE).

By default, every received event is rendered as a single, color-coded summary line:
  TIME  TYPE  SOURCE  EVENT-ID

Use --verbose to also print the full JSON payload after each summary, or --json / --json-compact to emit raw JSON suitable for piping into `jq`.

Heartbeat (`offset-only`) messages are suppressed by default and surfaced via a periodic faint indicator and a final cursor on disconnect; pass --show-heartbeats to render each one. Press Ctrl+C to disconnect; a per-type summary and the latest cursor will be printed so you can resume with --from.

Run with --list-event-types to print every value accepted by --event-type.

## Usage
```
auth0 event-streams subscribe [flags]
```

## Examples

```
  auth0 event-streams subscribe
  auth0 event-streams subscribe --list-event-types
  auth0 event-streams subscribe --event-type user.created
  auth0 event-streams subscribe --event-type user.created --event-type user.updated
  auth0 event-streams subscribe --from-timestamp 2026-05-01T00:00:00Z
  auth0 event-streams subscribe --from <cursor>
  auth0 event-streams subscribe -v
  auth0 event-streams subscribe --show-heartbeats
  auth0 event-streams subscribe --output-file events.jsonl
  auth0 event-streams subscribe --json | jq .
```


## Flags

```
      --event-type strings            Event type(s) to listen for. Specify multiple times for multiple types (e.g. --event-type user.created --event-type user.updated). If not provided, all event types are streamed.
      --from offset                   Opaque cursor token representing the position in the stream. If not provided, the stream starts from the latest events. Use the offset printed when the connection ends to resume from where you left off.
      --from-timestamp string         RFC-3339 timestamp indicating where to start streaming events from. Use this on the initial query when no cursor (--from) is available; prefer --from on subsequent runs as it is more accurate.
      --json                          Output each event as JSON (one indented object per event).
      --json-compact                  Output each event as compact, single-line JSON (newline-delimited).
      --list-event-types              Print every event type accepted by --event-type and exit, without opening a subscription.
      --max-reconnects int            Maximum number of transparent mid-stream reconnect attempts. 0 keeps the SDK default (5). Ignored when --no-reconnect is set.
      --no-reconnect                  Disable transparent mid-stream reconnection. By default the SDK reconnects up to 5 times when the connection drops, preserving the cursor so no events are missed.
      --output-file string            Append every received event as a JSON line to this file (raw payload). Independent of the stdout format.
      --show-heartbeats offset-only   Show every offset-only heartbeat as its own line. By default heartbeats are silently tracked and only the latest cursor is reported on disconnect.
  -v, --verbose                       Print the full JSON payload after each event summary line.
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
- [auth0 event-streams subscribe](auth0_event-streams_subscribe.md) - Subscribe to live events via Server-Sent Events (SSE)
- [auth0 event-streams trigger](auth0_event-streams_trigger.md) - Trigger a test event for an event stream
- [auth0 event-streams update](auth0_event-streams_update.md) - Update an event stream


