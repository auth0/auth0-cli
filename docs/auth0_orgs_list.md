---
layout: default
---
## auth0 orgs list

List your organizations

### Synopsis

List your existing organizations. To create one try:
auth0 orgs create

```
auth0 orgs list [flags]
```

### Examples

```
auth0 orgs list
auth0 orgs ls
auth0 orgs ls -n 100
```

### Options

```
  -h, --help         help for list
      --json         Output in json format.
  -n, --number int   Number of apps to retrieve (default 50)
```

### Options inherited from parent commands

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```

### SEE ALSO

* [auth0 orgs](auth0_orgs.md)	 - Manage resources for organizations

