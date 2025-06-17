---
layout: default
parent: auth0 domains
has_toc: false
---
# auth0 domains create

Create a custom domain.

To create interactively, use `auth0 domains create` with no arguments.

To create non-interactively, supply the domain name, type, policy and other information through the flags.

## Usage
```
auth0 domains create [flags]
```

## Examples

```
  auth0 domains create
  auth0 domains create --domain <domain-name>
  auth0 domains create --domain <domain-name> --policy recommended
  auth0 domains create --domain <domain-name> --policy recommended --metadata '{"key1":"value1","key2":"value2"}' 
  auth0 domains create --domain <domain-name> --policy recommended --type auth0
  auth0 domains create --domain <domain-name> --policy recommended --type auth0 --ip-header "cf-connecting-ip"
  auth0 domains create -d <domain-name> -p recommended -t auth0 -i "cf-connecting-ip" --json
```


## Flags

```
  -d, --domain string         Domain name.
  -i, --ip-header string      The HTTP header to fetch the client's IP address.
      --json                  Output in json format.
  -m, --metadata string       The Custom Domain Metadata, formatted as JSON.
  -p, --policy string         The TLS version policy. Can be either 'compatible' or 'recommended'.
  -t, --type string           Custom domain provisioning type. Must be 'auth0' for Auth0-managed certs or 'self' for self-managed certs.
  -v, --verification string   *DEPRECATED* Custom domain verification method. Must be 'txt'.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 domains create](auth0_domains_create.md) - Create a custom domain
- [auth0 domains delete](auth0_domains_delete.md) - Delete a custom domain
- [auth0 domains list](auth0_domains_list.md) - List your custom domains
- [auth0 domains show](auth0_domains_show.md) - Show a custom domain
- [auth0 domains update](auth0_domains_update.md) - Update a custom domain
- [auth0 domains verify](auth0_domains_verify.md) - Verify a custom domain


