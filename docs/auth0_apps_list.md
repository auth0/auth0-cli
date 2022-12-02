---
layout: default
---
## auth0 apps list

List your applications

### Synopsis

List your existing applications. To create one try:
auth0 apps create

```
auth0 apps list [flags]
```

### Examples

```
auth0 apps list
auth0 apps ls
auth0 apps ls -n 100
```

### Options

```
  -h, --help         help for list
  -n, --number int   Number of apps to retrieve (default 50)
  -r, --reveal       Display the Client Secret as part of the command output.
```

### Options inherited from parent commands

```
      --debug           Enable debug mode.
      --json            Output in json format.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```

### SEE ALSO

* [auth0 apps](auth0_apps.md)	 - Manage resources for applications

