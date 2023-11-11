---
layout: default
parent: auth0 test
has_toc: false
---
# auth0 test login

Try out your tenant's Universal Login experience in a browser.

## Usage
```
auth0 test login [flags]
```

## Examples

```
  auth0 test login
  auth0 test login <client-id>
  auth0 test login <client-id> --connection-name <connection-name>
  auth0 test login <client-id> --connection-name <connection-name> --audience <api-identifier|api-audience>
  auth0 test login <client-id> --connection-name <connection-name> --audience <api-identifier|api-audience> --domain <domain>
  auth0 test login <client-id> --connection-name <connection-name> --audience <api-identifier|api-audience> --domain <domain> --scopes <scope1,scope2>
  auth0 test login <client-id> -c <connection-name> -a <api-identifier|api-audience> -d <domain> -s <scope1,scope2> --force
  auth0 test login <client-id> -c <connection-name> -a <api-identifier|api-audience> -d <domain> -s <scope1,scope2> --json
  auth0 test login <client-id> -c <connection-name> -a <api-identifier|api-audience> -d <domain> -s <scope1,scope2> --force --json
```


## Flags

```
  -a, --audience string          The unique identifier of the target API you want to access. For Machine to Machine and Regular Web Applications, only the enabled APIs will be shown within the interactive prompt.
  -c, --connection-name string   The connection name to test during login.
  -d, --domain string            One of your custom domains.
      --force                    Skip confirmation.
      --json                     Output in json format.
  -s, --scopes strings           The list of scopes you want to use. (default [openid,profile])
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 test login](auth0_test_login.md) - Try out your tenant's Universal Login experience
- [auth0 test token](auth0_test_token.md) - Request an access token for a given application and API


