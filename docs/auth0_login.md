---
layout: default
---
## auth0 login

Authenticate the Auth0 CLI

### Synopsis

Authenticates the Auth0 CLI either as a user using personal credentials or as a machine using client credentials.

```
auth0 login [flags]
```

### Examples

```
auth0 login
auth0 login --domain <tenant-domain> --client-id <client-id> --client-secret <client-secret>
auth0 login --scopes "read:client_grants,create:client_grants"
```

### Options

```
      --client-id string       Client ID of the application when authenticating via client credentials.
      --client-secret string   Client secret of the application when authenticating via client credentials.
      --domain string          Tenant domain of the application when authenticating via client credentials.
  -h, --help                   help for login
      --scopes api             Scopes to request in addition to required defaults when authenticating via device code flow. Primarily useful when using api command to execute arbitrary Management API requests.
```

### Options inherited from parent commands

```
      --debug           Enable debug mode.
      --json            Output in json format.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```

### SEE ALSO

* [auth0](/auth0-cli/)	 - Supercharge your development workflow.

