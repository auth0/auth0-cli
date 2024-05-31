---
layout: default
parent: auth0 orgs roles
has_toc: false
---
# auth0 orgs roles list

List roles assigned to members of an organization.

## Usage
```
auth0 orgs roles list [flags]
```

## Examples

```
  auth0 orgs roles list
  auth0 orgs roles ls <org-id>
  auth0 orgs roles list <org-id> --number 100
  auth0 orgs roles ls <org-id> -n 100 --json
  auth0 orgs roles ls <org-id> --csv
```


## Flags

```
      --csv          Output in csv format.
      --json         Output in json format.
  -n, --number int   Number of organization roles to retrieve. Minimum 1, maximum 1000. (default 100)
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 orgs roles list](auth0_orgs_roles_list.md) - List roles of an organization
- [auth0 orgs roles members](auth0_orgs_roles_members.md) - Manage roles of organization members


