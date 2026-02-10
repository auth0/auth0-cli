---
layout: default
parent: auth0 orgs invitations
has_toc: false
---
# auth0 orgs invitations show

Display information about an organization invitation.

To show interactively, use `auth0 orgs invs show` with no flags.

To show non-interactively, supply the organization id and invitation id through the flags.

## Usage
```
auth0 orgs invitations show [flags]
```

## Examples

```
  auth0 orgs invs show
  auth0 orgs invs show --org-id <org-id>
  auth0 orgs invs show --org-id <org-id> --invitation-id <invitation-id>
  auth0 orgs invs show --org-id <org-id> --invitation-id <invitation-id> --json
  auth0 orgs invs show --org-id <org-id> --i <invitation-id> --json-compact
```


## Flags

```
  -i, --invitation-id string   ID of the invitation.
      --json                   Output in json format.
      --json-compact           Output in compact json format.
      --org-id string          ID of the organization.
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


