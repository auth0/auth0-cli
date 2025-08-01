---
layout: default
parent: auth0 logs streams update
has_toc: false
---
# auth0 logs streams update eventgrid

A single service for routing events from any source to destination.

To update interactively, use `auth0 logs streams create eventgrid` with no arguments.

To update non-interactively, supply the log stream name through the flag.

## Usage
```
auth0 logs streams update eventgrid [flags]
```

## Examples

```
  auth0 logs streams update eventgrid
  auth0 logs streams update eventgrid <log-stream-id> --name <name>
  auth0 logs streams update eventgrid <log-stream-id> -n <name>
  auth0 logs streams update eventgrid <log-stream-id> -n mylogstream --json
  auth0 logs streams update eventgrid <log-stream-id> -n mylogstream --json-compact
```


## Flags

```
      --json           Output in json format.
      --json-compact   Output in compact json format.
  -n, --name string    The name of the log stream.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 logs streams update datadog](auth0_logs_streams_update_datadog.md) - Update an existing Datadog log stream
- [auth0 logs streams update eventbridge](auth0_logs_streams_update_eventbridge.md) - Update an existing Amazon Event Bridge log stream
- [auth0 logs streams update eventgrid](auth0_logs_streams_update_eventgrid.md) - Update an existing Azure Event Grid log stream
- [auth0 logs streams update http](auth0_logs_streams_update_http.md) - Update an existing Custom Webhook log stream
- [auth0 logs streams update splunk](auth0_logs_streams_update_splunk.md) - Update an existing Splunk log stream
- [auth0 logs streams update sumo](auth0_logs_streams_update_sumo.md) - Update an existing Sumo Logic log stream


