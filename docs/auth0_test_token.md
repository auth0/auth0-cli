---
layout: default
parent: auth0 test
has_toc: false
---
# auth0 test token

Fetch an access token for the given application.
If --client-id is not provided, the default client "CLI Login Testing" will be used (and created if not exists).
Specify the API you want this token for with --audience (API Identifer). Additionally, you can also specify the --scope to use.

## Usage
```
auth0 test token [flags]
```

## Examples

```
  auth0 test token
  auth0 test token --client-id <id> --audience <audience> --scopes <scope1,scope2>
  auth0 test token -c <id> -a <audience> -s <scope1,scope2>
  auth0 test token -c <id> -a <audience> -s <scope1,scope2> --force
  auth0 test token -c <id> -a <audience> -s <scope1,scope2> --json
  auth0 test token -c <id> -a <audience> -s <scope1,scope2> --force --json
```


## Flags

```
  -a, --audience string    The unique identifier of the target API you want to access.
  -c, --client-id string   Client Id of an Auth0 application.
      --force              Skip confirmation.
      --json               Output in json format.
  -s, --scopes strings     The list of scopes you want to use.
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


