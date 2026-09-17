---
layout: default
parent: auth0 guardian enrollments
has_toc: false
---
# auth0 guardian enrollments show

Display the status, type and details of an MFA enrollment.

## Usage
```
auth0 guardian enrollments show [flags]
```

## Examples

```
  auth0 guardian enrollments show <enrollment-id>
  auth0 guardian enrollments show <enrollment-id> --json
```


## Flags

```
      --json           Output in json format.
      --json-compact   Output in compact json format.
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

- [auth0 guardian enrollments create-ticket](auth0_guardian_enrollments_create-ticket.md) - Create a multi-factor authentication enrollment ticket
- [auth0 guardian enrollments delete](auth0_guardian_enrollments_delete.md) - Delete a multi-factor authentication enrollment
- [auth0 guardian enrollments show](auth0_guardian_enrollments_show.md) - Show a multi-factor authentication enrollment


