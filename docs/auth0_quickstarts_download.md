---
layout: default
parent: auth0 quickstarts
has_toc: false
---
# auth0 quickstarts download

Download a Quickstart sample application for that’s already configured for your Auth0 application. There are many different tech stacks available.

## Usage
```
auth0 quickstarts download [flags]
```

## Examples

```
  auth0 quickstarts download
  auth0 quickstarts download <app-id>
  auth0 quickstarts download <app-id> --stack <stack>
  auth0 qs download <app-id> -s <stack>
  auth0 qs download <app-id> -s "Next.js"
  auth0 qs download <app-id> -s "Next.js" --force
```


## Flags

```
      --force          Skip confirmation.
  -s, --stack string   Tech/language of the Quickstart sample to download. You can use the 'auth0 quickstarts list' command to see all available tech stacks. 
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


