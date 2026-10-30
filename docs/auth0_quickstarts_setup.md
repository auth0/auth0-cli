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

  # API + new app, granting a subset of the API scopes, with a JSON result:
  auth0 quickstarts setup --api --app --type regular --framework express --identifier https://my-api --scopes "read:data,write:data" --grant-scopes read:data --json
```


## Flags

```
      --api                      Create an Auth0 API resource server
      --app                      Create an Auth0 application (SPA, regular web, or native)
      --audience string          Unique URL identifier for the API (audience), e.g. https://my-api
      --build-tool string        Build tool used by the project (composer, gradle, maven, none, vite, webpack) (default "none")
      --callback-url strings     Override the allowed callback URLs for the application. Comma-separate or repeat the flag for multiple values; replaces the default list.
      --force                    Replace an existing config file without prompting (a backup is written first). In agent mode or with --no-input, an existing .env is merged instead, and any other existing config file is only replaced with --force.
      --framework string         Framework to configure (e.g., react, nextjs, vue, express)
      --grant-scopes strings     [API] Comma-separated subset of --scopes to grant the app created with --app or given with --linked-app-id (default: all --scopes). Every value must also be listed in --scopes. An empty value grants no scopes.
      --identifier string        Unique URL identifier for the API (audience), e.g. https://my-api
      --json                     Output in json format.
      --json-compact             Output in compact json format.
      --linked-app-id string     [API] Client ID of an existing application to link to the API (skips app creation)
      --logout-url strings       Override the allowed logout URLs for the application. Comma-separate or repeat the flag for multiple values; replaces the default list.
      --name string              Name of the Auth0 application
      --offline-access           Allow offline access (enables refresh tokens)
      --port int                 Local port the application runs on (default varies by framework, e.g. 3000, 5173)
      --scopes string            [API] Comma-separated permission scopes the API defines (e.g. read:data,write:data). The app created with --app or given with --linked-app-id is granted all of them unless --grant-scopes is set.
      --signing-alg string       [API] Token signing algorithm: RS256, PS256, or HS256 (leave blank to be prompted interactively)
      --token-lifetime string    [API] Access token lifetime in seconds (default: 86400 = 24 hours)
      --type string              Application type: spa, regular, native, or m2m
      --web-origin-url strings   Override the allowed web origin URLs for the application. Comma-separate or repeat the flag for multiple values; replaces the default list.
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

- [auth0 quickstarts download](auth0_quickstarts_download.md) - Download a Quickstart sample app for a specific tech stack
- [auth0 quickstarts list](auth0_quickstarts_list.md) - List the available Quickstarts
- [auth0 quickstarts setup](auth0_quickstarts_setup.md) - Set up Auth0 for your quickstart application


