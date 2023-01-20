---
layout: default
---
# auth0 apis delete

Delete an API.

To delete interactively, use `auth0 apis delete` with no arguments.

To delete non-interactively, supply the API id and the `--force` flag to skip confirmation.

## Usage
```
auth0 apis delete [flags]
```

## Examples

```
  auth0 apis delete 
  auth0 apis rm
  auth0 apis delete <api-id|api-audience>
  auth0 apis delete <api-id|api-audience> --force
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


## Related Commands

- [auth0 apis create](auth0_apis_create.md) - Create a new API
- [auth0 apis delete](auth0_apis_delete.md) - Delete an API
- [auth0 apis list](auth0_apis_list.md) - List your APIs
- [auth0 apis open](auth0_apis_open.md) - Open the settings page of an API
- [auth0 apis scopes](auth0_apis_scopes.md) - Manage resources for API scopes
- [auth0 apis show](auth0_apis_show.md) - Show an API
- [auth0 apis update](auth0_apis_update.md) - Update an API


