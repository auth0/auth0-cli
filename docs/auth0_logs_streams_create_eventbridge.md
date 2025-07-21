---
layout: default
parent: auth0 logs streams create
has_toc: false
---
# auth0 logs streams create eventbridge

Stream real-time Auth0 data to over 15 targets like AWS Lambda.

To create interactively, use `auth0 logs streams create eventbridge` with no arguments.

To create non-interactively, supply the log stream name and other information through the flags.

## Usage
```
auth0 logs streams create eventbridge [flags]
```

## Examples

```
  auth0 logs streams create eventbridge
  auth0 logs streams create eventbridge --name <name>
  auth0 logs streams create eventbridge --name <name> --aws-id <aws-id>
  auth0 logs streams create eventbridge --name <name> --aws-id <aws-id> --aws-region <aws-region>
  auth0 logs streams create eventbridge --name <name> --aws-id <aws-id> --aws-region <aws-region> --pii-config '{"log_fields": ["first_name", "last_name"], "method": "mask", "algorithm": "xxhash"}'
  auth0 logs streams create eventbridge -n <name> -i <aws-id> -r <aws-region>
  auth0 logs streams create eventbridge -n mylogstream -i 999999999999 -r "eu-west-1" --json
```


## Flags

```
  -i, --aws-id string       ID of the AWS account.
  -r, --aws-region string   The AWS region in which eventbridge will be created, e.g. 'us-east-2'.
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

- [auth0 logs streams create datadog](auth0_logs_streams_create_datadog.md) - Create a new Datadog log stream
- [auth0 logs streams create eventbridge](auth0_logs_streams_create_eventbridge.md) - Create a new Amazon Event Bridge log stream
- [auth0 logs streams create eventgrid](auth0_logs_streams_create_eventgrid.md) - Create a new Azure Event Grid log stream
- [auth0 logs streams create http](auth0_logs_streams_create_http.md) - Create a new Custom Webhook log stream
- [auth0 logs streams create splunk](auth0_logs_streams_create_splunk.md) - Create a new Splunk log stream
- [auth0 logs streams create sumo](auth0_logs_streams_create_sumo.md) - Create a new Sumo Logic log stream


