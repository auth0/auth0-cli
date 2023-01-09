---
layout: default
---
# auth0 login

Authenticates the Auth0 CLI either as a user using personal credentials or as a machine using client credentials.

Authenticating as a user is recommended when working on a personal machine or other interactive environment; it is not available for Private Cloud users. Authenticating as a machine is recommended when running on a server or non-interactive environments (ex: CI).

## Usage
```
auth0 login [flags]
```

## Examples

```
  auth0 login
  auth0 login --domain <tenant-domain> --client-id <client-id> --client-secret <client-secret>
  auth0 login --scopes "read:client_grants,create:client_grants"
```


## Flags

```
      --client-id string       Client ID of the application when authenticating via client credentials.
      --client-secret string   Client secret of the application when authenticating via client credentials.
      --domain string          Tenant domain of the application when authenticating via client credentials.
      --scopes strings         Additional scopes to request when authenticating via device code flow. By default, only scopes for first-class functions are requested. Primarily useful when using the api command to execute arbitrary Management API requests.
```


## InheritedFlags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


