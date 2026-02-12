---
layout: default
parent: auth0 protection bot-detection
has_toc: false
---
# auth0 protection bot-detection update

Update the bot detection settings.

## Usage
```
auth0 protection bot-detection update [flags]
```

## Examples

```
  auth0 protection bot-detection update
  auth0 ap bd update --bot-detection-level medium --json-compact
  auth0 ap bd update --bot-detection-level low --challenge-password-policy never
  auth0 ap bd update --monitoring-mode=true --allowlist "198.51.100.42,10.0.0.0/24"
  auth0 ap bd update -l high -a "198.51.100.42" -m=false --json
```


## Flags

```
  -a, --allowlist strings                        List of comma-separated trusted IP addresses that will not have bot detection enforced against them. Supports IPv4, IPv6 and CIDR notations.
  -l, --bot-detection-level string               The level of bot detection sensitivity. Possible values: low, medium, high.
      --challenge-password-policy string         Determines how often to challenge users with a CAPTCHA for password-based login. Possible values: never, when_risky, always.
      --challenge-password-reset-policy string   Determines how often to challenge users with a CAPTCHA for password reset. Possible values: never, when_risky, always.
      --challenge-passwordless-policy string     Determines how often to challenge users with a CAPTCHA for passwordless login. Possible values: never, when_risky, always.
      --json                                     Output in json format.
      --json-compact                             Output in compact json format.
  -m, --monitoring-mode                          Enable (or disable) monitoring mode. When enabled, logs but does not block.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 protection bot-detection show](auth0_protection_bot-detection_show.md) - Show bot detection settings
- [auth0 protection bot-detection update](auth0_protection_bot-detection_update.md) - Update bot detection settings


