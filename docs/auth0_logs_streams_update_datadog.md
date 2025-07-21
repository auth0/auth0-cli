---
layout: default
parent: auth0 logs streams update
has_toc: false
---
# auth0 logs streams update datadog

Build interactive dashboards and get alerted on critical issues.

To update interactively, use `auth0 logs streams create datadog` with no arguments.

To update non-interactively, supply the log stream name and other information through the flags.

## Usage
```
auth0 logs streams update datadog [flags]
```

## Examples

```
  auth0 logs streams update datadog
  auth0 logs streams update datadog <log-stream-id> --name <name>
  auth0 logs streams update datadog <log-stream-id> --name <name> --region <region>
  auth0 logs streams update datadog <log-stream-id> --name <name> --region <region> --api-key <api-key>
  auth0 logs streams update datadog <log-stream-id> --name <name> --region <region> --api-key <api-key> --pii-config '{"log_fields": ["first_name", "last_name"], "method": "mask", "algorithm": "xxhash"}'
  auth0 logs streams update datadog <log-stream-id> -n <name> -r <region> -k <api-key> -c null
  auth0 logs streams update datadog <log-stream-id> -n mylogstream -r eu -k 121233123455 --json
```


## Flags

```
  -k, --api-key string      Datadog API Key. To obtain a key, see the Datadog Authentication documentation (https://docs.datadoghq.com/api/latest/authentication).
      --json                Output in json format.
  -n, --name string         The name of the log stream.
  -c, --pii-config string   Specifies how PII fields are logged, Formatted as JSON. 
                            including which fields to log (first_name, last_name, username, email, phone, address),the protection method (mask or hash), and the hashing algorithm (xxhash). 
                             Example : {"log_fields": ["first_name", "last_name"], "method": "mask", "algorithm": "xxhash"}. 
                             (default "{}")
  -r, --region string       The region in which the datadog dashboard is created.
                            If you are in the datadog EU site ('app.datadoghq.eu'), the Region should be EU otherwise it should be US.
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


