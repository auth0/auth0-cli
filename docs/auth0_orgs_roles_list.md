---
layout: default
---
# auth0 orgs roles list

List roles assigned to members of an organization.

```
auth0 orgs roles list [flags]
```


## Flags

```
      --json         Output in json format.
  -n, --number int   Number of apps to retrieve (default 50)
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
  auth0 orgs roles list
  auth0 orgs roles ls <id>
  auth0 orgs roles ls <id> --json
```


## Related Commands

- [auth0 orgs roles list](auth0_orgs_roles_list.md) - List roles of an organization
- [auth0 orgs roles members](auth0_orgs_roles_members.md) - Manage roles of organization members


