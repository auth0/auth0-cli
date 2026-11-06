---
layout: default
parent: auth0 universal-portals
has_toc: false
---
# auth0 universal-portals audit

Checks a Universal Portal and its application against what setup makes, and tells what is wrong and how to fix it.

An error breaks the portal or one of its components, and a warning is something degraded or different from the setup guide. The command fails when there is an error, and with --strict when there is a warning too.

With a terminal it offers to fix the errors the CLI can fix, and with --fix it fixes them without asking. Warnings are never changed, because they may be intentional.

## Usage
```
auth0 universal-portals audit <portal-id> [flags]
```

## Examples

```
  auth0 universal-portals audit up_123
  auth0 universal-portals audit up_123 --fix
  auth0 up audit up_123 --organization org_123 --strict --json
```


## Flags

```
      --fix                   Apply the errors the CLI can fix, without asking. Warnings are never changed.
      --json                  Output in json format.
  -o, --organization string   Business only: the ID of the organization the portal opens in, to check that it shares a connection with the application.
      --strict                Also fail when there are warnings.
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


