---
layout: default
parent: auth0 guardian enrollments
has_toc: false
---
# auth0 guardian enrollments create-ticket

Create an MFA enrollment ticket for a user and, optionally, email it to them.

The returned ticket URL is the link the user follows to enroll.

## Usage
```
auth0 guardian enrollments create-ticket [flags]
```

## Examples

```
  auth0 guardian enrollments create-ticket --user-id "auth0|123"
  auth0 guardian enrollments create-ticket --user-id "auth0|123" --factor push-notification
  auth0 guardian enrollments create-ticket --user-id "auth0|123" --send-email --email me@example.com
  auth0 guardian enrollments create-ticket --user-id "auth0|123" --allow-multiple --json
```


## Flags

```
      --allow-multiple        Allow a user who has previously enrolled in MFA to enroll with additional factors. Universal Login only.
      --email string          Alternate email address to send the enrollment email to. Defaults to the user's email.
      --email-locale string   Locale of the enrollment email. Used with --send-email.
  -f, --factor string         Factor the user must enroll with, e.g. push-notification, sms, email, otp, webauthn-roaming, webauthn-platform, recovery-code, duo.
      --json                  Output in json format.
      --json-compact          Output in compact json format.
      --send-email            Send an email to the user to start the enrollment.
  -u, --user-id string        User ID to create the enrollment ticket for.
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


