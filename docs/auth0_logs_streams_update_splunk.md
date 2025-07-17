---
layout: default
parent: auth0 logs streams update
has_toc: false
---
# auth0 logs streams update splunk

Monitor real-time logs and display log analytics.

To update interactively, use `auth0 logs streams create splunk` with no arguments.

To update non-interactively, supply the log stream name and other information through the flags.

## Usage
```
auth0 logs streams update splunk [flags]
```

## Examples

```
  auth0 log streams update splunk
  auth0 log streams update splunk <log-stream-id> --name <name>
  auth0 log streams update splunk <log-stream-id> --name <name> --domain <domain>
  auth0 log streams update splunk <log-stream-id> --name <name> --domain <domain> --token <token>
  auth0 log streams update splunk <log-stream-id> --name <name> --domain <domain> --token <token> --port <port>
  auth0 log streams update splunk <log-stream-id> --name <name> --domain <domain> --token <token> --port <port> --pii-config "{\"log_fields\": [\"first_name\", \"last_name\"], \"method\": \"mask\", \"algorithm\": \"xxhash\"}"
  auth0 log streams update splunk <log-stream-id> --name <name> --domain <domain> --token <token> --port <port> --secure=false
  auth0 log streams update splunk <log-stream-id> -n <name> -d <domain> -t <token> -p <port> -s -c null
  auth0 log streams update splunk <log-stream-id> -n mylogstream -d "demo.splunk.com" -t "12a34ab5-c6d7-8901-23ef-456b7c89d0c1" -p "8088" -s=false --json
```


## Flags

```
  -d, --domain string       The domain name of the splunk instance.
      --json                Output in json format.
  -n, --name string         The name of the log stream.
  -c, --pii-config string   Specifies how PII fields are logged, Formatted as JSON. 
                            including which fields to log (first_name, last_name, username, email, phone, address),the protection method (mask or hash), and the hashing algorithm (xxhash). 
                             Example : {"log_fields": ["first_name", "last_name"], "method": "mask", "algorithm": "xxhash"}. 
                            
  -p, --port string         The port of the HTTP event collector.
  -s, --secure              This should be set to 'false' when using self-signed certificates.
  -t, --token string        Splunk event collector token.
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


