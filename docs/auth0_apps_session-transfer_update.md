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
  auth0 apps session-transfer update <app-id> --can-create-token=true --allowed-auth-methods=cookie,query --enforce-device-binding=ip
```


## Flags

```
  -m, --allowed-auth-methods strings    Comma-separated list of authentication methods (e.g., cookie, query).
  -t, --can-create-token                Allow creation of session transfer tokens.
  -e, --enforce-device-binding string   Device binding enforcement: 'none', 'ip', or 'asn'.
      --json                            Output in json format.
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


