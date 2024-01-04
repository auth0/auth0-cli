---
layout: default
parent: auth0 apis scopes
has_toc: false
---
# auth0 apis scopes list

List the scopes of an API. To update scopes, run: `auth0 apis update <id|audience> -s <scopes>`.

## Usage
```
auth0 apis scopes list [flags]
```

## Examples

```
  auth0 apis scopes list
  auth0 apis scopes ls <api-id|api-audience>
  auth0 apis scopes ls <api-id|api-audience> --json
  auth0 apis scopes ls <api-id|api-audience> --csv
```


## Flags

```
      --csv    Output in csv format.
      --json   Output in json format.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 apis scopes list](auth0_apis_scopes_list.md) - List the scopes of an API


