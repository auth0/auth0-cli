---
layout: default
---
# auth0 domains update

Update a custom domain.

To update interactively, use `auth0 domains update` with no arguments.

To update non-interactively, supply the domain name, type, policy and other information through the flags.

```
auth0 domains update [flags]
```


## Flags

```
  -i, --ip-header string   The HTTP header to fetch the client's IP address.
      --json               Output in json format.
  -p, --policy string      The TLS version policy. Can be either 'compatible' or 'recommended'.
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
  auth0 domains update
  auth0 domains update <id> --policy compatible
  auth0 domains update <id> -p compatible --ip-header "cf-connecting-ip"
```


## Related Commands

- [auth0 domains create](auth0_domains_create.md) - Create a custom domain
- [auth0 domains delete](auth0_domains_delete.md) - Delete a custom domain
- [auth0 domains list](auth0_domains_list.md) - List your custom domains
- [auth0 domains show](auth0_domains_show.md) - Show a custom domain
- [auth0 domains update](auth0_domains_update.md) - Update a custom domain
- [auth0 domains verify](auth0_domains_verify.md) - Verify a custom domain


