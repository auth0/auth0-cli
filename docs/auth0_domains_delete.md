---
layout: default
---
# auth0 domains delete

Delete a custom domain.

To delete interactively, use `auth0 domains delete` with no arguments.

To delete non-interactively, supply the custom domain id and the `--force` flag to skip confirmation.

```
auth0 domains delete [flags]
```


## Flags

```
      --force   Skip confirmation.
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
  auth0 domains delete
  auth0 domains delete <id>
  auth0 domains delete <id> --force
```


## Related Commands

- [auth0 domains create](auth0_domains_create.md) - Create a custom domain
- [auth0 domains delete](auth0_domains_delete.md) - Delete a custom domain
- [auth0 domains list](auth0_domains_list.md) - List your custom domains
- [auth0 domains show](auth0_domains_show.md) - Show a custom domain
- [auth0 domains update](auth0_domains_update.md) - Update a custom domain
- [auth0 domains verify](auth0_domains_verify.md) - Verify a custom domain


