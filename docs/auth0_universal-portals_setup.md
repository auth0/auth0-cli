---
layout: default
parent: auth0 universal-portals
has_toc: false
---
# auth0 universal-portals setup

Creates a Universal Portal with its application, forms and pages, and checks that the logged-in session has the permissions it needs.

All flags are handled by the tool. Run "auth0 universal-portals setup --help" for the list.

## Usage
```
auth0 universal-portals setup [flags]
```

## Examples

```
  auth0 universal-portals setup
  auth0 universal-portals setup --template business --name "Acme Admin" --user jane@acme.com
  auth0 up setup --dry-run --template consumer --name "Acme Account" --json
  auth0 up setup --no-input --template consumer --name "Acme Account" --json
```




## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 universal-portals audit](auth0_universal-portals_audit.md) - Check a Universal Portal
- [auth0 universal-portals setup](auth0_universal-portals_setup.md) - Set up a Universal Portal


