---
layout: default
parent: auth0 apps
has_toc: false
---
# auth0 apps use

Specify the default application used when running other commands. Specifically when downloading quickstarts and testing Universal login flow.

## Usage
```
auth0 apps use [flags]
```

## Examples

```
  auth0 apps use
  auth0 apps use --none
  auth0 apps use <app-id>
```


## Flags

```
  -n, --none   Specify none of your apps.
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


