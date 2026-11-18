---
layout: default
parent: auth0 orgs invitations
has_toc: false
---
# auth0 orgs invitations create

Create a new invitation to an organization with required and optional parameters.

To create interactively, use `auth0 orgs invs create` with no flags and answer the prompts.

To create non-interactively, supply the organization id and the other parameters through flags.

Use '--schema' to print the request payload schema and exit.
Use '--data' with '--org-id' to supply the full JSON payload (validated against the schema before sending).

## Usage
```
auth0 orgs invitations create [flags]
```

## Examples

```
  auth0 orgs invs create
  auth0 orgs invs create --org-id <org-id>
  auth0 orgs invs create --org-id <org-id> --inviter-name "Inviter Name" --invitee-email "invitee@example.com"
  auth0 orgs invs create --org-id <org-id> --invitee-email "invitee@example.com" --client-id "client_id"
  auth0 orgs invs create --org-id <org-id> -n "Inviter Name" -e "invitee@example.com" --client-id "client_id" --connection-id "connection_id" -t 86400
  auth0 orgs invs create --org-id <org-id> --json --send-email=false --inviter-name "Inviter Name"

  # Discover the payload schema
  auth0 orgs invs create --schema
  auth0 orgs invs create --schema --json

  # JSON input mode (for agents and automation)
  auth0 orgs invs create --org-id <org-id> --data '{"inviter":{"name":"Inviter Name"},"invitee":{"email":"invitee@example.com"},"client_id":"<client-id>"}'
  auth0 orgs invs create --org-id <org-id> --data @invitation.json
```


## Flags

```
  -a, --app-metadata string    Data related to the user that affects the application's core functionality, formatted as JSON
      --client-id string       Auth0 client ID. Used to resolve the application's login initiation endpoint.
      --connection-id string   The id of the connection to force invitee to authenticate with.
      --data string            JSON payload for the operation, as a JSON string or file path (@file.json). Can also be piped via stdin.
  -e, --invitee-email string   Email address of the person being invited.
  -n, --inviter-name string    Name of the person sending the invitation.
      --json                   Output in json format.
      --json-compact           Output in compact json format.
      --org-id string          ID of the organization.
  -r, --roles strings          Roles IDs to associate with the user.
      --schema                 Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
  -s, --send-email             Whether to send the invitation email to the invitee. (default true)
  -t, --ttl-sec int            Number of seconds for which the invitation is valid before expiration.
  -u, --user-metadata string   Data related to the user that does not affect the application's core functionality, formatted as JSON
```


## Inherited Flags

```
      --agent-mode      Output JSON, disable prompts and colors. Auto-enabled for AI agents; set AUTH0_AGENT_MODE=false to disable.
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


