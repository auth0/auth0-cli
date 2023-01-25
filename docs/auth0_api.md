---
layout: default
has_toc: false
---
# auth0 api

Makes an authenticated HTTP request to the [Auth0 Management API](https://auth0.com/docs/api/management/v2) and returns the response as JSON.

Method argument is optional, defaults to `GET` for requests without data and `POST` for requests with data.

Additional scopes may need to be requested during authentication step via the `--scopes` flag. For example: `auth0 login --scopes read:client_grants`.

## Usage
```
auth0 api <method> <url-path> [flags]
```

## Examples

```
  auth0 api get "tenants/settings"
  auth0 api "stats/daily" -q "from=20221101" -q "to=20221118"
  auth0 api delete "actions/actions/<action-id>" --force
  auth0 api clients --data "{\"name\":\"ssoTest\",\"app_type\":\"sso_integration\"}"
  cat data.json | auth0 api post clients
```


## Flags

```
  -d, --data string            JSON data payload to send with the request. Data can be piped in as well instead of using this flag.
      --force                  Skip confirmation when using the delete method.
  -q, --query stringToString   Query params to send with the request. (default [])
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


