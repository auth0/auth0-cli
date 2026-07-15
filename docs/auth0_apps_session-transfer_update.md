---
layout: default
parent: auth0 apps session-transfer
has_toc: false
---
# auth0 apps session-transfer update



## Usage
```
auth0 apps session-transfer update [flags]
```

## Examples

```
 auth0 apps session-transfer update 
  auth0 apps session-transfer update <app-id>
  auth0 apps session-transfer update <app-id> --can-create-token --json
  auth0 apps session-transfer update <app-id> --delegation-allow-delegated-access=true --delegation-enforce-device-binding=asn
  auth0 apps session-transfer update <app-id> --can-create-token=true --allowed-auth-methods=cookie,query --enforce-device-binding=ip
```


## Flags

```
  -m, --allowed-auth-methods strings               Comma-separated list of authentication methods (e.g., cookie, query).
  -t, --can-create-token                           Allow creation of session transfer tokens.
  -d, --delegation-allow-delegated-access          (Early Access) Allow the application to accept Session Transfer Tokens containing an Actor, enabling delegated (impersonation) access. Defaults to false.
  -b, --delegation-enforce-device-binding string   (Early Access) Device binding enforcement for delegated (impersonation) access: 'ip' or 'asn'. Defaults to 'ip'.
  -e, --enforce-device-binding string              Device binding enforcement: 'none', 'ip', or 'asn'.
      --json                                       Output in json format.
      --json-compact                               Output in compact json format.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 apps session-transfer show](auth0_apps_session-transfer_show.md) - Show session transfer settings for an app
- [auth0 apps session-transfer update](auth0_apps_session-transfer_update.md) - Update session transfer settings for an app


