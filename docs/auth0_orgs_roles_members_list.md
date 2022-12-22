---
layout: default
---
# auth0 orgs roles members list

List organization members that have a given role assigned to them.

```
auth0 orgs roles members list [flags]
```


## Flags

```
      --json             Output in json format.
  -n, --number int       Number of apps to retrieve (default 50)
  -r, --role-id string   Role Identifier.
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
  auth0 orgs roles members list
  auth0 orgs roles members list <org id> --role-id role
```


## Related Commands

- [auth0 orgs roles members list](auth0_orgs_roles_members_list.md) - List organization members for a role


