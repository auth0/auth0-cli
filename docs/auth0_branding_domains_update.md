---
layout: default
---
## auth0 branding domains update

Update a custom domain

### Synopsis

Update a custom domain.

```
auth0 branding domains update [flags]
```

### Examples

```
auth0 branding domains update
auth0 branding domains update <id> --policy compatible
auth0 branding domains update <id> -p compatible --ip-header "cf-connecting-ip"
```

### Options

```
  -h, --help               help for update
  -i, --ip-header string   The HTTP header to fetch the client's IP address.
  -p, --policy string      The TLS version policy. Can be either 'compatible' or 'recommended'.
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

* [auth0 branding domains](auth0_branding_domains.md)	 - Manage custom domains

