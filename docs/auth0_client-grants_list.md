---
layout: default
parent: auth0 client-grants
has_toc: false
---
# auth0 client-grants list

List your existing client grants. To create one, run: `auth0 client-grants create`.

Use the filter flags to narrow the results server-side by client, audience, subject type, default group or organization usage.

## Usage
```
auth0 client-grants list [flags]
```

## Examples

```
  auth0 client-grants list
  auth0 client-grants ls
  auth0 client-grants ls --number 100
  auth0 client-grants ls --audience <api-identifier>
  auth0 client-grants ls --client-id <client-id> --subject-type client
  auth0 client-grants ls --default-for third_party_clients
  auth0 client-grants ls --allow-any-organization=true
  auth0 client-grants ls -n 100 --json
  auth0 client-grants ls --csv
```


## Flags

```
      --allow-any-organization   Filter by whether any organization can be used with the grant (true) or only explicitly assigned organizations (false).
  -a, --audience string          Filter by audience (API identifier).
  -c, --client-id string         Filter by client ID. Mutually exclusive with --default-for.
      --csv                      Output in csv format.
      --default-for string       Filter by the group this grant is the default for. Possible value: third_party_clients. Mutually exclusive with --client-id.
      --json                     Output in json format.
      --json-compact             Output in compact json format.
  -n, --number int               Number of client grants to retrieve. Minimum 1, maximum 1000. (default 100)
      --subject-type string      Filter by subject type. Possible values: client, user, anonymous_user.
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

- [auth0 client-grants create](auth0_client-grants_create.md) - Create a new client grant
- [auth0 client-grants delete](auth0_client-grants_delete.md) - Delete a client grant
- [auth0 client-grants list](auth0_client-grants_list.md) - List your client grants
- [auth0 client-grants organizations](auth0_client-grants_organizations.md) - Manage organizations of a client grant
- [auth0 client-grants show](auth0_client-grants_show.md) - Show a client grant
- [auth0 client-grants update](auth0_client-grants_update.md) - Update a client grant


