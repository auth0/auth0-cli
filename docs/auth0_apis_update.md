---
layout: default
---
# auth0 apis update

Update an API.

To update interactively, use `auth0 apis update` with no arguments.

To update non-interactively, supply the name, identifier, scopes, token lifetime and whether to allow offline access through the flags.

```
auth0 apis update [flags]
```


## Flags

```
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

## Examples

```
  auth0 apis update 
  auth0 apis update <id|audience>
  auth0 apis update <id|audience> --name myapi
  auth0 apis update -n myapi --token-expiration 6100
  auth0 apis update -n myapi -e 6100 --offline-access=true
```


## Related Commands

- [auth0 apis create](auth0_apis_create.md) - Create a new API
- [auth0 apis delete](auth0_apis_delete.md) - Delete an API
- [auth0 apis list](auth0_apis_list.md) - List your APIs
- [auth0 apis open](auth0_apis_open.md) - Open the settings page of an API
- [auth0 apis scopes](auth0_apis_scopes.md) - Manage resources for API scopes
- [auth0 apis show](auth0_apis_show.md) - Show an API
- [auth0 apis update](auth0_apis_update.md) - Update an API


