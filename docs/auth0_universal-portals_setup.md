---
layout: default
parent: auth0 universal-portals
has_toc: false
---
# auth0 universal-portals setup

Provisions the Auth0 resources required by a Universal Portals application:

  - Auth0 My Account API and My Organization API (resource servers)
  - A Regular Web App client with the required configuration
  - Client grants for My Account API, My Organization API, and the Management API

## Usage
```
auth0 universal-portals setup [flags]
```

## Examples

```
  auth0 universal-portals setup
  auth0 universal-portals setup --name "My Portal"
  auth0 up setup -n "My Portal"
```


## Flags

```
  -n, --name string   Name of the Universal Portals application.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 universal-portals setup](auth0_universal-portals_setup.md) - Set up Auth0 resources for a Universal Portals application


