---
layout: default
---
# auth0 logs streams create splunk

Monitor real-time logs and display log analytics.

To create interactively, use `auth0 logs streams create splunk` with no arguments.

To create non-interactively, supply the log stream name and other information through the flags.

## Usage
```
auth0 logs streams create splunk [flags]
```

## Examples

```
  auth0 log streams create splunk
  auth0 log streams create splunk --name <name>
  auth0 log streams create splunk --name <name> --domain <domain>
  auth0 log streams create splunk --name <name> --domain <domain> --token <token>
  auth0 log streams create splunk --name <name> --domain <domain> --token <token> --port <port>
  auth0 log streams create splunk --name <name> --domain <domain> --token <token> --port <port> --secure
  auth0 log streams create splunk -n <name> -d <domain> -t <token> -p <port> -s
  auth0 log streams create splunk -n mylogstream -d "demo.splunk.com" -t "12a34ab5-c6d7-8901-23ef-456b7c89d0c1" -p "8088" -s false --json
```


## Flags

```
  -d, --domain string   The domain name of the splunk instance.
      --json            Output in json format.
  -n, --name string     The name of the log stream.
  -p, --port string     The port of the HTTP event collector.
  -s, --secure          This should be set to 'false' when using self-signed certificates.
  -t, --token string    Splunk event collector token.
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


