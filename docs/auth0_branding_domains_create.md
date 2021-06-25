---
layout: default
---
## auth0 branding domains create

Create a custom domain

### Synopsis

Create a custom domain.

```
auth0 branding domains create [flags]
```

### Examples

```
auth0 branding domains create 
auth0 branding domains create <id>
```

### Options

```
  -d, --domain string         Domain name.
  -h, --help                  help for create
  -i, --ip-header string      The HTTP header to fetch the client's IP address.
  -p, --policy string         The TLS version policy. Can be either 'compatible' or 'recommended'.
  -t, --type string           Custom domain provisioning type. Must be 'auth0' for Auth0-managed certs or 'self' for self-managed certs.
  -v, --verification string   Custom domain verification method. Must be 'txt'.
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

