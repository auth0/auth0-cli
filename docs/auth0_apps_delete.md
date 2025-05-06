---
layout: default
parent: auth0 apps
has_toc: false
---
# auth0 apps delete

Delete an application.

To delete interactively, use `auth0 apps delete` with no arguments.

To delete non-interactively, supply the application id and the `--force` flag to skip confirmation.

## Usage
```
auth0 apps delete [flags]
```

## Examples

```
  auth0 apps delete 
  auth0 apps rm
  auth0 apps delete <app-id>
  auth0 apps delete <app-id> --force
  auth0 apps delete <app-id> <app-id2> <app-idn>
  auth0 apps delete <app-id> <app-id2> <app-idn> --force
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

- [auth0 apps create](auth0_apps_create.md) - Create a new application
- [auth0 apps delete](auth0_apps_delete.md) - Delete an application
- [auth0 apps list](auth0_apps_list.md) - List your applications
- [auth0 apps open](auth0_apps_open.md) - Open the settings page of an application
- [auth0 apps session-transfer](auth0_apps_session-transfer.md) - Manage session transfer settings for an application
- [auth0 apps show](auth0_apps_show.md) - Show an application
- [auth0 apps update](auth0_apps_update.md) - Update an application
- [auth0 apps use](auth0_apps_use.md) - Choose a default application for the Auth0 CLI


