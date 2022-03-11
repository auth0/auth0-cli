---
layout: default
---
## auth0 attack-protection breached-password-detection update

Update breached password detection settings

### Synopsis

Update breached password detection settings.

```
auth0 attack-protection breached-password-detection update [flags]
```

### Examples

```
auth0 attack-protection breached-password-detection update
```

### Options

```
  -f, --admin-notification-frequency strings   When "admin_notification" is enabled, determines how often email notifications are sent. Possible values:
                                               immediately, daily, weekly, monthly. Comma-separated.
  -e, --enabled                                Enable (or disable) breached password detection.
  -h, --help                                   help for update
  -m, --method string                          The subscription level for breached password detection methods. Use "enhanced" to enable Credential Guard.
                                               Possible values: standard, enhanced.
  -s, --shields strings                        Action to take when a breached password is detected. Possible values: block, user_notification,
                                               admin_notification. Comma-separated.
```

### Options inherited from parent commands

```
      --debug           Enable debug mode.
      --force           Skip confirmation.
      --format string   Command output format. Options: json.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```

### SEE ALSO

* [auth0 attack-protection](auth0_attack_protection.md)	 - Manage attack protection settings
* [auth0 attack-protection breached-password-detection](auth0_attack_protection_breached_password_detection.md)	 - Manage breached password detection settings
