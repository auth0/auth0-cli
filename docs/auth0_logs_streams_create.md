---
layout: default
---
## auth0 logs streams create

Create a new log stream

### Synopsis

Create a new log stream.

```
auth0 logs streams create [flags]
```

### Examples

```
auth0 logs streams create
auth0 logs streams create -n mylogstream -t http --http-type application/json --http-format JSONLINES --http-auth 1343434
auth0 logs streams create -n mydatadog -t datadog --datadog-key 9999999 --datadog-id us
auth0 logs streams create -n myeventbridge -t eventbridge --eventbridge-id 999999999999 --eventbridge-region us-east-1
auth0 logs streams create -n test-splunk -t splunk --splunk-domain demo.splunk.com --splunk-token 12a34ab5-c6d7-8901-23ef-456b7c89d0c1 --splunk-port 8080 --splunk-secure=true
```

### Options

```
      --datadog-id string           The region in which datadog dashboard is created.
                                    if you are in the datadog EU site ('app.datadoghq.eu'), the Region should be EU otherwise it should be US.
      --datadog-key string          Datadog API Key. To obtain a key, see the Datadog Authentication documentation (https://docs.datadoghq.com/api/latest/authentication).
      --eventbridge-id string       Id of the AWS account.
      --eventbridge-region string   The region in which eventbridge will be created.
      --eventgrid-group string      The name of the Azure resource group.
      --eventgrid-id string         Id of the Azure subscription.
      --eventgrid-region string     The region in which the Azure subscription is hosted.
  -h, --help                        help for create
      --http-auth string            HTTP Authorization header.
      --http-endpoint string        HTTP endpoint.
      --http-format string          HTTP Content-Format header. Possible values: jsonlines, jsonarray, jsonobject.
      --http-type string            HTTP Content-Type header. Possible values: application/json.
  -n, --name string                 Name of the log stream.
      --splunk-domain string        The domain name of the splunk instance.
      --splunk-port string          The port of the HTTP event collector.
      --splunk-secure               This should be set to 'false' when using self-signed certificates.
      --splunk-token string         Splunk event collector token.
      --sumo-source string          Generated URL for your defined HTTP source in Sumo Logic.
  -t, --type string                 Type of the log stream. Possible values: http, eventbridge, eventgrid, datadog, splunk, sumo.
```

### Options inherited from parent commands

```
      --debug           Enable debug mode.
      --force           Skip confirmation.
      --format string   Command output format. Options: json.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```

### SEE ALSO

* [auth0 logs streams](auth0_logs_streams.md)	 - Manage resources for log streams

