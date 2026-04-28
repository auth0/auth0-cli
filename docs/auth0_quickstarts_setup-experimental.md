---
layout: default
parent: auth0 quickstarts
has_toc: false
---
# auth0 quickstarts setup-experimental

Creates an Auth0 application and/or API and generates a config file with the necessary Auth0 settings.

The command will:
  1. Check if you are authenticated (and prompt for login if needed)
  2. Auto-detect your project framework from the current directory
  3. Create an Auth0 application and/or API resource server
  4. Generate a config file with the appropriate environment variables

Supported frameworks are dynamically loaded from the QuickstartConfigs map.

## Usage
```
auth0 quickstarts setup-experimental [flags]
```

## Examples

```
  auth0 quickstarts setup-experimental
  auth0 quickstarts setup-experimental --app --framework react --type spa
  auth0 quickstarts setup-experimental --api --identifier https://my-api
  auth0 quickstarts setup-experimental --app --api --name "My App"
```


## Flags

```
      --api                     Create an Auth0 API resource server
      --app                     Create an Auth0 application (SPA, regular web, or native)
      --audience string         Alias for --identifier (unique audience URL for the API)
      --build-tool string       Build tool used by the project (vite, webpack, cra, none) (default "none")
      --callback-url string     Override the allowed callback URL for the application
      --framework string        Framework to configure (e.g., react, nextjs, vue, express)
      --identifier string       Unique URL identifier for the API (audience), e.g. https://my-api
      --logout-url string       Override the allowed logout URL for the application
      --name string             Name of the Auth0 application
      --offline-access          Allow offline access (enables refresh tokens)
      --port int                Local port the application runs on (default varies by framework, e.g. 3000, 5173)
      --scopes string           [API] Comma-separated list of permission scopes for the API
      --signing-alg string      [API] Token signing algorithm: RS256, PS256, or HS256 (leave blank to be prompted interactively)
      --token-lifetime string   [API] Access token lifetime in seconds (default: 86400 = 24 hours)
      --type string             Application type: spa, regular, native, or m2m
      --web-origin-url string   Override the allowed web origin URL for the application
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


