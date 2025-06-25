---
layout: default
has_toc: false
---
# auth0 login

Authenticates the Auth0 CLI using either personal credentials (user login) or client credentials (machine login).

Use user login on personal machines or interactive environments (not supported for Private Cloud users).
Use machine login for servers, CI, or any non-interactive environments — this is the recommended method for Private Cloud users.



## Usage
```
auth0 login [flags]
```

## Examples

```
  auth0 login
  auth0 login --domain <tenant-domain> --client-id <client-id> --client-secret <client-secret>
  auth0 login --domain <tenant-domain> --client-id <client-id> --client-assertion-signing-alg RS256 --client-assertion-private-key <path-to-private-key>
  auth0 login --domain <tenant-domain> --client-id <client-id> --client-assertion-signing-alg RS256 --client-assertion-private-key <client-assertion-private-key>
  auth0 login --scopes "read:client_grants,create:client_grants"
```


## Flags

```
      --client-assertion-private-key string   Client Assertion Private key with either a file path or direct content when authenticating via Private key JWT.
      --client-assertion-signing-alg string   Client Assertion Signing Algorithm when authenticating via Private key JWT. Supported algorithms: RS256, RS384, PS256.
      --client-id string                      Client ID of the application when authenticating via client credentials.
      --client-secret string                  Client secret of the application when authenticating via client credentials.
      --domain string                         Tenant domain of the application when authenticating via client credentials.
      --scopes strings                        Additional scopes to request when authenticating via device code flow. By default, only scopes for first-class functions are requested. Primarily useful when using the api command to execute arbitrary Management API requests.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


