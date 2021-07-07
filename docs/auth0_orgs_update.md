---
layout: default
---
## auth0 orgs update

Update an organization

### Synopsis

Update an organization.

```
auth0 orgs update [flags]
```

### Examples

```
auth0 orgs update <id> 
auth0 orgs update <id> --display "My Organization"
auth0 orgs update <id> -d "My Organization" -l "https://example.com/logo.png" -a "#635DFF" -b "#2A2E35"
auth0 orgs update <id> -d "My Organization" -m "KEY=value" -m "OTHER_KEY=other_value"
```

### Options

```
  -a, --accent string             Accent color used to customize the login pages.
  -b, --background string         Background color used to customize the login pages.
  -d, --display string            Friendly name of the organization.
  -h, --help                      help for update
  -l, --logo string               URL of the logo to be displayed on the login page.
  -m, --metadata stringToString   Metadata associated with the organization (max 255 chars). Maximum of 10 metadata properties allowed. (default [])
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

