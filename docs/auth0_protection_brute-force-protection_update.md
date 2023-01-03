---
layout: default
---
# auth0 protection brute-force-protection update

Update the brute force protection settings.

```
auth0 protection brute-force-protection update [flags]
```


## Flags

```
  -l, --allowlist strings   List of trusted IP addresses that will not have attack protection enforced against them. Comma-separated.
  -e, --enabled             Enable (or disable) brute force protection.
      --json                Output in json format.
  -a, --max-attempts int    Maximum number of unsuccessful attempts. (default 1)
  -m, --mode string         Account Lockout: Determines whether or not IP address is used when counting failed attempts. Possible values: count_per_identifier_and_ip, count_per_identifier.
  -s, --shields strings     Action to take when a brute force protection threshold is violated. Possible values: block, user_notification. Comma-separated.
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
  auth0 protection brute-force-protection update
  auth0 ap bfp update --json
```


## Related Commands

- [auth0 protection brute-force-protection show](auth0_protection_brute-force-protection_show.md) - Show brute force protection settings
- [auth0 protection brute-force-protection update](auth0_protection_brute-force-protection_update.md) - Update brute force protection settings


