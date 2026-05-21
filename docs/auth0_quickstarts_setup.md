---
layout: default
parent: auth0 quickstarts
has_toc: false
---
# auth0 quickstarts setup

Auto-detects your project, creates an Auth0 application and/or API, and generates a config file.

Workflows:
  --app                          Create an application (auto-detects framework).
  --api                          Create an API (prompts to create or link an app).
  --api --linked-app-id <id>     Create an API linked to an existing application.

## Usage
```
auth0 quickstarts setup [flags]
```

## Examples

```
  # Interactive setup:
  auth0 quickstarts setup

  # App only:
  auth0 quickstarts setup --app --type spa --framework react

  # App with all options:
  auth0 quickstarts setup --app --type spa --framework react --build-tool vite --name "My SPA" --port 5173

  # API + new app:
  auth0 quickstarts setup --api --app --type regular --framework express --identifier https://my-api

  # API + existing app:
  auth0 quickstarts setup --api --linked-app-id <client-id> --identifier https://my-api

  # API with custom settings:
  auth0 quickstarts setup --api --linked-app-id <client-id> --identifier https://my-api --scopes "read:data,write:data"
```


## Flags

```
      --api                     Create an Auth0 API resource server
      --app                     Create an Auth0 application (SPA, regular web, or native)
      --audience string         Unique URL identifier for the API (audience), e.g. https://my-api
      --build-tool string       Build tool used by the project (vite, webpack, cra, none) (default "none")
      --callback-url string     Override the allowed callback URL for the application
      --framework string        Framework to configure (e.g., react, nextjs, vue, express)
      --identifier string       Unique URL identifier for the API (audience), e.g. https://my-api
      --linked-app-id string    [API] Client ID of an existing application to link to the API (skips app creation)
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


