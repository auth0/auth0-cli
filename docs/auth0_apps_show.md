---
layout: default
parent: auth0 apps
has_toc: false
---
# auth0 apps show

Display the name, description, app type, and other information about an application.

## Usage
```
auth0 apps show [flags]
```

## Examples

```
  auth0 apps show
  auth0 apps show <app-id>
  auth0 apps show <app-id> --reveal-secrets
  auth0 apps show <app-id> -r --json
```


## Flags

```
      --json             Output in json format.
  -r, --reveal-secrets   Display the application secrets ('signing_keys', 'client_secret') as part of the command output.
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
- [auth0 apps show](auth0_apps_show.md) - Show an application
- [auth0 apps update](auth0_apps_update.md) - Update an application
- [auth0 apps use](auth0_apps_use.md) - Choose a default application for the Auth0 CLI


