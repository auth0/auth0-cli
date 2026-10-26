---
layout: default
parent: auth0 universal-portals
has_toc: false
---
# auth0 universal-portals audit

Checks a Universal Portal and its application against the setup defaults, and fixes the errors the tool is able to fix.

All flags are handled by the tool. Run "auth0 universal-portals audit --help" for the list.

## Usage
```
auth0 universal-portals audit [flags]
```

## Examples

```
  auth0 universal-portals audit up_123
  auth0 universal-portals audit up_123 --fix
  auth0 up audit up_123 --organization org_123 --strict --json
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


