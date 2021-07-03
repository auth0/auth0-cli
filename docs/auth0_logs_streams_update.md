---
layout: default
---
## auth0 logs streams update

Update a log stream

### Synopsis

Update a log stream.

```
auth0 logs streams update [flags]
```

### Examples

```
auth0 logs streams update
auth0 logs streams update <id> --name mylogstream
auth0 logs streams update <id> -n mylogstream --type http
auth0 logs streams update <id> -n mylogstream -t http --http-type application/json --http-format JSONLINES
auth0 logs streams update <id> -n mydatadog -t datadog --datadog-key 9999999 --datadog-id us
auth0 logs streams update <id> -n myeventbridge -t eventbridge
```

### Options

```
      --datadog-id string      The region in which datadog dashboard is created.
                               if you are in the datadog EU site ('app.datadoghq.eu'), the Region should be EU otherwise it should be US.
      --datadog-key string     Datadog API Key. To obtain a key, see the Datadog Authentication documentation (https://docs.datadoghq.com/api/latest/authentication).
  -h, --help                   help for update
      --http-auth string       HTTP Authorization header.
      --http-endpoint string   HTTP endpoint.
      --http-format string     HTTP Content-Format header. Possible values: jsonlines, jsonarray, jsonobject.
      --http-type string       HTTP Content-Type header. Possible values: application/json.
  -n, --name string            Name of the log stream.
      --splunk-domain string   The domain name of the splunk instance.
      --splunk-port string     The port of the HTTP event collector.
      --splunk-secure          This should be set to 'false' when using self-signed certificates.
      --splunk-token string    Splunk event collector token.
      --sumo-source string     Generated URL for your defined HTTP source in Sumo Logic.
  -t, --type string            Type of the log stream. Possible values: http, eventbridge, eventgrid, datadog, splunk, sumo.
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

