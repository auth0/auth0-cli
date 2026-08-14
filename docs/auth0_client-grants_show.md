---
layout: default
parent: auth0 client-grants
has_toc: false
---
# auth0 client-grants show

Display the client, audience, scopes, and other information about a client grant.

## Usage
```
auth0 client-grants show [flags]
```

## Examples

```
  auth0 client-grants show
  auth0 client-grants show <client-grant-id>
  auth0 client-grants show <client-grant-id> --json
  auth0 client-grants show <client-grant-id> --json-compact
```


## Flags

```
      --json           Output in json format.
      --json-compact   Output in compact json format.
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


