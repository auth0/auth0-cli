---
layout: default
parent: auth0 logs streams create
has_toc: false
---
# auth0 logs streams create datadog

Build interactive dashboards and get alerted on critical issues.

To create interactively, use `auth0 logs streams create datadog` with no arguments.

To create non-interactively, supply the log stream name and other information through the flags.

## Usage
```
auth0 logs streams create datadog [flags]
```

## Examples

```
  auth0 logs streams create datadog
  auth0 logs streams create datadog --name <name>
  auth0 logs streams create datadog --name <name> --region <region>
  auth0 logs streams create datadog --name <name> --region <region> --api-key <api-key>
  auth0 logs streams create datadog -n <name> -r <region> -k <api-key>
  auth0 logs streams create datadog -n mylogstream -r eu -k 121233123455 --json
  auth0 logs streams create datadog -n mylogstream -r eu -k 121233123455 --json-compact
```


## Flags

```
  -k, --api-key string   Datadog API Key. To obtain a key, see the Datadog Authentication documentation (https://docs.datadoghq.com/api/latest/authentication).
      --json             Output in json format.
      --json-compact     Output in compact json format.
  -n, --name string      The name of the log stream.
  -r, --region string    The region in which the datadog dashboard is created.
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

- [auth0 logs streams create datadog](auth0_logs_streams_create_datadog.md) - Create a new Datadog log stream
- [auth0 logs streams create eventbridge](auth0_logs_streams_create_eventbridge.md) - Create a new Amazon Event Bridge log stream
- [auth0 logs streams create eventgrid](auth0_logs_streams_create_eventgrid.md) - Create a new Azure Event Grid log stream
- [auth0 logs streams create http](auth0_logs_streams_create_http.md) - Create a new Custom Webhook log stream
- [auth0 logs streams create splunk](auth0_logs_streams_create_splunk.md) - Create a new Splunk log stream
- [auth0 logs streams create sumo](auth0_logs_streams_create_sumo.md) - Create a new Sumo Logic log stream


