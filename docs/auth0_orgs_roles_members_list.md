---
layout: default
---
## auth0 orgs roles members list

List organization members for a role

### Synopsis

List organization members that have a given role assigned to them.

```
auth0 orgs roles members list [flags]
```

### Examples

```
auth0 orgs roles members list
auth0 orgs roles members list <org id> --role-id role
```

### Options

```
  -h, --help             help for list
      --json             Output in json format.
  -n, --number int       Number of apps to retrieve (default 50)
  -r, --role-id string   Role Identifier.
```

### Options inherited from parent commands

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```

### SEE ALSO

* [auth0 orgs roles members](auth0_orgs_roles_members.md)	 - Manage roles of organization members

