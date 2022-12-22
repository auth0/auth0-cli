---
layout: default
---
# auth0 domains create

Create a custom domain.

To create interactively, use `auth0 domains create` with no arguments.

To create non-interactively, supply the domain name, type, policy and other information through the flags.

```
auth0 domains create [flags]
```


## Flags

```
  -d, --domain string         Domain name.
  -i, --ip-header string      The HTTP header to fetch the client's IP address.
      --json                  Output in json format.
  -p, --policy string         The TLS version policy. Can be either 'compatible' or 'recommended'.
  -t, --type string           Custom domain provisioning type. Must be 'auth0' for Auth0-managed certs or 'self' for self-managed certs.
  -v, --verification string   Custom domain verification method. Must be 'txt'.
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
  auth0 domains create
  auth0 domains create --domain <domain-name>
  auth0 domains create --domain <domain-name> --json
```


## Related Commands

- [auth0 domains create](auth0_domains_create.md) - Create a custom domain
- [auth0 domains delete](auth0_domains_delete.md) - Delete a custom domain
- [auth0 domains list](auth0_domains_list.md) - List your custom domains
- [auth0 domains show](auth0_domains_show.md) - Show a custom domain
- [auth0 domains update](auth0_domains_update.md) - Update a custom domain
- [auth0 domains verify](auth0_domains_verify.md) - Verify a custom domain


