---
layout: default
parent: auth0 universal-portals
has_toc: false
---
# auth0 universal-portals setup

Creates a Universal Portal with its application, forms and pages, and checks that the logged-in session has the permissions it needs before it creates anything.

Running the command again after a failure resumes: what exists is reused.

## Usage
```
auth0 universal-portals setup [flags]
```

## Examples

```
  auth0 universal-portals setup
  auth0 universal-portals setup --template business --name "Acme Admin" --user jane@acme.com
  auth0 universal-portals setup --template consumer --name "Acme Account" --dry-run
  auth0 up setup --no-input --template consumer --name "Acme Account" --json
```


## Flags

```
      --dry-run               Show what setup would create, without changing the tenant. The permission checks still send write requests with an empty body, which the API rejects.
      --json                  Output in json format.
  -n, --name string           Name of the portal. Asked when omitted.
  -o, --organization string   Business only: the ID or name of the organization, when the user has several.
  -s, --slug string           URL slug of the portal, for example acme-account. Defaults to a slug made from the name.
      --template string       Kind of portal: consumer, for end-users to manage their account, or business, for organization administrators to manage their organization. Asked when omitted.
  -u, --user string           Business only: the email or user ID of the user who will test the portal.
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

- [auth0 universal-portals audit](auth0_universal-portals_audit.md) - Check a Universal Portal
- [auth0 universal-portals setup](auth0_universal-portals_setup.md) - Set up a Universal Portal


