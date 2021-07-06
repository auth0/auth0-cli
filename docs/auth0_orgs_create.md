---
layout: default
---
## auth0 orgs create

Create a new organization

### Synopsis

Create a new organization.

```
auth0 orgs create [flags]
```

### Examples

```
auth0 orgs create 
auth0 orgs create --name myorganization
auth0 orgs create --n myorganization --display "My Organization"
auth0 orgs create --n myorganization -d "My Organization" -l "https://example.com/logo.png" -a "#635DFF" -b "#2A2E35"
auth0 orgs create --n myorganization -d "My Organization" -m "KEY=value" -m "OTHER_KEY=other_value"
```

### Options

```
  -a, --accent string             Accent color used to customize the login pages.
  -b, --background string         Background color used to customize the login pages.
  -d, --display string            Friendly name of the organization.
  -h, --help                      help for create
  -l, --logo string               URL of the logo to be displayed on the login page.
  -m, --metadata stringToString   Metadata associated with the organization (max 255 chars). Maximum of 10 metadata properties allowed. (default [])
  -n, --name string               Name of the organization.
```

### Options inherited from parent commands

```
      --debug           Enable debug mode.
      --force           Skip confirmation.
      --format string   Command output format. Options: json.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```

### SEE ALSO

* [auth0 orgs](auth0_orgs.md)	 - Manage resources for organizations

