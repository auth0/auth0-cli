---
layout: default
parent: auth0 client-grants
has_toc: false
---
# auth0 client-grants delete

Delete a client grant.

To delete interactively, use `auth0 client-grants delete` with no arguments.

To delete non-interactively, supply the client grant id and the `--force` flag to skip confirmation.

## Usage
```
auth0 client-grants delete [flags]
```

## Examples

```
  auth0 client-grants delete
  auth0 client-grants rm
  auth0 client-grants delete <client-grant-id>
  auth0 client-grants delete <client-grant-id> --force
  auth0 client-grants delete <client-grant-id> <client-grant-id2> <client-grant-idn>
  auth0 client-grants delete <client-grant-id> <client-grant-id2> <client-grant-idn> --force
```


## Flags

```
      --force   Skip confirmation.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 client-grants create](auth0_client-grants_create.md) - Create a new client grant
- [auth0 client-grants delete](auth0_client-grants_delete.md) - Delete a client grant
- [auth0 client-grants list](auth0_client-grants_list.md) - List your client grants
- [auth0 client-grants organizations](auth0_client-grants_organizations.md) - Manage organizations of a client grant
- [auth0 client-grants show](auth0_client-grants_show.md) - Show a client grant
- [auth0 client-grants update](auth0_client-grants_update.md) - Update a client grant


