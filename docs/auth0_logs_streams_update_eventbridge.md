---
layout: default
parent: auth0 logs streams update
has_toc: false
---
# auth0 logs streams update eventbridge

Stream real-time Auth0 data to over 15 targets like AWS Lambda.

To update interactively, use `auth0 logs streams create eventbridge` with no arguments.

To update non-interactively, supply the log stream name through the flag.

## Usage
```
auth0 logs streams update eventbridge [flags]
```

## Examples

```
  auth0 logs streams update eventbridge
  auth0 logs streams update eventbridge <log-stream-id> --name <name>
  auth0 logs streams update eventbridge <log-stream-id> --name <name>  --pii-config '{"log_fields": ["first_name", "last_name"], "method": "mask", "algorithm": "xxhash"}'
  auth0 logs streams update eventbridge <log-stream-id> -n <name> -p null
  auth0 logs streams update eventbridge <log-stream-id> -n mylogstream --json
```


## Flags

```
      --json                Output in json format.
  -n, --name string         The name of the log stream.
  -c, --pii-config string   Specifies how PII fields are logged, Formatted as JSON. 
                            including which fields to log (first_name, last_name, username, email, phone, address),the protection method (mask or hash), and the hashing algorithm (xxhash). 
                             Example : {"log_fields": ["first_name", "last_name"], "method": "mask", "algorithm": "xxhash"}. 
                             (default "{}")
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


