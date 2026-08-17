---
layout: default
parent: auth0 client-grants
has_toc: false
---
# auth0 client-grants update

Update a client grant.

To update interactively, use `auth0 client-grants update` with no arguments.

The client id and audience of a grant cannot be changed. To update non-interactively, supply the scopes or organization settings through the flags. Pass `--allow-all-scopes` to grant every scope on the API instead of a specific list, or `--no-scopes` to clear all scopes and authorize a token with no permissions.

Note: for the Auth0 Management API with `--subject-type user`, scopes must be a subset of the fixed current_user set and cannot be listed dynamically, so pass them inline, for example: `--scopes "read:current_user,update:current_user_metadata,delete:current_user_metadata,create:current_user_metadata,create:current_user_device_credentials,delete:current_user_device_credentials,update:current_user_identities"`.

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
  auth0 client-grants update <client-grant-id> --no-scopes
  auth0 client-grants update <client-grant-id> --authorization-details-types "payment,transfer"
  auth0 client-grants update <client-grant-id> -s "read:users" -o require --allow-any-organization=false
  auth0 client-grants update <client-grant-id> --json
```


## Flags

```
      --allow-all-scopes                      Grant every scope configured on the API. Mutually exclusive with --scopes.
      --allow-any-organization                Whether any organization can be used with this grant (true) or only explicitly assigned organizations (false).
      --authorization-details-types strings   Comma-separated list of authorization_details types allowed for this grant (Rich Authorization Requests).
      --json                                  Output in json format.
      --json-compact                          Output in compact json format.
      --no-scopes                             Clear all scopes on the grant, authorizing a token with no permissions. Mutually exclusive with --scopes and --allow-all-scopes.
  -o, --organization-usage string             Whether organizations can be used with this grant. Possible values: deny, allow, require.
  -s, --scopes strings                        Comma-separated list of scopes (permissions) to grant.
```


## Inherited Flags

```
      --agent-mode      Output JSON, disable prompts and colors. Auto-enabled for AI agents; set AUTH0_AGENT_MODE=false to disable.
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


