---
layout: default
parent: auth0 protection brute-force-protection
has_toc: false
---
# auth0 protection brute-force-protection update

Update the brute force protection settings.

## Usage
```
auth0 protection brute-force-protection update [flags]
```

## Examples

```
  auth0 protection brute-force-protection update
  auth0 ap bfp update --enabled=true
  auth0 ap bfp update --enabled=true --allowlist "156.156.156.156,175.175.175.175"
  auth0 ap bfp update --enabled=false --allowlist "156.156.156.156,175.175.175.175" --max-attempts 3
  auth0 ap bfp update --enabled=true --allowlist "156.156.156.156,175.175.175.175" --max-attempts 3 --mode count_per_identifier_and_ip
  auth0 ap bfp update --enabled=false --allowlist "156.156.156.156,175.175.175.175" --max-attempts 3 --mode count_per_identifier_and_ip --shields user_notification
  auth0 ap bfp update -e=true -l "156.156.156.156,175.175.175.175" -a 3 -m count_per_identifier_and_ip -s user_notification --json

  # Discover the payload schema
  auth0 ap bfp update --schema
  auth0 ap bfp update --schema --json

  # JSON input mode (for agents and automation)
  auth0 ap bfp update --data '{"enabled":true,"max_attempts":5,"shields":["block","user_notification"]}'
  auth0 ap bfp update --data @brute-force-protection.json
  cat brute-force-protection.json | auth0 ap bfp update
```


## Flags

```
  -l, --allowlist strings   List of trusted IP addresses that will not have attack protection enforced against them. Comma-separated.
      --data string         JSON payload for the operation, as a JSON string or file path (@file.json). Can also be piped via stdin.
  -e, --enabled             Enable (or disable) brute force protection.
      --json                Output in json format.
      --json-compact        Output in compact json format.
  -a, --max-attempts int    Maximum number of unsuccessful attempts. (default 1)
  -m, --mode string         Account Lockout: Determines whether or not IP address is used when counting failed attempts. Possible values: count_per_identifier_and_ip, count_per_identifier.
      --schema              Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
  -s, --shields strings     Action to take when a brute force protection threshold is violated. Possible values: block, user_notification. Comma-separated.
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

- [auth0 protection brute-force-protection show](auth0_protection_brute-force-protection_show.md) - Show brute force protection settings
- [auth0 protection brute-force-protection update](auth0_protection_brute-force-protection_update.md) - Update brute force protection settings


