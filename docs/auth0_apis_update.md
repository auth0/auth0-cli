---
layout: default
parent: auth0 apis
has_toc: false
---
# auth0 apis update

Update an API.

To update interactively, use `auth0 apis update` with no arguments.

To update non-interactively, supply the name, identifier, scopes, token lifetime and whether to allow offline access through the flags.

## Usage
```
auth0 apis update [flags]
```

## Examples

```
  auth0 apis update 
  auth0 apis update <api-id|api-audience>
  auth0 apis update <api-id|api-audience> --name myapi
  auth0 apis update <api-id|api-audience> --name myapi --token-lifetime 6100
  auth0 apis update <api-id|api-audience> --name myapi --token-lifetime 6100 --offline-access=false
  auth0 apis update <api-id|api-audience> --name myapi --token-lifetime 6100 --offline-access=false --scopes "letter:write,letter:read" --signing-alg "RS256"
  auth0 apis update <api-id|api-audience> -n myapi -t 6100 -o false -s "letter:write,letter:read" --signing-alg "RS256" --json
```


## Flags

```
      --json                 Output in json format.
  -n, --name string          Name of the API.
  -o, --offline-access       Whether Refresh Tokens can be issued for this API (true) or not (false).
  -s, --scopes strings       Comma-separated list of scopes (permissions).
      --signing-alg string   Algorithm used to sign JWTs. Can be HS256 or RS256. PS256 available via addon. (default "RS256")
  -l, --token-lifetime int   The amount of time in seconds that the token will be valid after being issued. Default value is 86400 seconds (1 day).
```


## Inherited Flags

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


