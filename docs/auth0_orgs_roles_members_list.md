---
layout: default
parent: auth0 orgs roles members
has_toc: false
---
# auth0 orgs roles members list

List organization members that have a given role assigned to them.

## Usage
```
auth0 orgs roles members list [flags]
```

## Examples

```
  auth0 orgs roles members list
  auth0 orgs roles members ls
  auth0 orgs roles members list <org-id> --role-id role
  auth0 orgs roles members list <org-id> --role-id role --number 100
  auth0 orgs roles members ls <org-id> -r role -n 100
  auth0 orgs roles members ls <org-id> -r role -n 100 --json
```


## Flags

```
      --json             Output in json format.
  -n, --number int       Number of members to retrieve. Minimum 1, maximum 1000. (default 50)
  -r, --role-id string   Role Identifier.
```


## InheritedFlags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 orgs roles members list](auth0_orgs_roles_members_list.md) - List organization members for a role


