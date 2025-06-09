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
  auth0 login --scopes "read:client_grants,create:client_grants"
```


## Flags

```
      --client-id string       Client ID of the application when authenticating via client credentials.
      --client-secret string   Client secret of the application when authenticating via client credentials.
      --domain string          Tenant domain of the application when authenticating via client credentials.
      --profile string         Tenant Profile Label name to load Auth0 credentials from. If not provided, the default profile will be used.
      --scopes strings         Additional scopes to request when authenticating via device code flow. By default, only scopes for first-class functions are requested. Primarily useful when using the api command to execute arbitrary Management API requests.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


