---
layout: default
parent: auth0 logs streams create
has_toc: false
---
# auth0 logs streams create sumo

Visualize logs and detect threats faster with security insights.

To create interactively, use `auth0 logs streams create sumo` with no arguments.

To create non-interactively, supply the log stream name and other information through the flags.

## Usage
```
auth0 logs streams create sumo [flags]
```

## Examples

```
  auth0 logs streams create sumo
  auth0 logs streams create sumo --name <name>
  auth0 logs streams create sumo --name <name> --source <source>
  auth0 logs streams create sumo -n <name> -s <source>
  auth0 logs streams create sumo -n "mylogstream" -s "demo.sumo.com" --json
```


## Flags

```
      --json            Output in json format.
  -n, --name string     The name of the log stream.
  -s, --source string   Generated URL for your defined HTTP source in Sumo Logic.
```


## InheritedFlags

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


