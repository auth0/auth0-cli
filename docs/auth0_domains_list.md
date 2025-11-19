---
layout: default
parent: auth0 domains
has_toc: false
---
# auth0 domains list

List your existing custom domains. To create one, run: `auth0 domains create`.

## Usage
```
auth0 domains list [flags]
```

## Examples

```
  auth0 domains list
  auth0 domains ls
  auth0 domains ls --json
  auth0 domains ls --json-compact
  auth0 domains ls --csv
  auth0 domains ls --filter "domain:demo* AND status:pending_verification"
```


## Flags

```
      --csv             Output in csv format.
      --filter string   Filter custom domains (EA-only).
      --json            Output in json format.
      --json-compact    Output in compact json format.
      --sort string     Sort by a field (EA-only). Only 'domain' is supported.
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


