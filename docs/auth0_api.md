---
layout: default
---
## auth0 api

Makes an authenticated HTTP request to the Auth0 Management API

### Synopsis

Makes an authenticated HTTP request to the Auth0 Management API and prints the response as JSON.

The method argument is optional, and when you don’t specify it, the command defaults to GET for requests without data and POST for requests with data.

Auth0 Management API Docs:
  https://auth0.com/docs/api/management/v2

Available Methods:
  get, post, put, patch, delete

```
auth0 api <method> <uri> [flags]
```

### Examples

```
auth0 api "stats/daily" -q "from=20221101" -q "to=20221118"
auth0 api get "tenants/settings"
auth0 api clients --data "{\"name\":\"ssoTest\",\"app_type\":\"sso_integration\"}"

```

### Options

```
  -d, --data string            JSON data payload to send with the request.
  -h, --help                   help for api
  -q, --query stringToString   Query params to send with the request. (default [])
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

