---
layout: default
parent: auth0 orgs
has_toc: false
---
# auth0 orgs delete

Delete an organization.

To delete interactively, use `auth0 orgs delete` with no arguments.

To delete non-interactively, supply the organization id and the `--force` flag to skip confirmation.

## Usage
```
auth0 orgs delete [flags]
```

## Examples

```
  auth0 orgs delete
  auth0 orgs rm
  auth0 orgs delete <org-id>
  auth0 orgs delete <org-id> --force
  auth0 orgs delete <org-id> <org-id2> <org-idn>
  auth0 orgs delete <org-id> <org-id2> <org-idn> --force
```


## Flags

```
      --force   Skip confirmation.
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


