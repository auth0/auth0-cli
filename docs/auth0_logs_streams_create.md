---
layout: default
---
# auth0 logs streams create

Create a new log stream.

To create interactively, use `auth0 logs streams create` with no arguments.

To create non-interactively, supply the log stream name, type and other information through the flags.

## Usage
```
auth0 logs streams create [flags]
```

## Examples

```
  auth0 logs streams create

  # Custom Webhook
  auth0 logs streams create -n mylogstream -t http --http-type application/json --http-format JSONLINES --http-auth 1343434

  # Datadog 
  auth0 logs streams create -n mydatadog -t datadog --datadog-key 9999999 --datadog-id us

  # EventBridge
  auth0 logs streams create -n myeventbridge -t eventbridge --eventbridge-id 999999999999 --eventbridge-region us-east-1

  # Splunk
  auth0 logs streams create -n test-splunk -t splunk --splunk-domain demo.splunk.com --splunk-token 12a34ab5-c6d7-8901-23ef-456b7c89d0c1 --splunk-port 8080 --splunk-secure=true
```


## Flags

```
      --datadog-id string           The region in which datadog dashboard is created.
                                    if you are in the datadog EU site ('app.datadoghq.eu'), the Region should be EU otherwise it should be US.
      --datadog-key string          Datadog API Key. To obtain a key, see the Datadog Authentication documentation (https://docs.datadoghq.com/api/latest/authentication).
      --eventbridge-id string       Id of the AWS account.
      --eventbridge-region string   The region in which eventbridge will be created.
      --eventgrid-group string      The name of the Azure resource group.
      --eventgrid-id string         Id of the Azure subscription.
      --eventgrid-region string     The region in which the Azure subscription is hosted.
      --http-auth string            HTTP Authorization header.
      --http-endpoint string        HTTP endpoint.
      --http-format string          HTTP Content-Format header. Possible values: jsonlines, jsonarray, jsonobject.
      --http-type string            HTTP Content-Type header. Possible values: application/json.
      --json                        Output in json format.
  -n, --name string                 Name of the log stream.
      --splunk-domain string        The domain name of the splunk instance.
      --splunk-port string          The port of the HTTP event collector.
      --splunk-secure               This should be set to 'false' when using self-signed certificates.
      --splunk-token string         Splunk event collector token.
      --sumo-source string          Generated URL for your defined HTTP source in Sumo Logic.
  -t, --type string                 Type of the log stream. Possible values: http, eventbridge, eventgrid, datadog, splunk, sumo.
```


## InheritedFlags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 logs streams create](auth0_logs_streams_create.md) - Create a new log stream
- [auth0 logs streams delete](auth0_logs_streams_delete.md) - Delete a log stream
- [auth0 logs streams list](auth0_logs_streams_list.md) - List all log streams
- [auth0 logs streams open](auth0_logs_streams_open.md) - Open the settings page of a log stream
- [auth0 logs streams show](auth0_logs_streams_show.md) - Show a log stream by Id
- [auth0 logs streams update](auth0_logs_streams_update.md) - Update a log stream


