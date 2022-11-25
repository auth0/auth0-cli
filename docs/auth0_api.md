---
layout: default
---
## auth0 api

Makes an authenticated HTTP request to the Auth0 Management API

### Synopsis

Makes an authenticated HTTP request to the Auth0 Management API and prints the response as JSON.

The method argument is optional, and when you don’t specify it, the command defaults to GET for requests without data
and POST for requests with data.

Auth0 Management API Docs:
  https://auth0.com/docs/api/management/v2

Available Methods:
  GET, POST, PUT, PATCH, DELETE

```
auth0 api <method> <uri> [flags]
```

### Examples

```
auth0 api "/organizations?include_totals=true"
auth0 api get "/organizations?include_totals=true"
auth0 api clients --data "{\"name\":\"apiTest\"}"

```

### Options

```
  -d, --data string   JSON data payload to send with the request.
  -h, --help          help for api
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

* [auth0](/auth0-cli/)	 - Supercharge your development workflow.

