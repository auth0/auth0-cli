---
layout: default
parent: auth0 test
has_toc: false
---
# auth0 test token

Request an access token for a given application. Specify the API you want this token for with `--audience` (API Identifier). Additionally, you can also specify the `--scopes` to grant.

## Usage
```
auth0 test token [flags]
```

## Examples

```
  auth0 test token
  auth0 test token <client-id> --audience <api-audience|api-identifier> --organization <org-id> --scopes <scope1,scope2> --params "foo=bar"
  auth0 test token <client-id> -a <api-audience|api-identifier> -o <org-id> -s <scope1,scope2>
  auth0 test token <client-id> -a <api-audience|api-identifier> -s <scope1,scope2> --force
  auth0 test token <client-id> -a <api-audience|api-identifier> -o <org-id> -s <scope1,scope2> -p "foo=bar" -p "bazz=buzz" --force
  auth0 test token <client-id> -a <api-audience|api-identifier> -s <scope1,scope2> --json
  auth0 test token <client-id> -a <api-audience|api-identifier> -o <org-id> -s <scope1,scope2> -p "foo=bar","bazz=buzz" --json
  auth0 test token <client-id> -a <api-audience|api-identifier> -s <scope1,scope2> --force --json
```


## Flags

```
  -a, --audience string         The unique identifier of the target API you want to access. For Machine to Machine Applications, only the enabled APIs will be shown within the interactive prompt.
  -d, --domain string           One of your custom domains.
      --force                   Skip confirmation.
      --json                    Output in json format.
  -o, --organization string     organization-id to use for the login. Can use organization-name if allow_organization_name_in_authentication_api is enabled for tenant
  -p, --params stringToString   Custom parameters to include in the login URL. (default [])
  -s, --scopes strings          The list of scopes you want to use.
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


