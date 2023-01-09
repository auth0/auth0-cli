---
layout: default
---
# auth0 apis create

Create a new API.

To create interactively, use `auth0 apis create` with no flags.

To create non-interactively, supply the name, identifier, scopes, token lifetime and whether to allow offline access through the flags.

## Usage
```
auth0 apis create [flags]
```

## Examples

```
  auth0 apis create 
  auth0 apis create --name myapi
  auth0 apis create --name myapi --identifier http://my-api
  auth0 apis create --name myapi --identifier http://my-api --token-lifetime 6100
  auth0 apis create --name myapi --identifier http://my-api --token-lifetime 6100 --offline-access
  auth0 apis create --name myapi --identifier http://my-api --token-lifetime 6100 --offline-access false --scopes "letter:write,letter:read"
  auth0 apis create -n myapi -i http://my-api -t 6100 -o false -s "letter:write,letter:read" --json
```


## Flags

```
  -i, --identifier string    Identifier of the API. Cannot be changed once set.
      --json                 Output in json format.
  -n, --name string          Name of the API.
  -o, --offline-access       Whether Refresh Tokens can be issued for this API (true) or not (false).
  -s, --scopes strings       Comma-separated list of scopes (permissions).
  -l, --token-lifetime int   The amount of time in seconds that the token will be valid after being issued. Default value is 86400 seconds (1 day).
```


## InheritedFlags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 apis create](auth0_apis_create.md) - Create a new API
- [auth0 apis delete](auth0_apis_delete.md) - Delete an API
- [auth0 apis list](auth0_apis_list.md) - List your APIs
- [auth0 apis open](auth0_apis_open.md) - Open the settings page of an API
- [auth0 apis scopes](auth0_apis_scopes.md) - Manage resources for API scopes
- [auth0 apis show](auth0_apis_show.md) - Show an API
- [auth0 apis update](auth0_apis_update.md) - Update an API


