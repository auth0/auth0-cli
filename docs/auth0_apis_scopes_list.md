---
layout: default
---
# auth0 apis scopes list

List the scopes of an API. To update scopes, run: `auth0 apis update <id|audience> -s <scopes>`.

```
auth0 apis scopes list [flags]
```


## Flags

```
      --json   Output in json format.
```


## InheritedFlags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```

## Examples

```
  auth0 apis scopes list
  auth0 apis scopes ls <id|audience>
  auth0 apis scopes ls <id|audience> --json
```


## Related Commands

- [auth0 apis scopes list](auth0_apis_scopes_list.md) - List the scopes of an API


