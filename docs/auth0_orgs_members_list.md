---
layout: default
parent: auth0 orgs members
has_toc: false
---
# auth0 orgs members list

List the members of an organization.

## Usage
```
auth0 orgs members list [flags]
```

## Examples

```
  auth0 orgs members list
  auth0 orgs members ls <org-id>
  auth0 orgs members list <org-id> --number 100
  auth0 orgs members ls <org-id> -n 100 --json
  auth0 orgs members ls <org-id> -n 100 --json-compact
  auth0 orgs members ls <org-id> --csv
```


## Flags

```
      --csv            Output in csv format.
      --json           Output in json format.
      --json-compact   Output in compact json format.
  -n, --number int     Number of organization members to retrieve. Minimum 1, maximum 1000. (default 100)
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 orgs members list](auth0_orgs_members_list.md) - List members of an organization


