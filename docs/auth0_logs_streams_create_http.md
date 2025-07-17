---
layout: default
parent: auth0 logs streams create
has_toc: false
---
# auth0 logs streams create http

Specify a URL you'd like Auth0 to post events to.

To create interactively, use `auth0 logs streams create http` with no arguments.

To create non-interactively, supply the log stream name and other information through the flags.

## Usage
```
auth0 logs streams create http [flags]
```

## Examples

```
  auth0 logs streams create http
  auth0 logs streams create http --name <name>
  auth0 logs streams create http --name <name> --endpoint <endpoint>
  auth0 logs streams create http --name <name> --endpoint <endpoint> --type <type>
  auth0 logs streams create http --name <name> --endpoint <endpoint> --type <type> --format <format>
  auth0 logs streams create http --name <name> --endpoint <endpoint> --type <type> --format <format> --pii-config "{\"log_fields\": [\"first_name\", \"last_name\"], \"method\": \"hash\", \"algorithm\": \"xxhash\"}"
  auth0 logs streams create http --name <name> --endpoint <endpoint> --type <type> --format <format> --authorization <authorization>
  auth0 logs streams create http -n <name> -e <endpoint> -t <type> -f <format> -a <authorization>
  auth0 logs streams create http -n mylogstream -e "https://example.com/webhook/logs" -t "application/json" -f "JSONLINES" -a "AKIAXXXXXXXXXXXXXXXX" --json
```


## Flags

```
  -a, --authorization string   Sent in the HTTP "Authorization" header with each request.
  -e, --endpoint string        The HTTP endpoint to send streaming logs to.
  -f, --format string          The format of data sent over HTTP. Options are "JSONLINES", "JSONARRAY" or "JSONOBJECT"
      --json                   Output in json format.
  -n, --name string            The name of the log stream.
  -c, --pii-config string      Specifies how PII fields are logged, Formatted as JSON. 
                               including which fields to log (first_name, last_name, username, email, phone, address),the protection method (mask or hash), and the hashing algorithm (xxhash). 
                                Example : {"log_fields": ["first_name", "last_name"], "method": "mask", "algorithm": "xxhash"}. 
                                (default "{}")
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

- [auth0 logs streams create datadog](auth0_logs_streams_create_datadog.md) - Create a new Datadog log stream
- [auth0 logs streams create eventbridge](auth0_logs_streams_create_eventbridge.md) - Create a new Amazon Event Bridge log stream
- [auth0 logs streams create eventgrid](auth0_logs_streams_create_eventgrid.md) - Create a new Azure Event Grid log stream
- [auth0 logs streams create http](auth0_logs_streams_create_http.md) - Create a new Custom Webhook log stream
- [auth0 logs streams create splunk](auth0_logs_streams_create_splunk.md) - Create a new Splunk log stream
- [auth0 logs streams create sumo](auth0_logs_streams_create_sumo.md) - Create a new Sumo Logic log stream


