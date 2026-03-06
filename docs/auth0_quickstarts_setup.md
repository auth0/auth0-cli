---
layout: default
parent: auth0 quickstarts
has_toc: false
---
# auth0 quickstarts setup

Creates an Auth0 application and generates a .env file with the necessary configuration.

The command will:
  1. Check if you are authenticated (and prompt for login if needed)
  2. Create an Auth0 application based on the specified type
  3. Generate a .env file with the appropriate environment variables

Supported types:
  - vite: For client-side SPAs (React, Vue, Svelte, etc.)
  - nextjs: For Next.js server-side applications
  - fastify: For Fastify web applications
  - jhipster-rwa: For JHipster regular web applications

## Usage
```
auth0 quickstarts setup [flags]
```

## Examples

```
  auth0 quickstarts setup --type vite
  auth0 quickstarts setup --type nextjs
  auth0 quickstarts setup --type fastify
  auth0 quickstarts setup --type vite --name "My App"
  auth0 quickstarts setup --type nextjs --port 8080
  auth0 quickstarts setup --type jhipster-rwa
  auth0 qs setup --type fastify -n "My App" -p 3000
```


## Flags

```
      --json          Output in json format.
  -n, --name string   Name of the Auth0 application (default: 'My App' for vite, nextjs and fastify, 'JHipster' for jhipster-rwa)
  -p, --port int      Port number for the application (default: 5173 for vite, 3000 for nextjs/fastify, 8080 for jhipster-rwa)
  -t, --type string   Type of quickstart (vite, nextjs, fastify, jhipster-rwa)
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


