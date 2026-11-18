---
layout: default
parent: auth0 protection breached-password-detection
has_toc: false
---
# auth0 protection breached-password-detection update

Update the breached password detection settings.

Use '--schema' to print the request payload schema and exit.
Use '--data' to supply the full JSON payload (validated against the schema before sending).

## Usage
```
auth0 protection breached-password-detection update [flags]
```

## Examples

```
  auth0 protection breached-password-detection update
  auth0 ap bpd update --enabled=true
  auth0 ap bpd update --enabled=true --admin-notification-frequency weekly
  auth0 ap bpd update --enabled=false --admin-notification-frequency weekly --method enhanced
  auth0 ap bpd update --enabled=true --admin-notification-frequency weekly --method enhanced --shields admin_notification
  auth0 ap bpd update -e=true -f weekly -m enhanced -s admin_notification --json

  # Discover the payload schema
  auth0 ap bpd update --schema
  auth0 ap bpd update --schema --json

  # JSON input mode (for agents and automation)
  auth0 ap bpd update --data '{"enabled":true,"shields":["admin_notification"],"admin_notification_frequency":["weekly"]}'
  auth0 ap bpd update --data @breached-password-detection.json
  cat breached-password-detection.json | auth0 ap bpd update
```


## Flags

```
  -f, --admin-notification-frequency strings   When "admin_notification" is enabled, determines how often email notifications are sent. Possible values: immediately, daily, weekly, monthly. Comma-separated.
      --data string                            JSON payload for the operation, as a JSON string or file path (@file.json). Can also be piped via stdin.
  -e, --enabled                                Enable (or disable) breached password detection.
      --json                                   Output in json format.
      --json-compact                           Output in compact json format.
  -m, --method string                          The subscription level for breached password detection methods. Use "enhanced" to enable Credential Guard. Possible values: standard, enhanced.
      --schema                                 Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
  -s, --shields strings                        Action to take when a breached password is detected. Possible values: block, user_notification, admin_notification. Comma-separated.
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

- [auth0 protection breached-password-detection show](auth0_protection_breached-password-detection_show.md) - Show breached password detection settings
- [auth0 protection breached-password-detection update](auth0_protection_breached-password-detection_update.md) - Update breached password detection settings


