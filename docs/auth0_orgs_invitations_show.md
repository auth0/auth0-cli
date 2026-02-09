---
layout: default
parent: auth0 orgs invitations
has_toc: false
---
# auth0 orgs invitations show

Display information about an organization invitation.

To show interactively, use `auth0 orgs invs show` with no arguments.

To show non-interactively, supply the organization id and invitation id through the arguments.

## Usage
```
auth0 orgs invitations show [flags]
```

## Examples

```
  auth0 orgs invs show
  auth0 orgs invs show <org-id>
  auth0 orgs invs show <org-id> <invitation-id>
  auth0 orgs invs show <org-id> <invitation-id> --json
```


## Flags

```
      --json           Output in json format.
      --json-compact   Output in compact json format.
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


