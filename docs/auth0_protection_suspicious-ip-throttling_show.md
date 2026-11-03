---
layout: default
parent: auth0 protection suspicious-ip-throttling
has_toc: false
---
# auth0 protection suspicious-ip-throttling show

Display the current suspicious ip throttling settings.

## Usage
```
auth0 protection suspicious-ip-throttling show [flags]
```

## Examples

```
  auth0 protection suspicious-ip-throttling show
  auth0 ap sit show --json
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

- [auth0 protection suspicious-ip-throttling ips](auth0_protection_suspicious-ip-throttling_ips.md) - Manage blocked IP addresses
- [auth0 protection suspicious-ip-throttling show](auth0_protection_suspicious-ip-throttling_show.md) - Show suspicious ip throttling settings
- [auth0 protection suspicious-ip-throttling update](auth0_protection_suspicious-ip-throttling_update.md) - Update suspicious ip throttling settings


