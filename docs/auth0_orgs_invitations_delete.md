---
layout: default
parent: auth0 orgs invitations
has_toc: false
---
# auth0 orgs invitations delete

Delete invitation(s) from an organization.

To delete interactively, use `auth0 orgs invs delete` with no flags.

To delete non-interactively, supply the organization id, invitation id(s) and the `--force` flag to skip confirmation.

## Usage
```
auth0 orgs invitations delete [flags]
```

## Examples

```
  auth0 orgs invs delete
  auth0 orgs invs rm
  auth0 orgs invs delete --org-id <org-id> --invitation-id <invitation-id>
  auth0 orgs invs delete --org-id <org-id> --invitation-id <inv-id1>,<inv-id2>,<inv-id3>
  auth0 orgs invs delete --org-id <org-id> --invitation-id <invitation-id> --force
```


## Flags

```
      --force                   Skip confirmation.
  -i, --invitation-id strings   ID of the invitation.
      --org-id string           ID of the organization.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 orgs invitations create](auth0_orgs_invitations_create.md) - Create a new invitation to an organization
- [auth0 orgs invitations delete](auth0_orgs_invitations_delete.md) - Delete invitation(s) from an organization
- [auth0 orgs invitations list](auth0_orgs_invitations_list.md) - List invitations of an organization
- [auth0 orgs invitations show](auth0_orgs_invitations_show.md) - Show an organization invitation


