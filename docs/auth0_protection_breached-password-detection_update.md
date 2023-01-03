---
layout: default
---
# auth0 protection breached-password-detection update

Update the breached password detection settings.

```
auth0 protection breached-password-detection update [flags]
```


## Flags

```
  -f, --admin-notification-frequency strings   When "admin_notification" is enabled, determines how often email notifications are sent. Possible values: immediately, daily, weekly, monthly. Comma-separated.
  -e, --enabled                                Enable (or disable) breached password detection.
      --json                                   Output in json format.
  -m, --method string                          The subscription level for breached password detection methods. Use "enhanced" to enable Credential Guard. Possible values: standard, enhanced.
  -s, --shields strings                        Action to take when a breached password is detected. Possible values: block, user_notification, admin_notification. Comma-separated.
```


## InheritedFlags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```

## Examples

```
  auth0 protection breached-password-detection update
  auth0 ap bpd update --json
```


## Related Commands

- [auth0 protection breached-password-detection show](auth0_protection_breached-password-detection_show.md) - Show breached password detection settings
- [auth0 protection breached-password-detection update](auth0_protection_breached-password-detection_update.md) - Update breached password detection settings


