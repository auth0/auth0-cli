---
layout: default
parent: auth0 client-grants organizations
has_toc: false
---
# auth0 client-grants organizations list

List the organizations associated with a client grant.

## Usage
```
auth0 client-grants organizations list [flags]
```

## Examples

```
  auth0 client-grants organizations list
  auth0 client-grants organizations ls <client-grant-id>
  auth0 client-grants organizations list <client-grant-id> --number 100
  auth0 client-grants organizations ls <client-grant-id> -n 100 --json
  auth0 client-grants organizations list <client-grant-id> --csv
```


## Flags

```
      --csv            Output in csv format.
      --json           Output in json format.
      --json-compact   Output in compact json format.
  -n, --number int     Number of organizations to retrieve. Minimum 1, maximum 1000. (default 100)
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

- [auth0 client-grants organizations list](auth0_client-grants_organizations_list.md) - List the organizations of a client grant


