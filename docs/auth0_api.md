---
layout: default
has_toc: false
---
# auth0 api

Makes an authenticated HTTP request to the [Auth0 Management API](https://auth0.com/docs/api/management/v2) and returns the response as JSON.

Method argument is optional, defaults to `GET` for requests without data and `POST` for requests with data.

Additional scopes may need to be requested during authentication step via the `--scopes` flag. For example: `auth0 login --scopes read:client_grants`.

Like the typed commands, secrets such as `client_secret` and `signing_keys` are removed from the response unless you pass `--reveal-secrets`.

## Usage
```
auth0 api <method> <url-path> [flags]
```

## Examples

```
  auth0 api get "tenants/settings"
  auth0 api "stats/daily" -q "from=20221101" -q "to=20221118"
  auth0 api "clients" -q "fields=name,app_type,callbacks"
  auth0 api delete "actions/actions/<action-id>" --force
  auth0 api clients --data "{\"name\":\"ssoTest\",\"app_type\":\"sso_integration\"}"
  cat data.json | auth0 api post clients
  auth0 api get "clients/<client-id>" --reveal-secrets
```


## Flags

```
  -d, --data string         JSON data payload to send with the request. Pass inline JSON, @file to read from a file, or @- to read from stdin. Data can also be piped in instead of using this flag.
      --force               Skip confirmation when using the delete method.
      --json                Output in json format.
      --json-compact        Output in compact json format.
  -q, --query stringArray   Query params to send with the request. A comma-separated value is sent as a single param, for example -q "fields=a,b,c". Repeat the flag to send a param more than once, for example -q "fields=a" -q "fields=b".
  -r, --reveal-secrets      Display the application secrets ('signing_keys', 'client_secret') as part of the command output.
```


## Inherited Flags

```
      --agent-mode      Output JSON, disable prompts and colors. Auto-enabled for AI agents; set AUTH0_AGENT_MODE=false to disable.
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


