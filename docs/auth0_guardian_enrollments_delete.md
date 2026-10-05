---
layout: default
parent: auth0 guardian enrollments
has_toc: false
---
# auth0 guardian enrollments delete

Delete an MFA enrollment, allowing the user to re-enroll.

To delete interactively, use `auth0 guardian enrollments delete` with no arguments.

To delete non-interactively, supply the enrollment id and the `--force` flag to skip confirmation.

## Usage
```
auth0 guardian enrollments delete [flags]
```

## Examples

```
  auth0 guardian enrollments delete
  auth0 guardian enrollments rm
  auth0 guardian enrollments delete <enrollment-id>
  auth0 guardian enrollments delete <enrollment-id> --force
```


## Flags

```
      --force   Skip confirmation.
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


