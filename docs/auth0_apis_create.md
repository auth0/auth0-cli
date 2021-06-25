---
layout: default
---
## auth0 apis create

Create a new API

### Synopsis

Create a new API.

```
auth0 apis create [flags]
```

### Examples

```
auth0 apis create 
auth0 apis create --name myapi
auth0 apis create -n myapi --identifier http://my-api
auth0 apis create -n myapi --token-expiration 6100
auth0 apis create -n myapi -e 6100 --offline-access=true
```

### Options

```
  -h, --help                 help for create
  -i, --identifier string    Identifier of the API. Cannot be changed once set.
  -n, --name string          Name of the API.
  -o, --offline-access       Whether Refresh Tokens can be issued for this API (true) or not (false).
  -s, --scopes strings       Comma-separated list of scopes (permissions).
  -l, --token-lifetime int   The amount of time in seconds that the token will be valid after being issued. Default value is 86400 seconds (1 day).
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

* [auth0 apis](auth0_apis.md)	 - Manage resources for APIs

