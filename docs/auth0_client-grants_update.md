---
layout: default
parent: auth0 client-grants
has_toc: false
---
# auth0 client-grants update

Update a client grant.

To update interactively, use `auth0 client-grants update` with no arguments.

The client id and audience of a grant cannot be changed. To update non-interactively, supply the scopes or organization settings through the flags. Pass `--allow-all-scopes` to grant every scope on the API instead of a specific list.

## Usage
```
auth0 client-grants update [flags]
```

## Examples

```
  auth0 client-grants update
  auth0 client-grants update <client-grant-id>
  auth0 client-grants update <client-grant-id> --scopes "read:users,update:users"
  auth0 client-grants update <client-grant-id> --allow-all-scopes
  auth0 client-grants update <client-grant-id> -s "read:users" -o require --allow-any-organization=false
  auth0 client-grants update <client-grant-id> --json
```


## Flags

```
      --allow-all-scopes            Grant every scope configured on the API. Mutually exclusive with --scopes.
      --allow-any-organization      Whether any organization can be used with this grant (true) or only explicitly assigned organizations (false).
      --json                        Output in json format.
      --json-compact                Output in compact json format.
  -o, --organization-usage string   Whether organizations can be used with this grant. Possible values: deny, allow, require.
  -s, --scopes strings              Comma-separated list of scopes (permissions) to grant.
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
- [auth0 client-grants show](auth0_client-grants_show.md) - Show a client grant
- [auth0 client-grants update](auth0_client-grants_update.md) - Update a client grant


