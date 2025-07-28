---
layout: default
parent: auth0 logs streams create
has_toc: false
---
# auth0 logs streams create eventgrid

A single service for routing events from any source to destination.

To create interactively, use `auth0 logs streams create eventgrid` with no arguments.

To create non-interactively, supply the log stream name and other information through the flags.

## Usage
```
auth0 logs streams create eventgrid [flags]
```

## Examples

```
  auth0 logs streams create eventgrid
  auth0 logs streams create eventgrid --name <name>
  auth0 logs streams create eventgrid --name <name> --azure-id <azure-id> 
  auth0 logs streams create eventgrid --name <name> --azure-id <azure-id> --azure-region <azure-region>
  auth0 logs streams create eventgrid --name <name> --azure-id <azure-id> --azure-region <azure-region> --azure-group <azure-group>
  auth0 logs streams create eventgrid --name <name> --azure-id <azure-id> --azure-region <azure-region> --azure-group <azure-group> --filters '[{"type":"category","name":"auth.login.fail"},{"type":"category","name":"auth.signup.fail"}]'
  auth0 logs streams create eventgrid --name <name> --azure-id <azure-id> --azure-region <azure-region> --azure-group <azure-group> --pii-config  '{"log_fields": ["first_name", "last_name"], "method": "hash", "algorithm": "xxhash"}'
  auth0 logs streams create eventgrid -n <name> -i <azure-id> -r <azure-region> -g <azure-group>
  auth0 logs streams create eventgrid -n mylogstream -i "b69a6835-57c7-4d53-b0d5-1c6ae580b6d5" -r northeurope -g "azure-logs-rg" --json
```


## Flags

```
  -g, --azure-group string    The name of the Azure resource group.
  -i, --azure-id string       Id of the Azure subscription.
  -r, --azure-region string   The region in which the Azure subscription is hosted.
  -m, --filters string        Events matching these filters will be delivered by the stream. (default "[]")
      --json                  Output in json format.
  -n, --name string           The name of the log stream.
  -c, --pii-config string     Specifies how PII fields are logged, Formatted as JSON. 
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

- [auth0 logs streams create datadog](auth0_logs_streams_create_datadog.md) - Create a new Datadog log stream
- [auth0 logs streams create eventbridge](auth0_logs_streams_create_eventbridge.md) - Create a new Amazon Event Bridge log stream
- [auth0 logs streams create eventgrid](auth0_logs_streams_create_eventgrid.md) - Create a new Azure Event Grid log stream
- [auth0 logs streams create http](auth0_logs_streams_create_http.md) - Create a new Custom Webhook log stream
- [auth0 logs streams create splunk](auth0_logs_streams_create_splunk.md) - Create a new Splunk log stream
- [auth0 logs streams create sumo](auth0_logs_streams_create_sumo.md) - Create a new Sumo Logic log stream


