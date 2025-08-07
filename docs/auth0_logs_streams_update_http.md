---
layout: default
parent: auth0 logs streams update
has_toc: false
---
# auth0 logs streams update http

Specify a URL you'd like Auth0 to post events to.

To update interactively, use `auth0 logs streams create http` with no arguments.

To update non-interactively, supply the log stream name and other information through the flags.

## Usage
```
auth0 logs streams update http [flags]
```

## Examples

```
  auth0 logs streams update http
  auth0 logs streams update http <log-stream-id> --name <name>
  auth0 logs streams update http <log-stream-id> --name <name> --endpoint <endpoint>
  auth0 logs streams update http <log-stream-id> --name <name> --endpoint <endpoint> --type <type>
  auth0 logs streams update http <log-stream-id> --name <name> --endpoint <endpoint> --type <type> --format <format>
  auth0 logs streams update http <log-stream-id> --name <name> --endpoint <endpoint> --type <type> --format <format> --authorization <authorization>
  auth0 logs streams update http <log-stream-id> -n <name> -e <endpoint> -t <type> -f <format> -a <authorization>
  auth0 logs streams update http <log-stream-id> -n mylogstream -e "https://example.com/webhook/logs" -t "application/json" -f "JSONLINES" -a "AKIAXXXXXXXXXXXXXXXX" --json
  auth0 logs streams update http <log-stream-id> -n mylogstream -e "https://example.com/webhook/logs" -t "application/json" -f "JSONLINES" -a "AKIAXXXXXXXXXXXXXXXX" --json-compact
```


## Flags

```
  -a, --authorization string   Sent in the HTTP "Authorization" header with each request.
  -e, --endpoint string        The HTTP endpoint to send streaming logs to.
  -f, --format string          The format of data sent over HTTP. Options are "JSONLINES", "JSONARRAY" or "JSONOBJECT"
      --json                   Output in json format.
      --json-compact           Output in compact json format.
  -n, --name string            The name of the log stream.
  -t, --type string            The "Content-Type" header to send over HTTP. Common value is "application/json".
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


