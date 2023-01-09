---
layout: default
---
# auth0 test login

Launch a browser to try out your Universal Login box.

## Usage
```
auth0 test login [flags]
```

## Examples

```
  auth0 test login
  auth0 test login <client-id>
  auth0 test login <client-id> --connection <connection>
  auth0 test login <client-id> --connection <connection> --audience <audience>
  auth0 test login <client-id> --connection <connection> --audience <audience> --domain <domain>
  auth0 test login <client-id> --connection <connection> --audience <audience> --domain <domain> --scopes <scope1,scope2>
  auth0 test login <client-id> -c <connection> -a <audience> -d <domain> -s <scope1,scope2> --force
  auth0 test login <client-id> -c <connection> -a <audience> -d <domain> -s <scope1,scope2> --json
  auth0 test login <client-id> -c <connection> -a <audience> -d <domain> -s <scope1,scope2> --force --json
```


## Flags

```
  -a, --audience string     The unique identifier of the target API you want to access.
      --connection string   Connection to test during login.
  -d, --domain string       One of your custom domains.
      --force               Skip confirmation.
      --json                Output in json format.
  -s, --scopes strings      The list of scopes you want to use. (default [openid,profile])
```


## InheritedFlags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 test login](auth0_test_login.md) - Try out your Universal Login box
- [auth0 test token](auth0_test_token.md) - Fetch a token for the given application and API


