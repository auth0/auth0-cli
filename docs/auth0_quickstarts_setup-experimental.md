---
layout: default
parent: auth0 quickstarts
has_toc: false
---
# auth0 quickstarts setup-experimental

Creates an Auth0 application and generates a .env file with the necessary configuration.

The command will:
  1. Check if you are authenticated (and prompt for login if needed)
  2. Create an Auth0 application based on the specified type
  3. Generate a .env file with the appropriate environment variables

Supported types are dynamically loaded from the `QuickstartConfigs` map in the codebase.

## Usage
```
auth0 quickstarts setup-experimental [flags]
```

## Examples

```
  auth0 quickstarts setup-experimental --type spa:react:vite
  auth0 quickstarts setup-experimental --type regular:nextjs:none
  auth0 quickstarts setup-experimental --type native:react-native:none
```


## Flags

```
      --name string   Name of the Auth0 application
      --port int      Port number for the application
      --type string   Type of the quickstart application (e.g., spa:react:vite, regular:nextjs:none)
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 quickstarts download](auth0_quickstarts_download.md) - Download a Quickstart sample app for a specific tech stack
- [auth0 quickstarts list](auth0_quickstarts_list.md) - List the available Quickstarts
- [auth0 quickstarts setup](auth0_quickstarts_setup.md) - Set up Auth0 for your quickstart application
- [auth0 quickstarts setup-experimental](auth0_quickstarts_setup-experimental.md) - Set up Auth0 for your quickstart application


