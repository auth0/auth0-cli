---
layout: default
parent: auth0 orgs
has_toc: false
---
# auth0 orgs update

Update an organization.

To update interactively, use `auth0 orgs update` with no arguments.

To update non-interactively, supply the organization id and other information through the flags.

## Usage
```
auth0 orgs update [flags]
```

## Examples

```
  auth0 orgs update <org-id>
  auth0 orgs update <org-id> --display "My Organization"
  auth0 orgs update <org-id> -d "My Organization" -l "https://example.com/logo.png" -a "#635DFF" -b "#2A2E35"
  auth0 orgs update <org-id> -d "My Organization" -m "KEY=value" -m "OTHER_KEY=other_value"
```


## Flags

```
  -a, --accent string             Accent color used to customize the login pages.
  -b, --background string         Background color used to customize the login pages.
  -d, --display string            Friendly name of the organization.
      --json                      Output in json format.
      --json-compact              Output in compact json format.
  -l, --logo string               URL of the logo to be displayed on the login page.
  -m, --metadata stringToString   Metadata associated with the organization (max 255 chars). Maximum of 10 metadata properties allowed. (default [])
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 orgs create](auth0_orgs_create.md) - Create a new organization
- [auth0 orgs delete](auth0_orgs_delete.md) - Delete an organization
- [auth0 orgs invitations](auth0_orgs_invitations.md) - Manage invitations of an organization
- [auth0 orgs list](auth0_orgs_list.md) - List your organizations
- [auth0 orgs members](auth0_orgs_members.md) - Manage members of an organization
- [auth0 orgs open](auth0_orgs_open.md) - Open the settings page of an organization
- [auth0 orgs roles](auth0_orgs_roles.md) - Manage roles of an organization
- [auth0 orgs show](auth0_orgs_show.md) - Show an organization
- [auth0 orgs update](auth0_orgs_update.md) - Update an organization


