---
layout: default
parent: auth0 ips
has_toc: false
---
# auth0 ips check

Check if a given IP address is blocked via the Suspicious IP Throttling due to multiple suspicious attempts.

## Usage
```
auth0 ips check [flags]
```

## Examples

```
  auth0 ips check
  auth0 ips check <ip>
  auth0 ips check "178.178.178.178"
```




## InheritedFlags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 ips check](auth0_ips_check.md) - Check IP address
- [auth0 ips unblock](auth0_ips_unblock.md) - Unblock IP address


