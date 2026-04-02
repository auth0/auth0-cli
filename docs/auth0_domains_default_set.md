---
layout: default
parent: auth0 domains default
has_toc: false
---
# auth0 domains default set

Set the default custom domain for the tenant.

To set interactively, use `auth0 domains default set` with no arguments.

To set non-interactively, supply the domain name as an argument or through the flag.

## Usage
```
auth0 domains default set [flags]
```

## Examples

```
  auth0 domains default set
  auth0 domains default set <domain>
  auth0 domains default set --domain <domain>
  auth0 domains default set --domain <domain> --json
```


## Flags

```
  -d, --domain string   Domain name.
      --json            Output in json format.
      --json-compact    Output in compact json format.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 domains default set](auth0_domains_default_set.md) - Set the default custom domain
- [auth0 domains default show](auth0_domains_default_show.md) - Show the default custom domain


