---
layout: default
parent: auth0 client-grants
has_toc: false
---
# auth0 client-grants create

Create a new client grant.

To create interactively, use `auth0 client-grants create` with no flags.

To create non-interactively, supply the audience and either a client id (`--client-id`) or a default group (`--default-for`), which are mutually exclusive, along with any optional scopes or organization settings through the flags. A grant can authorize specific scopes (`--scopes`), every scope on the API (`--allow-all-scopes`), or no scopes at all.

Note: for the Auth0 Management API with `--subject-type user`, scopes must be a subset of the fixed current_user set and cannot be listed dynamically, so pass them inline, for example: `--scopes "read:current_user,update:current_user_metadata,delete:current_user_metadata,create:current_user_metadata,create:current_user_device_credentials,delete:current_user_device_credentials,update:current_user_identities"`.

## Usage
```
auth0 client-grants create [flags]
```

## Examples

```
  auth0 client-grants create
  auth0 client-grants create --client-id <client-id> --audience <api-identifier>
  auth0 client-grants create --default-for third_party_clients --audience <api-identifier>
  auth0 client-grants create --client-id <client-id> --audience <api-identifier> --scopes "read:users,update:users"
  auth0 client-grants create --client-id <client-id> --audience <api-identifier> --allow-all-scopes
  auth0 client-grants create --client-id <client-id> --audience <api-identifier> --authorization-details-types "payment,transfer"
  auth0 client-grants create -c <client-id> -a <api-identifier> -s "read:users" -o require --allow-any-organization=false
  auth0 client-grants create -c <client-id> -a <api-identifier> --subject-type user
  auth0 client-grants create -c <client-id> -a <api-identifier> --json
```


## Flags

```
      --allow-all-scopes                      Grant every scope configured on the API. Mutually exclusive with --scopes.
      --allow-any-organization                Whether any organization can be used with this grant (true) or only explicitly assigned organizations (false).
  -a, --audience string                       Audience (API identifier) of the client grant. Cannot be changed once set.
      --authorization-details-types strings   Comma-separated list of authorization_details types allowed for this grant (Rich Authorization Requests).
  -c, --client-id string                      Client ID of the application to authorize. Cannot be changed once set. Mutually exclusive with --default-for.
      --default-for string                    Make this the default grant for a group of clients instead of authorizing a specific client. Mutually exclusive with --client-id. Possible value: third_party_clients.
      --json                                  Output in json format.
      --json-compact                          Output in compact json format.
  -o, --organization-usage string             Whether organizations can be used with this grant. Possible values: deny, allow, require.
  -s, --scopes strings                        Comma-separated list of scopes (permissions) to grant.
      --subject-type string                   Subject type of the grant. Cannot be changed once set. Possible values: client, user, anonymous_user.
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


